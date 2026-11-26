package catalog

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/crud"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/storage"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/validate"
	"github.com/jfebiyanto/easyPOS-Go/internal/uom"
)

const maxImageBytes = 2 << 20

// Products: master produk. Barcode alternatif (product_barcodes) dikirim sebagai `barcodes` (replace-set).
var Products = crud.Resource{
	Path: "products", Table: "products", Entity: "product", Noun: "Produk", Key: "admin.products",
	Search:      []string{"sku", "name", "barcode"},
	SearchExtra: `EXISTS (SELECT 1 FROM product_barcodes pb WHERE pb.product_id=t.id AND pb.code ILIKE {q})`,
	Filters:     []string{"product_group_id", "brand_id", "manage_by"},
	Order:       "t.sku",
	Fields: []crud.Field{
		crud.U("product_group_id", "Grup produk", "product_groups").Req(),
		crud.U("brand_id", "Merek", "brands"),
		crud.U("unit_id", "Satuan dasar", "units").Req(),
		crud.S("sku", "SKU", 40).Req().Uniq(),
		crud.S("barcode", "Barcode", 60).Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.S("description", "Deskripsi", 500),
		crud.D("base_price", "Harga dasar", 2).Range("0", "9999999999999.99").Def("0.00"),
		crud.D("cost_price", "Harga pokok", 2).RO(),
		crud.B("track_stock", "Lacak stok", true),
		crud.B("is_active", "Aktif", true),
		crud.U("uom_group_id", "UoM group", "uom_groups"),
		crud.U("sales_unit_id", "Satuan jual", "units"),
		crud.S("manage_by", "Kelola per", 10).Req().In("none", "batch", "serial").Def("none"),
		crud.S("image_path", "Gambar", 255).RO(),
	},
	Expand: []crud.Expand{
		{Field: "product_group_id", As: "product_group", Table: "product_groups"},
		{Field: "brand_id", As: "brand", Table: "brands"},
		{Field: "unit_id", As: "unit", Table: "units"},
		{Field: "sales_unit_id", As: "sales_unit", Table: "units"},
		{Field: "uom_group_id", As: "uom_group", Table: "uom_groups"},
	},
	Validate:  validateProduct,
	AfterSave: saveBarcodes,
	CanDelete: productInUse,
	Decorate:  decorateProducts,
}

// ---- barcode alternatif ----

type bcIn struct{ id, code, unit, note string }

type bcRow struct {
	id, code, unit, source string
}

// rawBarcodes membaca `barcodes` apa adanya (string dipangkas). provided=false bila tidak dikirim.
func rawBarcodes(in map[string]any) (out []bcIn, provided bool, bad bool) {
	raw, ok := in["barcodes"]
	if !ok {
		return nil, false, false
	}
	arr, isArr := raw.([]any)
	if !isArr {
		return nil, true, true
	}
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return strings.TrimSpace(s) }
	for _, it := range arr {
		m, isMap := it.(map[string]any)
		if !isMap {
			bad = true
			continue
		}
		out = append(out, bcIn{id: str(m, "id"), code: str(m, "code"), unit: strings.ToLower(str(m, "unit_id")), note: str(m, "note")})
	}
	return out, true, bad
}

