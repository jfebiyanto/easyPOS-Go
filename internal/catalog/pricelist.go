package catalog

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/crud"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/validate"
	"github.com/jfebiyanto/easyPOS-Go/internal/pricing"
	"github.com/jfebiyanto/easyPOS-Go/internal/uom"
)

var PriceLists = crud.Resource{
	Path: "price-lists", Table: "price_lists", Entity: "price_list", Noun: "Price list", Key: "admin.price-lists", Show: true,
	Search: []string{"code", "name"}, Filters: []string{"store_id"}, Order: "t.code",
	Fields: []crud.Field{
		crud.U("store_id", "Gerai", "stores"),
		crud.S("code", "Kode", 30).Req().Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.B("is_default", "Default", false),
		crud.Dt("effective_from", "Berlaku mulai"),
		crud.Dt("effective_until", "Berlaku sampai"),
		crud.B("is_active", "Aktif", true),
	},
	Expand: []crud.Expand{{Field: "store_id", As: "store", Table: "stores"}},
	Validate: func(_ *crud.Ctx, m, _ crud.Record, _ map[string]any, errs validate.Errors) error {
		from, _ := m["effective_from"].(string)
		until, _ := m["effective_until"].(string)
		if from != "" && until != "" && until < from { // format YYYY-MM-DD: urutan teks = urutan tanggal
			errs.Add("effective_until", "Tidak boleh sebelum tanggal mulai.")
		}
		return nil
	},
	AfterSave: func(c *crud.Ctx, id string, m, _ crud.Record, _ map[string]any) error {
		if m["is_default"] != true { // satu default per company
			return nil
		}
		_, err := c.Q.Exec(c.Ctx, "UPDATE price_lists SET is_default=false, updated_at=$1 WHERE company_id=$2 AND id<>$3::uuid AND is_default", time.Now().UTC(), c.A.CompanyID, id)
		return err
	},
	CanDelete: func(c *crud.Ctx, r crud.Record) (string, error) {
		var n int // store_price_lists ber-CASCADE sehingga tidak tertangkap cek otomatis
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM store_price_lists WHERE price_list_id=$1::uuid", r["id"]).Scan(&n); err != nil {
			return "", err
		}
		if n > 0 {
			return fmt.Sprintf("Price list dipakai %d gerai.", n), nil
		}
		return "", nil
	},
	Decorate: func(c *crud.Ctx, recs []crud.Record, detail bool) error {
		if !detail {
			return nil
		}
		for _, r := range recs {
			var n int
			if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM price_list_items WHERE price_list_id=$1::uuid", r["id"]).Scan(&n); err != nil {
				return err
			}
			r["item_count"] = n
		}
		return nil
	},
}

// itemIn adalah isi request item yang sudah divalidasi dan dinormalkan.
type itemIn struct {
	product string
	unit    *string // nil = satuan dasar
	minQty  int
	price   string
}

