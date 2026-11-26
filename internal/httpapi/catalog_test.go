package httpapi_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func (e *env) unit(auth, code string) string {
	e.t.Helper()
	return data(e.t, e.want(e.do("POST", "/api/units", auth, false, m{"code": code, "name": "Satuan " + code}), 201))["id"].(string)
}

func (e *env) sql(q string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), q, args...); err != nil {
		e.t.Fatal(err)
	}
}

func TestCatalogAkses(t *testing.T) {
	e := setup(t)
	owner, manager, cashier := e.bearer("owner"), e.bearer("manager"), e.bearer("cashier")
	for _, p := range []string{"units", "uom-groups", "product-groups", "brands"} {
		e.want(e.do("GET", "/api/"+p, manager, false, nil), 200)
		e.want(e.do("POST", "/api/"+p, manager, false, m{}), 403)
		e.want(e.do("GET", "/api/"+p, cashier, false, nil), 403)
		e.want(e.do("GET", "/api/"+p, "", false, nil), 401)
		e.want(e.do("GET", "/api/"+p, owner, false, nil), 200)
	}
}

func TestUnitsGroupsBrands(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")

	// Units
	pcs := e.unit(owner, "PCS")
	e.want(e.do("POST", "/api/units", owner, false, m{"code": "PCS", "name": "dobel"}), 422)
	if r := e.do("POST", "/api/units", owner, false, m{"code": "ABCDEFGHIJKLMNOPQRSTU", "name": "x"}); r.Code != 422 || !errField(r, "code") {
		t.Fatalf("kode > 20: %d %v", r.Code, r.Body)
	}
	e.want(e.do("PUT", "/api/units/"+pcs, owner, false, m{"code": "PCS", "name": "Pieces"}), 200)

	// Product groups & brands
	pg := data(t, e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "MKN", "name": "Makanan", "description": "Semua makanan"}), 201))["id"].(string)
	e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "MKN", "name": "dobel"}), 422)
	e.want(e.do("POST", "/api/brands", owner, false, m{"code": "BRD", "name": "Merek"}), 201)
	e.want(e.do("POST", "/api/brands", owner, false, m{"code": "BRD", "name": "dobel"}), 422)
	if r := e.do("GET", "/api/product-groups?q=makan", owner, false, nil); r.Body["meta"].(map[string]any)["total"] != float64(1) {
		t.Fatalf("cari grup: %v", r.Body)
	}

	// UoM group: 1 Dus = 12 Pcs; baris satuan dasar yang dikirim diabaikan dan selalu 1:1
	dus, pak := e.unit(owner, "DUS"), e.unit(owner, "PAK")
	g := data(t, e.want(e.do("POST", "/api/uom-groups", owner, false, m{
		"code": "DUS-PCS", "name": "Dus ke Pcs", "base_unit_id": pcs,
		"lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "12"}, {"unit_id": pcs, "alt_qty": "5", "base_qty": "9"}},
	}), 201))
	gid := g["id"].(string)
	lines := g["lines"].([]any)
	if len(lines) != 2 {
		t.Fatalf("harus 2 baris (dasar + Dus): %v", lines)
	}
	first, second := lines[0].(map[string]any), lines[1].(map[string]any)
	if first["is_base"] != true || first["alt_qty"] != "1.000" || first["base_qty"] != "1.000" || first["factor"] != "1.000" {
		t.Fatalf("baris dasar harus 1:1 dan di urutan pertama: %v", first)
	}
	if second["factor"] != "12.000" || second["unit"].(map[string]any)["code"] != "DUS" || g["base_unit"].(map[string]any)["code"] != "PCS" {
		t.Fatalf("baris Dus: %v", second)
	}

	for name, bad := range map[string]m{
		"lines.0.unit_id":  {"code": "G1", "name": "x", "base_unit_id": pcs, "lines": []m{{"unit_id": uuid.NewString(), "alt_qty": "1", "base_qty": "2"}}},
		"lines.1.unit_id":  {"code": "G1", "name": "x", "base_unit_id": pcs, "lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "2"}, {"unit_id": dus, "alt_qty": "1", "base_qty": "3"}}},
		"lines.0.alt_qty":  {"code": "G1", "name": "x", "base_unit_id": pcs, "lines": []m{{"unit_id": dus, "alt_qty": "0", "base_qty": "2"}}},
		"lines.0.base_qty": {"code": "G1", "name": "x", "base_unit_id": pcs, "lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "1.2345"}}},
		"base_unit_id":     {"code": "G1", "name": "x"},
		"code":             {"code": "DUS-PCS", "name": "x", "base_unit_id": pcs},
	} {
		if r := e.do("POST", "/api/uom-groups", owner, false, bad); r.Code != 422 || !errField(r, name) {
			t.Fatalf("%s: %d %v", name, r.Code, r.Body)
		}
	}

	// ganti isi baris (replace-set): Dus jadi 24, tambah Pak 2 = 5 Pcs
	g = data(t, e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, m{
		"code": "DUS-PCS", "name": "Dus ke Pcs", "base_unit_id": pcs,
		"lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "24"}, {"unit_id": pak, "alt_qty": "2", "base_qty": "5"}},
	}), 200))
	ls := g["lines"].([]any)
	if len(ls) != 3 || ls[1].(map[string]any)["factor"] != "24.000" || ls[2].(map[string]any)["factor"] != "2.500" {
		t.Fatalf("setelah replace: %v", ls)
	}
	// tanpa `lines` di PUT: baris tetap
	g = data(t, e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, m{"code": "DUS-PCS", "name": "Nama baru", "base_unit_id": pcs}), 200))
	if len(g["lines"].([]any)) != 3 {
		t.Fatalf("baris hilang saat PUT tanpa lines: %v", g["lines"])
	}
	list := data(t, e.want(e.do("GET", "/api/uom-groups/"+gid, owner, false, nil), 200))
	if list["name"] != "Nama baru" {
		t.Fatalf("%v", list)
	}

	// satuan dipakai UoM group tidak bisa dihapus
	e.want(e.do("DELETE", "/api/units/"+dus, owner, false, nil), 409)
	e.want(e.do("DELETE", "/api/units/"+pcs, owner, false, nil), 409)

	// produk memakai group: barcode & price list item pada satuan Dus menahan penghapusan baris
	co, now := e.seed.Company, "now() at time zone 'utc'"
	prod, pl := uuid.New(), uuid.New()
	e.sql("INSERT INTO products (id, company_id, product_group_id, unit_id, sku, name, uom_group_id, created_at, updated_at) VALUES ($1,$2,$3::uuid,$4::uuid,'SKU1','Produk',$5::uuid,"+now+","+now+")", prod, co, pg, pcs, gid)
	e.sql("INSERT INTO product_barcodes (id, company_id, product_id, unit_id, code, created_at, updated_at) VALUES ($1,$2,$3,$4::uuid,'8990001',"+now+","+now+")", uuid.New(), co, prod, dus)
	e.sql("INSERT INTO price_lists (id, company_id, code, name, created_at, updated_at) VALUES ($1,$2,'PL','Harga',"+now+","+now+")", pl, co)
	e.sql("INSERT INTO price_list_items (id, price_list_id, product_id, price, min_qty, unit_id, created_at, updated_at) VALUES ($1,$2,$3,1000,1,$4::uuid,"+now+","+now+")", uuid.New(), pl, prod, pak)

	body := func(lines ...m) m {
		return m{"code": "DUS-PCS", "name": "Nama baru", "base_unit_id": pcs, "lines": lines}
	}
	dusLine, pakLine := m{"unit_id": dus, "alt_qty": "1", "base_qty": "24"}, m{"unit_id": pak, "alt_qty": "2", "base_qty": "5"}
	if r := e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, body(pakLine)), 409); r.Body["message"] == nil {
		t.Fatal("pesan 409 (barcode Dus) wajib ada")
	}
	e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, body(dusLine)), 409) // Pak dipakai price list item
	e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, body(dusLine, pakLine)), 200)
	// satuan dasar tidak bisa diganti, dan group yang dipakai produk tidak bisa dihapus
	e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, m{"code": "DUS-PCS", "name": "x", "base_unit_id": dus, "lines": []m{}}), 409)
	e.want(e.do("DELETE", "/api/uom-groups/"+gid, owner, false, nil), 409)

	// tanpa produk: ganti satuan dasar wajib kirim ulang lines; lalu hapus group
	e.sql("DELETE FROM product_barcodes")
	e.sql("DELETE FROM price_list_items")
	e.sql("DELETE FROM products")
	if r := e.do("PUT", "/api/uom-groups/"+gid, owner, false, m{"code": "DUS-PCS", "name": "x", "base_unit_id": dus}); r.Code != 422 || !errField(r, "lines") {
		t.Fatalf("ganti dasar tanpa lines: %d %v", r.Code, r.Body)
	}
	g = data(t, e.want(e.do("PUT", "/api/uom-groups/"+gid, owner, false, m{"code": "DUS-PCS", "name": "x", "base_unit_id": dus,
		"lines": []m{{"unit_id": pcs, "alt_qty": "12", "base_qty": "1"}}}), 200))
	if ls := g["lines"].([]any); len(ls) != 2 || ls[0].(map[string]any)["unit"].(map[string]any)["code"] != "DUS" {
		t.Fatalf("dasar baru harus di depan: %v", ls)
	}
	e.want(e.do("DELETE", "/api/uom-groups/"+gid, owner, false, nil), 204)
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM uom_group_lines").Scan(&n)
	if n != 0 {
		t.Fatalf("baris yatim tersisa: %d", n)
	}
	e.want(e.do("DELETE", "/api/units/"+pak, owner, false, nil), 204)
}