func existingBarcodes(c *crud.Ctx, product string) ([]bcRow, error) {
	rows, err := c.Q.Query(c.Ctx, "SELECT id::text, code, unit_id::text, source FROM product_barcodes WHERE product_id=$1::uuid", product)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bcRow
	for rows.Next() {
		var b bcRow
		if err := rows.Scan(&b.id, &b.code, &b.unit, &b.source); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func validateProduct(c *crud.Ctx, m, old crud.Record, in map[string]any, errs validate.Errors) error {
	unit, _ := m["unit_id"].(string)
	self := uuid.Nil.String()
	if old != nil {
		self, _ = old["id"].(string)
	}

	// UoM group: satuan dasar produk = satuan dasar group; satuan jual & barcode harus anggota group.
	var grp *uom.Group
	if g, _ := m["uom_group_id"].(string); g != "" {
		gg, err := uom.Load(c.Ctx, c.Q, c.A.CompanyID, uuid.MustParse(g))
		if err != nil {
			return err
		}
		if gg.BaseUnitID.String() != unit {
			errs.Add("unit_id", "Harus sama dengan satuan dasar UoM group.")
		}
		grp = &gg
	}
	member := func(u string) bool {
		if u == unit {
			return true
		}
		id, err := uuid.Parse(u)
		return grp != nil && err == nil && grp.Has(id)
	}
	if su, _ := m["sales_unit_id"].(string); su != "" && !member(su) {
		errs.Add("sales_unit_id", "Harus satuan dasar atau anggota UoM group.")
	}

	// manage_by: dikelola SAP bila aktif; tidak boleh berubah setelah ada batch/serial/stok.
	mb, _ := m["manage_by"].(string)
	oldMB := "none"
	if old != nil {
		oldMB, _ = old["manage_by"].(string)
	}
	if mb != oldMB {
		var sap bool
		if err := c.Q.QueryRow(c.Ctx, "SELECT sap_enabled FROM company_details WHERE id=$1", c.A.CompanyID).Scan(&sap); err != nil {
			return err
		}
		if sap {
			errs.Add("manage_by", "Dikelola dari SAP, tidak bisa diubah di sini.")
		} else if old != nil {
			var n int
			if err := c.Q.QueryRow(c.Ctx, `SELECT (SELECT count(*) FROM product_batches WHERE product_id=$1::uuid)
 + (SELECT count(*) FROM product_serials WHERE product_id=$1::uuid) + (SELECT count(*) FROM warehouse_stocks WHERE product_id=$1::uuid)`, self).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return crud.Conflict("Metode kelola tidak bisa diubah: produk sudah punya batch, serial, atau stok.")
			}
		}
	}

	// Barcode utama tidak boleh sama dengan barcode satuan produk lain.
	if bc, _ := m["barcode"].(string); bc != "" && !errs.Has("barcode") {
		var n int
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM product_barcodes WHERE company_id=$1 AND code=$2 AND product_id<>$3::uuid", c.A.CompanyID, bc, self).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			errs.Add("barcode", "Sudah dipakai sebagai barcode satuan produk lain.")
		}
	}

	// Barcode alternatif.
	list, provided, bad := rawBarcodes(in)
	if bad {
		errs.Add("barcodes", "Baris barcode tidak valid.")
		return nil
	}
	var existing []bcRow
	if old != nil {
		var err error
		if existing, err = existingBarcodes(c, self); err != nil {
			return err
		}
	}
	if !provided { // baris lama harus tetap cocok dengan group/satuan hasil ubahan
		for _, b := range existing {
			if !member(b.unit) {
				errs.Add("barcodes", fmt.Sprintf("Barcode %s memakai satuan di luar UoM group; kirim ulang daftar barcodes.", b.code))
				break
			}
		}
		return nil
	}
	ids := map[string]bcRow{}
	for _, b := range existing {
		ids[b.id] = b
	}
	seen := map[string]bool{}
	mainBC, _ := m["barcode"].(string)
	for i, b := range list {
		key := func(f string) string { return fmt.Sprintf("barcodes.%d.%s", i, f) }
		if b.id != "" {
			if _, ok := ids[strings.ToLower(b.id)]; !ok {
				errs.Add(key("id"), "Barcode tidak ditemukan pada produk ini.")
			}
		}
		if len([]rune(b.note)) > 255 {
			errs.Add(key("note"), "Maksimal 255 karakter.")
		}
		if _, err := uuid.Parse(b.unit); err != nil {
			errs.Add(key("unit_id"), "Satuan wajib dipilih.")
		} else if !member(b.unit) {
			errs.Add(key("unit_id"), "Harus satuan dasar atau anggota UoM group.")
		}
		switch {
		case b.code == "":
			errs.Add(key("code"), "Wajib diisi.")
		case len([]rune(b.code)) > 60:
			errs.Add(key("code"), "Maksimal 60 karakter.")
		case seen[b.code] || b.code == mainBC:
			errs.Add(key("code"), "Kode dobel pada produk ini.")
		default:
			var n int
			if err := c.Q.QueryRow(c.Ctx, `SELECT (SELECT count(*) FROM products WHERE company_id=$1 AND barcode=$2 AND id<>$3::uuid)
 + (SELECT count(*) FROM product_barcodes WHERE company_id=$1 AND code=$2 AND product_id<>$3::uuid)`, c.A.CompanyID, b.code, self).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				errs.Add(key("code"), "Sudah dipakai produk lain.")
			}
		}
		seen[b.code] = true
	}
	// Baris dari SAP tidak bisa diubah atau dihapus.
	for _, e := range existing {
		if e.source != "sap" {
			continue
		}
		found := false
		for _, b := range list {
			if strings.EqualFold(b.id, e.id) {
				found = true
				if b.code != e.code || b.unit != e.unit {
					return crud.Conflict(fmt.Sprintf("Barcode %s berasal dari SAP dan tidak bisa diubah.", e.code))
				}
			}
		}
		if !found {
			return crud.Conflict(fmt.Sprintf("Barcode %s berasal dari SAP dan tidak bisa dihapus.", e.code))
		}
	}
	return nil
}