// parseItem memvalidasi body item. `old` (edit) mengunci produk; field yang tak dikirim memakai nilai lama.
func parseItem(c *crud.Ctx, in map[string]any, old map[string]any, errs validate.Errors) (*itemIn, error) {
	out := &itemIn{minQty: 1}
	if old != nil {
		out.product, _ = old["product_id"].(string)
		out.unit, _ = old["unit_id"].(*string)
		out.minQty, _ = old["min_qty"].(int)
		out.price, _ = old["price"].(string)
	} else {
		p, _ := in["product_id"].(string)
		if _, err := uuid.Parse(p); err != nil {
			errs.Add("product_id", "Produk wajib dipilih.")
		}
		out.product = p
	}
	if v, ok := in["unit_id"]; ok || old == nil {
		out.unit = nil
		if s, _ := v.(string); s != "" {
			if _, err := uuid.Parse(s); err != nil {
				errs.Add("unit_id", "Satuan tidak valid.")
			}
			out.unit = &s
		}
	}
	if v, ok := in["min_qty"]; ok || old == nil {
		if v == nil {
			out.minQty = 1
		} else if n, msg := crud.I("min_qty", "Min qty").Range("1", "1000000").Parse(v); msg != "" {
			errs.Add("min_qty", msg)
		} else {
			out.minQty = int(n.(int64))
		}
	}
	if v, ok := in["price"]; ok || old == nil {
		val, msg := crud.D("price", "Harga", 2).Range("0", "9999999999999.99").Parse(v)
		if msg == "" && val == nil {
			msg = "Wajib diisi."
		}
		if msg != "" {
			errs.Add("price", msg)
		} else {
			out.price = val.(string)
		}
	}
	if errs.Any() {
		return nil, nil
	}
	// produk milik company + satuan harus anggota group produk
	var base string
	var group *string
	err := c.Q.QueryRow(c.Ctx, "SELECT unit_id::text, uom_group_id::text FROM products WHERE company_id=$1 AND id=$2::uuid", c.A.CompanyID, out.product).Scan(&base, &group)
	if errors.Is(err, pgx.ErrNoRows) {
		errs.Add("product_id", "Produk tidak ditemukan.")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if out.unit != nil && *out.unit == base {
		out.unit = nil // satuan dasar selalu disimpan NULL agar tidak ada dua cara menulis hal yang sama
	}
	if out.unit != nil {
		ok := false
		if group != nil {
			g, err := uom.Load(c.Ctx, c.Q, c.A.CompanyID, uuid.MustParse(*group))
			if err != nil {
				return nil, err
			}
			ok = g.Has(uuid.MustParse(*out.unit))
		}
		if !ok {
			errs.Add("unit_id", "Satuan harus satuan dasar atau anggota UoM group produk.")
			return nil, nil
		}
	}
	return out, nil
}

// itemConflict mengubah pelanggaran unik pli_unique_unit menjadi 422 di min_qty.
func itemConflict(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" && pe.ConstraintName == "pli_unique_unit" {
		return crud.Invalid(validate.Errors{"min_qty": {"Sudah ada harga untuk produk, satuan, dan min qty ini."}})
	}
	return err
}

// loadItem membaca satu item milik price list (nilai mentah untuk edit/audit).
func loadItem(c *crud.Ctx, plID, itemID string) (map[string]any, error) {
	if _, err := uuid.Parse(itemID); err != nil {
		return nil, crud.NotFound()
	}
	var id, pid string
	var unit *string
	var minQty int
	var price decimal.Decimal
	err := c.Q.QueryRow(c.Ctx, "SELECT id::text, product_id::text, unit_id::text, min_qty, price FROM price_list_items WHERE id=$1::uuid AND price_list_id=$2::uuid", itemID, plID).
		Scan(&id, &pid, &unit, &minQty, &price)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, crud.NotFound()
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "product_id": pid, "unit_id": unit, "min_qty": minQty, "price": price.StringFixed(2)}, nil
}

// itemSelect: item + produk + satuan + harga per satuan dasar (price_per_base) dan faktornya.
const itemSelect = `SELECT i.id::text, i.product_id::text, p.sku, p.name, bu.id::text, bu.code, p.base_price::text, p.uom_group_id::text,
 i.unit_id::text, u.code, i.min_qty, i.price::text,
 CASE WHEN i.unit_id IS NULL OR l.alt_qty IS NULL THEN i.price ELSE round(i.price * l.alt_qty / l.base_qty, 2) END::text,
 CASE WHEN i.unit_id IS NULL OR l.alt_qty IS NULL THEN '1.000' ELSE round(l.base_qty / l.alt_qty, 3)::text END
FROM price_list_items i
JOIN products p ON p.id=i.product_id
JOIN units bu ON bu.id=p.unit_id
LEFT JOIN units u ON u.id=i.unit_id
LEFT JOIN uom_group_lines l ON l.uom_group_id=p.uom_group_id AND l.unit_id=i.unit_id`