func TestCatalogIsolasiCompany(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	other, otherUnit := uuid.New(), uuid.New()
	e.sql("INSERT INTO company_details (id, name, created_at, updated_at) VALUES ($1,'PT Lain', now(), now())", other)
	e.sql("INSERT INTO units (id, company_id, code, name, created_at, updated_at) VALUES ($1,$2,'LAIN','Lain', now(), now())", otherUnit, other)

	e.want(e.do("GET", "/api/units/"+otherUnit.String(), owner, false, nil), 404)
	e.want(e.do("DELETE", "/api/units/"+otherUnit.String(), owner, false, nil), 404)
	// satuan milik company lain tidak boleh jadi satuan dasar maupun baris
	if r := e.do("POST", "/api/uom-groups", owner, false, m{"code": "X", "name": "x", "base_unit_id": otherUnit.String()}); r.Code != 422 || !errField(r, "base_unit_id") {
		t.Fatalf("base unit lintas company: %d %v", r.Code, r.Body)
	}
	pcs := e.unit(owner, "PCS")
	if r := e.do("POST", "/api/uom-groups", owner, false, m{"code": "X", "name": "x", "base_unit_id": pcs,
		"lines": []m{{"unit_id": otherUnit.String(), "alt_qty": "1", "base_qty": "2"}}}); r.Code != 422 || !errField(r, "lines.0.unit_id") {
		t.Fatalf("baris lintas company: %d %v", r.Code, r.Body)
	}
}
