package httpapi_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPriceListAkses(t *testing.T) {
	e := setup(t)
	owner, manager, cashier := e.bearer("owner"), e.bearer("manager"), e.bearer("cashier")
	pl := data(t, e.want(e.do("POST", "/api/price-lists", owner, false, m{"code": "PL", "name": "Harga"}), 201))["id"].(string)
	item := "/api/price-lists/" + pl + "/items"
	e.want(e.do("GET", "/api/price-lists", manager, false, nil), 200)
	e.want(e.do("GET", item, manager, false, nil), 200)
	e.want(e.do("GET", "/api/price-lists/"+pl+"/resolve?product_id="+uuid.NewString(), manager, false, nil), 422)
	e.want(e.do("POST", "/api/price-lists", manager, false, m{"code": "X", "name": "x"}), 403)
	e.want(e.do("POST", item, manager, false, m{}), 403)
	e.want(e.do("PUT", item+"/"+uuid.NewString(), manager, false, m{}), 403)
	e.want(e.do("DELETE", item+"/"+uuid.NewString(), manager, false, nil), 403)
	e.want(e.do("GET", "/api/price-lists", cashier, false, nil), 403)
	e.want(e.do("GET", item, "", false, nil), 401)
}

func TestPriceListHeader(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	mk := func(code string, over m) m {
		b := m{"code": code, "name": "Harga " + code}
		for k, v := range over {
			b[k] = v
		}
		return data(t, e.want(e.do("POST", "/api/price-lists", owner, false, b), 201))
	}
	a := mk("A", m{"is_default": true})
	b := mk("B", m{"is_default": true, "store_id": e.seed.StoreA.String(), "effective_from": "2026-01-01", "effective_until": "2026-12-31"})
	get := func(id string) m { return data(t, e.want(e.do("GET", "/api/price-lists/"+id, owner, false, nil), 200)) }
	if get(a["id"].(string))["is_default"] != false || get(b["id"].(string))["is_default"] != true {
		t.Fatal("default harus tunggal per company: B menggantikan A")
	}
	if got := get(b["id"].(string)); got["store"] == nil || got["effective_from"] != "2026-01-01" || got["item_count"] != float64(0) {
		t.Fatalf("detail: %v", got)
	}

	for field, over := range map[string]m{
		"code":            {"code": "A"},
		"name":            {"name": ""},
		"effective_until": {"effective_from": "2026-05-01", "effective_until": "2026-04-30"},
		"store_id":        {"store_id": uuid.NewString()},
	} {
		body := m{"code": "Z", "name": "z"}
		for k, v := range over {
			body[k] = v
		}
		if r := e.do("POST", "/api/price-lists", owner, false, body); r.Code != 422 || !errField(r, field) {
			t.Fatalf("%s: %d %v", field, r.Code, r.Body)
		}
	}
	// until sama dengan from boleh (satu hari)
	mk("C", m{"effective_from": "2026-05-01", "effective_until": "2026-05-01"})

	// dipakai gerai (pivot ber-CASCADE) → 409; setelah dilepas boleh dihapus
	e.sql("INSERT INTO store_price_lists (store_id, price_list_id, is_default, created_at, updated_at) VALUES ($1,$2,true,now(),now())", e.seed.StoreA, b["id"])
	if r := e.do("DELETE", "/api/price-lists/"+b["id"].(string), owner, false, nil); r.Code != 409 {
		t.Fatalf("dipakai gerai: %d %v", r.Code, r.Body)
	}
	e.sql("DELETE FROM store_price_lists")
	e.want(e.do("DELETE", "/api/price-lists/"+b["id"].(string), owner, false, nil), 204)
}