type itemRow struct {
	ID, ProductID, SKU, Name, BaseUnitID, BaseUnitCode, BasePrice string
	GroupID, UnitID, UnitCode                                     *string
	MinQty                                                        int
	Price, PricePerBase, Factor                                   string
}

func (r itemRow) json() map[string]any {
	var unit any
	if r.UnitID != nil {
		unit = map[string]string{"id": *r.UnitID, "code": *r.UnitCode}
	}
	return map[string]any{
		"id": r.ID, "product_id": r.ProductID, "min_qty": r.MinQty, "price": r.Price, "unit_id": r.UnitID, "unit": unit,
		"product":        map[string]any{"id": r.ProductID, "sku": r.SKU, "name": r.Name, "base_price": r.BasePrice, "uom_group_id": r.GroupID, "unit": map[string]string{"id": r.BaseUnitID, "code": r.BaseUnitCode}},
		"price_per_base": r.PricePerBase, "factor": r.Factor,
	}
}

func scanItem(row pgx.Row) (itemRow, error) {
	var r itemRow
	err := row.Scan(&r.ID, &r.ProductID, &r.SKU, &r.Name, &r.BaseUnitID, &r.BaseUnitCode, &r.BasePrice, &r.GroupID, &r.UnitID, &r.UnitCode, &r.MinQty, &r.Price, &r.PricePerBase, &r.Factor)
	return r, err
}

// ResolveResult hasil harga efektif untuk satu produk di satu price list.
type ResolveResult struct {
	UnitPrice string `json:"unit_price"`
	Source    string `json:"source"`
	Factor    string `json:"factor"`
	UnitID    string `json:"unit_id"`
}

// Resolve memuat price list + produk dari DB lalu memanggil pricing.ResolvePrice. unit "" = satuan dasar.
// date kosong = hari ini di zona waktu company. Dipakai endpoint uji dan POS fase 3.
func Resolve(c *crud.Ctx, plID, productID, unit string, qty decimal.Decimal, date time.Time) (*ResolveResult, error) {
	if _, err := uuid.Parse(productID); err != nil {
		return nil, crud.Invalid(validate.Errors{"product_id": {"Produk wajib dipilih."}})
	}
	var pl pricing.PriceList
	err := c.Q.QueryRow(c.Ctx, "SELECT is_active, effective_from, effective_until FROM price_lists WHERE company_id=$1 AND id=$2::uuid", c.A.CompanyID, plID).Scan(&pl.Active, &pl.From, &pl.Until)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, crud.NotFound()
	}
	if err != nil {
		return nil, err
	}

	var prod pricing.Product
	var baseUnit string
	var group *string
	err = c.Q.QueryRow(c.Ctx, "SELECT base_price, unit_id::text, uom_group_id::text FROM products WHERE company_id=$1 AND id=$2::uuid", c.A.CompanyID, productID).Scan(&prod.BasePrice, &baseUnit, &group)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, crud.Invalid(validate.Errors{"product_id": {"Produk tidak ditemukan."}})
	}
	if err != nil {
		return nil, err
	}
	prod.Group = uom.Group{BaseUnitID: uuid.MustParse(baseUnit)}
	if group != nil {
		if prod.Group, err = uom.Load(c.Ctx, c.Q, c.A.CompanyID, uuid.MustParse(*group)); err != nil {
			return nil, err
		}
	}
	rows, err := c.Q.Query(c.Ctx, "SELECT unit_id::text, min_qty, price FROM price_list_items WHERE price_list_id=$1::uuid AND product_id=$2::uuid", plID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u *string
		var it pricing.Item
		if err := rows.Scan(&u, &it.MinQty, &it.Price); err != nil {
			return nil, err
		}
		if u != nil {
			it.UnitID = uuid.MustParse(*u)
		}
		pl.Items = append(pl.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	uid := uuid.Nil
	if unit != "" {
		if uid, err = uuid.Parse(unit); err != nil {
			return nil, crud.Invalid(validate.Errors{"unit_id": {"Satuan tidak valid."}})
		}
	}
	if date.IsZero() {
		var tz string
		_ = c.Q.QueryRow(c.Ctx, "SELECT timezone FROM company_details WHERE id=$1", c.A.CompanyID).Scan(&tz)
		loc, lerr := time.LoadLocation(tz)
		if lerr != nil {
			loc = time.UTC
		}
		date = time.Now().In(loc)
	}
	res, err := pricing.ResolvePrice(&pl, prod, uid, qty, date)
	switch {
	case errors.Is(err, pricing.ErrQty):
		return nil, crud.Invalid(validate.Errors{"qty": {"Harus lebih dari nol."}})
	case errors.Is(err, uom.ErrUnknownUnit):
		return nil, crud.Invalid(validate.Errors{"unit_id": {"Satuan bukan anggota UoM group produk."}})
	case err != nil:
		return nil, err
	}
	if uid == uuid.Nil {
		uid = prod.Group.BaseUnitID
	}
	f, _ := prod.Group.Factor(uid)
	return &ResolveResult{UnitPrice: res.UnitPrice.StringFixed(2), Source: res.Source, Factor: f.StringFixed(3), UnitID: uid.String()}, nil
}

func auditItem(c *crud.Ctx, action, plID, desc string, props map[string]any) error {
	return audit.Log(c.Ctx, c.Q, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, StoreID: c.A.StoreID,
		Action: action, SubjectType: "price_list", SubjectID: uuid.MustParse(plID), Description: desc, Properties: props})
}

