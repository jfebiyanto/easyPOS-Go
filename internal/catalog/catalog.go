// Package catalog: satuan, UoM group, grup produk, dan merek.
package catalog

import (
	"fmt"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/crud"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/validate"
)

var Units = crud.Resource{
	Path: "units", Table: "units", Entity: "unit", Noun: "Satuan", Key: "admin.units",
	Search: []string{"code", "name"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 20).Req().Uniq(),
		crud.S("name", "Nama", 100).Req(),
		crud.B("is_active", "Aktif", true),
	},
}

var ProductGroups = crud.Resource{
	Path: "product-groups", Table: "product_groups", Entity: "product_group", Noun: "Grup produk", Key: "admin.groups",
	Search: []string{"code", "name"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 30).Req().Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.S("description", "Deskripsi", 500),
		crud.B("is_active", "Aktif", true),
	},
}

var Brands = crud.Resource{
	Path: "brands", Table: "brands", Entity: "brand", Noun: "Merek", Key: "admin.brands",
	Search: []string{"code", "name"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 30).Req().Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.B("is_active", "Aktif", true),
	},
}

// lineIn adalah satu baris `lines` dari request, sudah divalidasi.
type lineIn struct {
	unit            string
	altQty, baseQty decimal.Decimal
}

var qtyField = crud.D("qty", "Jumlah", 3).Pos()

// parseLines memvalidasi array `lines`. Baris satuan dasar diabaikan (server selalu menjamin 1:1).
// Mengembalikan (nil, false) bila `lines` tidak dikirim.
func parseLines(c *crud.Ctx, in map[string]any, base string, errs validate.Errors) ([]lineIn, bool, error) {
	raw, ok := in["lines"]
	if !ok {
		return nil, false, nil
	}
	arr, isArr := raw.([]any)
	if !isArr {
		errs.Add("lines", "Harus berupa daftar baris.")
		return nil, true, nil
	}
	var out []lineIn
	seen := map[string]bool{}
	for i, it := range arr {
		key := func(f string) string { return fmt.Sprintf("lines.%d.%s", i, f) }
		row, isMap := it.(map[string]any)
		if !isMap {
			errs.Add("lines", "Baris tidak valid.")
			continue
		}
		unit, _ := row["unit_id"].(string)
		if _, err := uuid.Parse(unit); err != nil {
			errs.Add(key("unit_id"), "Satuan wajib dipilih.")
			continue
		}
		var n int
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM units WHERE company_id=$1 AND id=$2::uuid", c.A.CompanyID, unit).Scan(&n); err != nil {
			return nil, true, err
		}
		if n == 0 {
			errs.Add(key("unit_id"), "Satuan tidak ditemukan.")
			continue
		}
		if unit == base {
			continue
		}
		if seen[unit] {
			errs.Add(key("unit_id"), "Satuan dobel dalam group.")
			continue
		}
		seen[unit] = true
		alt, msg1 := qtyField.Parse(row["alt_qty"])
		bq, msg2 := qtyField.Parse(row["base_qty"])
		if alt == nil && msg1 == "" {
			msg1 = "Wajib diisi."
		}
		if bq == nil && msg2 == "" {
			msg2 = "Wajib diisi."
		}
		if msg1 != "" {
			errs.Add(key("alt_qty"), msg1)
		}
		if msg2 != "" {
			errs.Add(key("base_qty"), msg2)
		}
		if msg1 != "" || msg2 != "" {
			continue
		}
		out = append(out, lineIn{unit: unit, altQty: decimal.RequireFromString(alt.(string)), baseQty: decimal.RequireFromString(bq.(string))})
	}
	return out, true, nil
}

// removedLineUsage: pesan bila baris yang akan dihapus masih dipakai barcode/price list item/satuan jual produk group ini.
func removedLineUsage(c *crud.Ctx, group string, keep map[string]bool) (string, error) {
	rows, err := c.Q.Query(c.Ctx, "SELECT unit_id::text FROM uom_group_lines WHERE uom_group_id=$1::uuid", group)
	if err != nil {
		return "", err
	}
	var existing []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			return "", err
		}
		existing = append(existing, u)
	}
	rows.Close()
	for _, u := range existing {
		if keep[u] {
			continue
		}
		var bc, pl, su int
		err := c.Q.QueryRow(c.Ctx, `SELECT
 (SELECT count(*) FROM product_barcodes b JOIN products p ON p.id=b.product_id WHERE p.uom_group_id=$1::uuid AND b.unit_id=$2::uuid),
 (SELECT count(*) FROM price_list_items i JOIN products p ON p.id=i.product_id WHERE p.uom_group_id=$1::uuid AND i.unit_id=$2::uuid),
 (SELECT count(*) FROM products p WHERE p.uom_group_id=$1::uuid AND p.sales_unit_id=$2::uuid)`, group, u).Scan(&bc, &pl, &su)
		if err != nil {
			return "", err
		}
		if bc+pl+su > 0 {
			var code string
			_ = c.Q.QueryRow(c.Ctx, "SELECT code FROM units WHERE id=$1::uuid", u).Scan(&code)
			return fmt.Sprintf("Satuan %s tidak bisa dihapus dari group: dipakai %d barcode, %d item price list, %d produk (satuan jual).", code, bc, pl, su), nil
		}
	}
	return "", nil
}