// saveBarcodes mengganti baris lokal (replace-set). Baris SAP tidak disentuh. Penghapusan-lalu-isi ulang
// membuat penukaran kode antar baris tidak bentrok dengan UNIQUE (company_id, code).
func saveBarcodes(c *crud.Ctx, id string, _, _ crud.Record, in map[string]any) error {
	list, provided, _ := rawBarcodes(in)
	if !provided {
		return nil
	}
	if _, err := c.Q.Exec(c.Ctx, "DELETE FROM product_barcodes WHERE product_id=$1::uuid AND source='local'", id); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, b := range list {
		var sap int
		if b.id != "" {
			if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM product_barcodes WHERE id=$1::uuid AND source='sap'", b.id).Scan(&sap); err != nil {
				return err
			}
		}
		if sap > 0 {
			continue
		}
		bid := b.id
		if bid == "" {
			n, _ := uuid.NewV7()
			bid = n.String()
		}
		var note any
		if b.note != "" {
			note = b.note
		}
		if _, err := c.Q.Exec(c.Ctx, `INSERT INTO product_barcodes (id, company_id, product_id, unit_id, code, source, note, created_at, updated_at)
VALUES ($1::uuid,$2,$3::uuid,$4::uuid,$5,'local',$6,$7,$7)`, bid, c.A.CompanyID, id, b.unit, b.code, note, now); err != nil {
			return err
		}
	}
	return nil
}

// productInUse menolak hapus bila ada stok, batch/serial, atau item price list. Promosi dan transaksi
// ditangkap pemeriksaan FK otomatis mesin CRUD.
func productInUse(c *crud.Ctx, rec crud.Record) (string, error) {
	var parts []string
	for _, t := range []struct{ table, label string }{
		{"price_list_items", "item price list"}, {"warehouse_stocks", "stok gudang"}, {"warehouse_stock_ledgers", "riwayat stok gudang"},
		{"stock_ledger", "kartu stok"}, {"stock_adjustment_lines", "baris penyesuaian stok"}, {"stock_transfer_lines", "baris transfer stok"},
		{"stock_transfer_shipment_lines", "baris pengiriman transfer"}, {"product_batches", "batch"}, {"product_serials", "serial"},
	} {
		var n int
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM "+t.table+" WHERE product_id=$1::uuid", rec["id"]).Scan(&n); err != nil {
			return "", err
		}
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, t.label))
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "Produk masih dipakai " + strings.Join(parts, ", ") + ".", nil
}