// MountPriceLists memasang CRUD price list, item per produk/satuan/min qty, dan endpoint uji harga efektif.
func MountPriceLists(b *crud.Binder, r chi.Router) {
	res := PriceLists
	b.Mount(r, &res)

	r.With(b.Route("admin.price-lists.show")).Get("/price-lists/{id}/items", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		pl, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		qs := req.URL.Query()
		per, _ := strconv.Atoi(qs.Get("per_page"))
		if per < 1 || per > 100 {
			per = 25
		}
		page, _ := strconv.Atoi(qs.Get("page"))
		if page < 1 {
			page = 1
		}
		where, args := "i.price_list_id=$1::uuid", []any{pl["id"]}
		if q := strings.TrimSpace(qs.Get("q")); q != "" {
			where += " AND (p.sku ILIKE $2 OR p.name ILIKE $2)"
			args = append(args, "%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)+"%")
		}
		var total int
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM price_list_items i JOIN products p ON p.id=i.product_id WHERE "+where, args...).Scan(&total); err != nil {
			crud.WriteErr(w, err)
			return
		}
		rows, err := c.Q.Query(c.Ctx, fmt.Sprintf("%s WHERE %s ORDER BY p.sku, u.code NULLS FIRST, i.min_qty LIMIT %d OFFSET %d", itemSelect, where, per, (page-1)*per), args...)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		defer rows.Close()
		data := []map[string]any{}
		for rows.Next() {
			it, err := scanItem(rows)
			if err != nil {
				crud.WriteErr(w, err)
				return
			}
			data = append(data, it.json())
		}
		if err := rows.Err(); err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": data, "meta": map[string]int{"page": page, "per_page": per, "total": total, "last_page": (total + per - 1) / per}})
	})

	one := func(c *crud.Ctx, plID, itemID string) (map[string]any, error) {
		it, err := scanItem(c.Q.QueryRow(c.Ctx, itemSelect+" WHERE i.id=$1::uuid AND i.price_list_id=$2::uuid", itemID, plID))
		if err != nil {
			return nil, err
		}
		return it.json(), nil
	}

	r.With(b.Route("admin.price-lists.items.store")).Post("/price-lists/{id}/items", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		pl, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		in, err := crud.DecodeMap(req)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		errs := validate.Errors{}
		it, err := parseItem(c, in, nil, errs)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		if errs.Any() {
			crud.WriteErr(w, crud.Invalid(errs))
			return
		}
		plID, id := pl["id"].(string), ""
		err = b.E.InTx(c, func(tc *crud.Ctx) error {
			nid, _ := uuid.NewV7()
			id = nid.String()
			now := time.Now().UTC()
			if _, err := tc.Q.Exec(tc.Ctx, "INSERT INTO price_list_items (id, price_list_id, product_id, unit_id, min_qty, price, created_at, updated_at) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6::numeric,$7,$7)",
				id, plID, it.product, it.unit, it.minQty, it.price, now); err != nil {
				return itemConflict(err)
			}
			return auditItem(tc, "price_list.item_added", plID, fmt.Sprintf("Item ditambahkan ke %v", pl["code"]), map[string]any{"new": map[string]any{"product_id": it.product, "unit_id": it.unit, "min_qty": it.minQty, "price": it.price}})
		})
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		row, err := one(c, plID, id)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 201, map[string]any{"data": row})
	})

	r.With(b.Route("admin.price-lists.items.update")).Put("/price-lists/{id}/items/{itemId}", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		pl, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		plID := pl["id"].(string)
		old, err := loadItem(c, plID, chi.URLParam(req, "itemId"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		in, err := crud.DecodeMap(req)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		errs := validate.Errors{}
		it, err := parseItem(c, in, old, errs)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		if errs.Any() {
			crud.WriteErr(w, crud.Invalid(errs))
			return
		}
		err = b.E.InTx(c, func(tc *crud.Ctx) error {
			if _, err := tc.Q.Exec(tc.Ctx, "UPDATE price_list_items SET unit_id=$1::uuid, min_qty=$2, price=$3::numeric, updated_at=$4 WHERE id=$5::uuid",
				it.unit, it.minQty, it.price, time.Now().UTC(), old["id"]); err != nil {
				return itemConflict(err)
			}
			return auditItem(tc, "price_list.item_updated", plID, fmt.Sprintf("Item diubah di %v", pl["code"]),
				map[string]any{"old": map[string]any{"unit_id": old["unit_id"], "min_qty": old["min_qty"], "price": old["price"]}, "new": map[string]any{"unit_id": it.unit, "min_qty": it.minQty, "price": it.price}})
		})
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		row, err := one(c, plID, old["id"].(string))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": row})
	})

	r.With(b.Route("admin.price-lists.items.destroy")).Delete("/price-lists/{id}/items/{itemId}", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		pl, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		plID := pl["id"].(string)
		old, err := loadItem(c, plID, chi.URLParam(req, "itemId"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		err = b.E.InTx(c, func(tc *crud.Ctx) error {
			if _, err := tc.Q.Exec(tc.Ctx, "DELETE FROM price_list_items WHERE id=$1::uuid", old["id"]); err != nil {
				return err
			}
			return auditItem(tc, "price_list.item_removed", plID, fmt.Sprintf("Item dihapus dari %v", pl["code"]), map[string]any{"old": old})
		})
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.With(b.Route("admin.price-lists.show")).Get("/price-lists/{id}/resolve", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		pl, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		qs := req.URL.Query()
		qty := decimal.NewFromInt(1)
		if s := qs.Get("qty"); s != "" {
			if qty, err = decimal.NewFromString(s); err != nil {
				crud.WriteErr(w, crud.Invalid(validate.Errors{"qty": {"Bukan angka."}}))
				return
			}
		}
		var date time.Time
		if s := qs.Get("date"); s != "" {
			if date, err = time.Parse("2006-01-02", s); err != nil {
				crud.WriteErr(w, crud.Invalid(validate.Errors{"date": {"Format YYYY-MM-DD."}}))
				return
			}
		}
		out, err := Resolve(c, pl["id"].(string), qs.Get("product_id"), qs.Get("unit_id"), qty, date)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": out})
	})
}