var UomGroups = crud.Resource{
	Path: "uom-groups", Table: "uom_groups", Entity: "uom_group", Noun: "UoM group", Key: "admin.uom-groups", Show: true,
	Search: []string{"code", "name"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 20).Req().Uniq(),
		crud.S("name", "Nama", 100).Req(),
		crud.U("base_unit_id", "Satuan dasar", "units").Req(),
		crud.B("is_active", "Aktif", true),
	},
	Expand: []crud.Expand{{Field: "base_unit_id", As: "base_unit", Table: "units"}},
	Validate: func(c *crud.Ctx, m, old crud.Record, in map[string]any, errs validate.Errors) error {
		base := m["base_unit_id"].(string)
		lines, provided, err := parseLines(c, in, base, errs)
		if err != nil || errs.Any() {
			return err
		}
		if old == nil {
			return nil
		}
		id := old["id"].(string)
		if old["base_unit_id"] != base {
			var n int
			if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM products WHERE uom_group_id=$1::uuid", id).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return crud.Conflict(fmt.Sprintf("Satuan dasar tidak bisa diganti: group dipakai %d produk.", n))
			}
			if !provided {
				errs.Add("lines", "Kirim ulang daftar baris saat satuan dasar diganti.")
				return nil
			}
		}
		if provided {
			keep := map[string]bool{}
			for _, l := range lines {
				keep[l.unit] = true
			}
			if msg, err := removedLineUsage(c, id, keep); err != nil {
				return err
			} else if msg != "" {
				return crud.Conflict(msg)
			}
		}
		return nil
	},
	AfterSave: func(c *crud.Ctx, id string, m, old crud.Record, in map[string]any) error {
		base := m["base_unit_id"].(string)
		lines, provided, err := parseLines(c, in, base, validate.Errors{})
		if err != nil {
			return err
		}
		if provided || old == nil {
			if _, err := c.Q.Exec(c.Ctx, "DELETE FROM uom_group_lines WHERE uom_group_id=$1::uuid", id); err != nil {
				return err
			}
			all := append([]lineIn{{unit: base, altQty: decimal.NewFromInt(1), baseQty: decimal.NewFromInt(1)}}, lines...)
			for _, l := range all {
				lid, _ := uuid.NewV7()
				if _, err := c.Q.Exec(c.Ctx, `INSERT INTO uom_group_lines (id, uom_group_id, unit_id, alt_qty, base_qty, created_at, updated_at)
VALUES ($1,$2::uuid,$3::uuid,$4::numeric,$5::numeric,now() at time zone 'utc',now() at time zone 'utc')`, lid, id, l.unit, l.altQty.String(), l.baseQty.String()); err != nil {
					return err
				}
			}
		}
		return nil
	},
	Decorate: func(c *crud.Ctx, recs []crud.Record, _ bool) error {
		ids := make([]string, 0, len(recs))
		by := map[string]crud.Record{}
		for _, r := range recs {
			id := r["id"].(string)
			ids = append(ids, id)
			by[id] = r
			r["lines"] = []map[string]any{}
		}
		if len(ids) == 0 {
			return nil
		}
		rows, err := c.Q.Query(c.Ctx, `SELECT l.uom_group_id::text, l.id::text, l.unit_id::text, u.code, u.name, l.alt_qty, l.base_qty
FROM uom_group_lines l JOIN units u ON u.id=l.unit_id WHERE l.uom_group_id = ANY($1::uuid[])`, ids)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var gid, lid, uid, code, name string
			var alt, bq decimal.Decimal
			if err := rows.Scan(&gid, &lid, &uid, &code, &name, &alt, &bq); err != nil {
				return err
			}
			r := by[gid]
			factor := bq.Div(alt)
			r["lines"] = append(r["lines"].([]map[string]any), map[string]any{
				"id": lid, "unit_id": uid, "unit": map[string]string{"id": uid, "code": code, "name": name},
				"alt_qty": alt.StringFixed(3), "base_qty": bq.StringFixed(3), "factor": factor.StringFixed(3),
				"is_base": uid == r["base_unit_id"],
			})
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, r := range recs { // satuan dasar dulu, lalu menurut kode
			ls := r["lines"].([]map[string]any)
			sort.SliceStable(ls, func(i, j int) bool {
				if ls[i]["is_base"] != ls[j]["is_base"] {
					return ls[i]["is_base"].(bool)
				}
				return ls[i]["unit"].(map[string]string)["code"] < ls[j]["unit"].(map[string]string)["code"]
			})
		}
		return nil
	},
}

// Mount mendaftarkan rute katalog dasar.
func Mount(b *crud.Binder, r chi.Router, storageDir string) {
	MountProducts(b, r, storageDir)
	for _, res := range []*crud.Resource{&Units, &UomGroups, &ProductGroups, &Brands} {
		b.Mount(r, res)
	}
}