func TestPriceListItemDanResolve(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	pcs, dus, pak := e.unit(owner, "PCS"), e.unit(owner, "DUS"), e.unit(owner, "PAK")
	pg := data(t, e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "G", "name": "G"}), 201))["id"].(string)
	grp := data(t, e.want(e.do("POST", "/api/uom-groups", owner, false, m{"code": "G1", "name": "G1", "base_unit_id": pcs,
		"lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "12"}}}), 201))["id"].(string)
	prod := func(sku string, over m) string {
		b := m{"product_group_id": pg, "unit_id": pcs, "sku": sku, "name": "Produk " + sku, "base_price": "1000"}
		for k, v := range over {
			b[k] = v
		}
		return data(t, e.want(e.do("POST", "/api/products", owner, false, b), 201))["id"].(string)
	}
	withGroup, plain := prod("S1", m{"uom_group_id": grp}), prod("S2", nil)
	pl := data(t, e.want(e.do("POST", "/api/price-lists", owner, false, m{"code": "PL", "name": "Harga"}), 201))["id"].(string)
	base := "/api/price-lists/" + pl + "/items"
	add := func(over m) resp {
		b := m{"product_id": withGroup, "min_qty": 1, "price": "900"}
		for k, v := range over {
			b[k] = v
		}
		return e.do("POST", base, owner, false, b)
	}

	// satuan dasar boleh ditulis eksplisit, disimpan NULL; harga per satuan dasar tampil untuk Dus
	it := data(t, e.want(add(m{"unit_id": pcs}), 201))
	if it["unit_id"] != nil || it["price"] != "900.00" || it["price_per_base"] != "900.00" {
		t.Fatalf("item dasar: %v", it)
	}
	dusItem := data(t, e.want(add(m{"unit_id": dus, "price": "10800"}), 201))
	if dusItem["price_per_base"] != "900.00" || dusItem["factor"] != "12.000" {
		t.Fatalf("item dus: %v", dusItem)
	}
	e.want(add(m{"min_qty": 10, "price": "800"}), 201)

	// validasi
	for name, tc := range map[string]struct {
		over  m
		field string
	}{
		"duplikat satuan dasar":    {m{"unit_id": nil}, "min_qty"},
		"duplikat dasar eksplisit": {m{"unit_id": pcs}, "min_qty"},
		"duplikat dus":             {m{"unit_id": dus}, "min_qty"},
		"satuan bukan anggota":     {m{"unit_id": pak, "min_qty": 2}, "unit_id"},
		"produk wajib":             {m{"product_id": ""}, "product_id"},
		"produk tak ada":           {m{"product_id": uuid.NewString()}, "product_id"},
		"harga negatif":            {m{"price": "-1", "min_qty": 3}, "price"},
		"harga wajib":              {m{"price": nil, "min_qty": 3}, "price"},
		"min_qty 0":                {m{"min_qty": 0}, "min_qty"},
		"min_qty desimal":          {m{"min_qty": 1.5}, "min_qty"},
	} {
		if r := add(tc.over); r.Code != 422 || !errField(r, tc.field) {
			t.Fatalf("%s: %d %v", name, r.Code, r.Body)
		}
	}
	// produk tanpa group: satuan non-dasar ditolak
	if r := add(m{"product_id": plain, "unit_id": dus}); r.Code != 422 || !errField(r, "unit_id") {
		t.Fatalf("tanpa group: %d %v", r.Code, r.Body)
	}
	e.want(add(m{"product_id": plain, "price": "500"}), 201)

	// edit: ubah harga; tabrakan dengan tier lain → 422; produk tidak bisa diganti
	e.want(e.do("PUT", base+"/"+dusItem["id"].(string), owner, false, m{"price": "11000"}), 200)
	if r := e.do("PUT", base+"/"+dusItem["id"].(string), owner, false, m{"unit_id": nil}); r.Code != 422 || !errField(r, "min_qty") {
		t.Fatalf("edit tabrakan: %d %v", r.Code, r.Body)
	}
	e.want(e.do("PUT", base+"/"+dusItem["id"].(string), owner, false, m{"product_id": plain, "min_qty": 5}), 200)
}

func TestPriceListResolveEndpoint(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	pcs, dus := e.unit(owner, "PCS"), e.unit(owner, "DUS")
	pg := data(t, e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "G", "name": "G"}), 201))["id"].(string)
	grp := data(t, e.want(e.do("POST", "/api/uom-groups", owner, false, m{"code": "G1", "name": "G1", "base_unit_id": pcs,
		"lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "12"}}}), 201))["id"].(string)
	p := data(t, e.want(e.do("POST", "/api/products", owner, false, m{"product_group_id": pg, "unit_id": pcs, "sku": "S1", "name": "P", "base_price": "1000", "uom_group_id": grp}), 201))["id"].(string)
	pl := data(t, e.want(e.do("POST", "/api/price-lists", owner, false, m{"code": "PL", "name": "Harga", "effective_from": "2026-01-01", "effective_until": "2026-12-31"}), 201))["id"].(string)
	items := "/api/price-lists/" + pl + "/items"
	e.want(e.do("POST", items, owner, false, m{"product_id": p, "min_qty": 1, "price": "900"}), 201)
	e.want(e.do("POST", items, owner, false, m{"product_id": p, "min_qty": 10, "price": "800"}), 201)
	e.want(e.do("POST", items, owner, false, m{"product_id": p, "unit_id": dus, "min_qty": 1, "price": "10000"}), 201)

	res := func(q string) m {
		return data(t, e.want(e.do("GET", "/api/price-lists/"+pl+"/resolve?product_id="+p+q, owner, false, nil), 200))
	}
	for _, tc := range []struct{ q, price, src string }{
		{"&qty=1&date=2026-06-01", "900.00", "price_list_base"},
		{"&qty=10&date=2026-06-01", "800.00", "price_list_base"},
		{"&unit_id=" + dus + "&qty=1&date=2026-06-01", "10000.00", "price_list_unit"},
		{"&unit_id=" + pcs + "&qty=1&date=2026-06-01", "900.00", "price_list_base"},
		{"&unit_id=" + dus + "&qty=1&date=2027-01-01", "12000.00", "base_price"}, // di luar masa berlaku
		{"&qty=1&date=2025-12-31", "1000.00", "base_price"},
	} {
		if got := res(tc.q); got["unit_price"] != tc.price || got["source"] != tc.src {
			t.Fatalf("%s: %v", tc.q, got)
		}
	}
	if got := res("&unit_id=" + dus + "&date=2026-06-01"); got["factor"] != "12.000" {
		t.Fatalf("faktor: %v", got)
	}
	for name, q := range map[string]string{
		"qty nol":         "&qty=0",
		"qty bukan angka": "&qty=abc",
		"tanggal salah":   "&date=01-06-2026",
		"satuan asing":    "&unit_id=" + uuid.NewString(),
	} {
		e.want(e.do("GET", "/api/price-lists/"+pl+"/resolve?product_id="+p+q, owner, false, nil), 422)
		_ = name
	}
	e.want(e.do("GET", "/api/price-lists/"+pl+"/resolve", owner, false, nil), 422)
	e.want(e.do("GET", "/api/price-lists/"+uuid.NewString()+"/resolve?product_id="+p, owner, false, nil), 404)

	// price list nonaktif → harga dasar
	e.want(e.do("PUT", "/api/price-lists/"+pl, owner, false, m{"is_active": false}), 200)
	if got := res("&qty=1&date=2026-06-01"); got["source"] != "base_price" {
		t.Fatalf("nonaktif: %v", got)
	}

	// produk dipakai item price list tidak bisa dihapus; item dihapus → bisa; hapus price list ikut item
	e.want(e.do("DELETE", "/api/products/"+p, owner, false, nil), 409)
}

func TestPriceListDaftarItemDanIsolasi(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	pcs := e.unit(owner, "PCS")
	pg := data(t, e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "G", "name": "G"}), 201))["id"].(string)
	var ids []string
	for _, sku := range []string{"A1", "B2", "C3"} {
		ids = append(ids, data(t, e.want(e.do("POST", "/api/products", owner, false, m{"product_group_id": pg, "unit_id": pcs, "sku": sku, "name": "Barang " + sku, "base_price": "100"}), 201))["id"].(string))
	}
	pl := data(t, e.want(e.do("POST", "/api/price-lists", owner, false, m{"code": "PL", "name": "Harga"}), 201))["id"].(string)
	items := "/api/price-lists/" + pl + "/items"
	var first string
	for i, id := range ids {
		it := data(t, e.want(e.do("POST", items, owner, false, m{"product_id": id, "min_qty": 1, "price": "50"}), 201))
		if i == 0 {
			first = it["id"].(string)
		}
	}
	list := e.want(e.do("GET", items+"?per_page=2", owner, false, nil), 200)
	meta := list.Body["meta"].(map[string]any)
	if rows := list.Body["data"].([]any); len(rows) != 2 || meta["total"] != float64(3) || meta["last_page"] != float64(2) {
		t.Fatalf("paginasi: %v", list.Body)
	}
	if rows := e.want(e.do("GET", items+"?q=b2", owner, false, nil), 200).Body["data"].([]any); len(rows) != 1 {
		t.Fatalf("cari: %v", rows)
	}

	// item milik price list lain tidak bisa diubah lewat price list ini; company lain tidak terlihat
	other := data(t, e.want(e.do("POST", "/api/price-lists", owner, false, m{"code": "PL2", "name": "Lain"}), 201))["id"].(string)
	e.want(e.do("PUT", "/api/price-lists/"+other+"/items/"+first, owner, false, m{"price": "1"}), 404)
	e.want(e.do("DELETE", "/api/price-lists/"+other+"/items/"+first, owner, false, nil), 404)
	co, otherPL, otherProd := uuid.New(), uuid.New(), uuid.New()
	e.sql("INSERT INTO company_details (id, name, created_at, updated_at) VALUES ($1,'PT Lain', now(), now())", co)
	e.sql("INSERT INTO price_lists (id, company_id, code, name, created_at, updated_at) VALUES ($1,$2,'X','X',now(),now())", otherPL, co)
	e.want(e.do("GET", "/api/price-lists/"+otherPL.String(), owner, false, nil), 404)
	e.want(e.do("GET", "/api/price-lists/"+otherPL.String()+"/items", owner, false, nil), 404)
	e.want(e.do("POST", "/api/price-lists/"+otherPL.String()+"/items", owner, false, m{"product_id": ids[0], "price": "1"}), 404)
	if r := e.do("POST", items, owner, false, m{"product_id": otherProd.String(), "price": "1"}); r.Code != 422 || !errField(r, "product_id") {
		t.Fatalf("produk lintas company: %d %v", r.Code, r.Body)
	}

	// hapus item lalu hapus price list ikut menghapus sisa item (CASCADE)
	e.want(e.do("DELETE", items+"/"+first, owner, false, nil), 204)
	e.want(e.do("DELETE", items+"/"+first, owner, false, nil), 404)
	e.want(e.do("DELETE", "/api/price-lists/"+pl, owner, false, nil), 204)
	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM price_list_items WHERE price_list_id=$1", pl).Scan(&n); err != nil || n != 0 {
		t.Fatalf("sisa item: %d %v", n, err)
	}
	// produk kini bebas dihapus
	e.want(e.do("DELETE", "/api/products/"+ids[0], owner, false, nil), 204)
}