func decorateProducts(c *crud.Ctx, recs []crud.Record, detail bool) error {
	for _, r := range recs {
		r["image_url"] = nil
		if p, _ := r["image_path"].(string); p != "" {
			r["image_url"] = "/media/" + p
		}
	}
	if !detail {
		return nil
	}
	for _, r := range recs {
		id := r["id"].(string)
		var grp *uom.Group
		if g, _ := r["uom_group_id"].(string); g != "" {
			if gg, err := uom.Load(c.Ctx, c.Q, c.A.CompanyID, uuid.MustParse(g)); err == nil {
				grp = &gg
			}
		}
		rows, err := c.Q.Query(c.Ctx, `SELECT b.id::text, b.code, b.unit_id::text, u.code, u.name, b.source, b.note
FROM product_barcodes b JOIN units u ON u.id=b.unit_id WHERE b.product_id=$1::uuid ORDER BY b.created_at, b.code`, id)
		if err != nil {
			return err
		}
		out := []map[string]any{}
		for rows.Next() {
			var bid, code, uid, ucode, uname, src string
			var note *string
			if err := rows.Scan(&bid, &code, &uid, &ucode, &uname, &src, &note); err != nil {
				rows.Close()
				return err
			}
			factor := "1.000"
			if grp != nil {
				if f, err := grp.Factor(uuid.MustParse(uid)); err == nil {
					factor = f.StringFixed(3)
				}
			}
			out = append(out, map[string]any{"id": bid, "code": code, "unit_id": uid, "unit": map[string]string{"id": uid, "code": ucode, "name": uname},
				"factor": factor, "source": src, "note": note})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		r["barcodes"] = out
	}
	return nil
}

// ---- lookup ----

// LookupResult: produk, satuan yang cocok dengan barcode, dan faktor ke satuan dasar.
type LookupResult struct {
	ProductID string
	Unit      map[string]string
	Factor    string
}

// Lookup mencari barcode utama (`products.barcode`, satuan dasar) lalu barcode satuan (`product_barcodes`).
// Dipakai POS fase 3. Tidak ditemukan → crud.NotFound().
func Lookup(c *crud.Ctx, code string) (*LookupResult, error) {
	var pid, unit string
	var group *string
	err := c.Q.QueryRow(c.Ctx, "SELECT id::text, unit_id::text, uom_group_id::text FROM products WHERE company_id=$1 AND barcode=$2", c.A.CompanyID, code).Scan(&pid, &unit, &group)
	if errors.Is(err, pgx.ErrNoRows) {
		err = c.Q.QueryRow(c.Ctx, `SELECT p.id::text, b.unit_id::text, p.uom_group_id::text FROM product_barcodes b JOIN products p ON p.id=b.product_id
WHERE b.company_id=$1 AND b.code=$2`, c.A.CompanyID, code).Scan(&pid, &unit, &group)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, crud.NotFound()
	}
	if err != nil {
		return nil, err
	}
	res := &LookupResult{ProductID: pid, Factor: "1.000"}
	var base string
	if err := c.Q.QueryRow(c.Ctx, "SELECT unit_id::text FROM products WHERE id=$1::uuid", pid).Scan(&base); err != nil {
		return nil, err
	}
	if unit != base {
		if group == nil {
			return nil, crud.NotFound() // data tak konsisten: satuan non-dasar tanpa group
		}
		g, err := uom.Load(c.Ctx, c.Q, c.A.CompanyID, uuid.MustParse(*group))
		if err != nil {
			return nil, err
		}
		f, err := g.Factor(uuid.MustParse(unit))
		if err != nil {
			return nil, crud.NotFound()
		}
		res.Factor = f.StringFixed(3)
	}
	var ucode, uname string
	if err := c.Q.QueryRow(c.Ctx, "SELECT code, name FROM units WHERE id=$1::uuid", unit).Scan(&ucode, &uname); err != nil {
		return nil, err
	}
	res.Unit = map[string]string{"id": unit, "code": ucode, "name": uname}
	return res, nil
}

// ---- rute ----

var imageTypes = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// MountProducts memasang CRUD produk, lookup barcode, dan unggah/hapus gambar (disimpan di storageDir).
func MountProducts(b *crud.Binder, r chi.Router, storageDir string) {
	res := Products
	res.AfterDelete = func(_ *crud.Ctx, rec crud.Record) {
		if p, _ := rec["image_path"].(string); p != "" {
			storage.Remove(storageDir, p)
		}
	}

	r.With(b.Route("admin.products.index")).Get("/products/lookup", func(w http.ResponseWriter, req *http.Request) {
		code := strings.TrimSpace(req.URL.Query().Get("code"))
		if code == "" {
			crud.WriteErr(w, crud.Invalid(validate.Errors{"code": {"Wajib diisi."}}))
			return
		}
		c := b.Ctx(req)
		lk, err := Lookup(c, code)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		prod, err := b.One(c, &res, lk.ProductID)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": map[string]any{"product": prod, "unit": lk.Unit, "factor": lk.Factor}})
	})
	b.Mount(r, &res)

	r.With(b.Route("admin.products.update")).Post("/products/{id}/image", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		rec, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		bad := func(msg string) { crud.WriteErr(w, crud.Invalid(validate.Errors{"image": {msg}})) }
		req.Body = http.MaxBytesReader(w, req.Body, maxImageBytes+(64<<10))
		file, _, err := req.FormFile("image")
		if err != nil {
			bad("Pilih file gambar (JPG, PNG, atau WebP, maksimal 2 MB).")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
		if err != nil || len(data) > maxImageBytes {
			bad("Ukuran gambar maksimal 2 MB.")
			return
		}
		ext, ok := imageTypes[http.DetectContentType(data)] // tipe dari isi file, bukan nama/header klien
		if !ok {
			bad("Hanya JPG, PNG, atau WebP.")
			return
		}
		n, _ := uuid.NewV7()
		rel := "products/" + n.String() + "." + ext
		full, _ := storage.Join(storageDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			crud.WriteErr(w, err)
			return
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			crud.WriteErr(w, err)
			return
		}
		id := rec["id"].(string)
		err = b.E.InTx(c, func(tc *crud.Ctx) error {
			if _, err := tc.Q.Exec(tc.Ctx, "UPDATE products SET image_path=$1, updated_at=$2 WHERE company_id=$3 AND id=$4::uuid", rel, time.Now().UTC(), c.A.CompanyID, id); err != nil {
				return err
			}
			return audit.Log(tc.Ctx, tc.Q, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, StoreID: c.A.StoreID,
				Action: "product.image_updated", SubjectType: "product", SubjectID: uuid.MustParse(id), Description: "Gambar produk diubah: " + fmt.Sprint(rec["sku"]),
				Properties: map[string]any{"old": map[string]any{"image_path": rec["image_path"]}, "new": map[string]any{"image_path": rel}}})
		})
		if err != nil {
			storage.Remove(storageDir, rel)
			crud.WriteErr(w, err)
			return
		}
		if p, _ := rec["image_path"].(string); p != "" {
			storage.Remove(storageDir, p)
		}
		out, err := b.One(c, &res, id)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": out})
	})

	r.With(b.Route("admin.products.update")).Delete("/products/{id}/image", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		rec, err := b.E.Load(c, &res, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		id := rec["id"].(string)
		old, _ := rec["image_path"].(string)
		if old != "" {
			err = b.E.InTx(c, func(tc *crud.Ctx) error {
				if _, err := tc.Q.Exec(tc.Ctx, "UPDATE products SET image_path=NULL, updated_at=$1 WHERE company_id=$2 AND id=$3::uuid", time.Now().UTC(), c.A.CompanyID, id); err != nil {
					return err
				}
				return audit.Log(tc.Ctx, tc.Q, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, StoreID: c.A.StoreID,
					Action: "product.image_removed", SubjectType: "product", SubjectID: uuid.MustParse(id), Description: "Gambar produk dihapus: " + fmt.Sprint(rec["sku"]),
					Properties: map[string]any{"old": map[string]any{"image_path": old}, "new": map[string]any{"image_path": nil}}})
			})
			if err != nil {
				crud.WriteErr(w, err)
				return
			}
			storage.Remove(storageDir, old)
		}
		out, err := b.One(c, &res, id)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": out})
	})
}
