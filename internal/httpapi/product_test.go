package httpapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// png1x1 adalah PNG sah terkecil; tipe dikenali dari isi file.
var png1x1, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNgYGD4DwABBAEAfbLI3wAAAABJRU5ErkJggg==")

func (e *env) upload(path, auth, filename string, content []byte) resp {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("image", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", e.url+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{Code: res.StatusCode}
	raw, _ := io.ReadAll(res.Body)
	out.Body = map[string]any{}
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

func (e *env) rawGet(path string) int {
	e.t.Helper()
	res, err := http.Get(e.url + path)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

func TestProdukAkses(t *testing.T) {
	e := setup(t)
	owner, manager, cashier := e.bearer("owner"), e.bearer("manager"), e.bearer("cashier")
	e.want(e.do("GET", "/api/products", manager, false, nil), 200)
	e.want(e.do("GET", "/api/products/lookup?code=x", manager, false, nil), 404)
	e.want(e.do("POST", "/api/products", manager, false, m{}), 403)
	e.want(e.do("POST", "/api/products/"+uuid.NewString()+"/image", manager, false, nil), 403)
	e.want(e.do("GET", "/api/products", cashier, false, nil), 403)
	e.want(e.do("GET", "/api/products", "", false, nil), 401)
	e.want(e.do("GET", "/api/products", owner, false, nil), 200)
}

func TestProduk(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	pcs, dus, pak := e.unit(owner, "PCS"), e.unit(owner, "DUS"), e.unit(owner, "PAK")
	pg := data(t, e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "MKN", "name": "Makanan"}), 201))["id"].(string)
	brand := data(t, e.want(e.do("POST", "/api/brands", owner, false, m{"code": "BRD", "name": "Merek"}), 201))["id"].(string)
	grp := data(t, e.want(e.do("POST", "/api/uom-groups", owner, false, m{"code": "DUS-PCS", "name": "Dus", "base_unit_id": pcs,
		"lines": []m{{"unit_id": dus, "alt_qty": "1", "base_qty": "12"}}}), 201))["id"].(string)

	base := func(o m) m {
		b := m{"product_group_id": pg, "unit_id": pcs, "sku": "SKU1", "name": "Mie Instan", "base_price": "3500"}
		for k, v := range o {
			b[k] = v
		}
		return b
	}

	// validasi
	for name, bad := range map[string]m{
		"sku":                base(m{"sku": ""}),
		"product_group_id":   base(m{"product_group_id": nil}),
		"name":               base(m{"name": strings.Repeat("x", 151)}),
		"base_price":         base(m{"base_price": "-1"}),
		"manage_by":          base(m{"manage_by": "lot"}),
		"unit_id":            base(m{"uom_group_id": grp, "unit_id": dus}), // harus = satuan dasar group
		"sales_unit_id":      base(m{"sales_unit_id": dus}),                // Dus bukan anggota (tanpa group)
		"barcodes.0.unit_id": base(m{"barcodes": []m{{"code": "1", "unit_id": dus}}}),
	} {
		if r := e.do("POST", "/api/products", owner, false, bad); r.Code != 422 || !errField(r, name) {
			t.Fatalf("%s: %d %v", name, r.Code, r.Body)
		}
	}

	// buat dengan group + barcode utama + barcode satuan
	created := data(t, e.want(e.do("POST", "/api/products", owner, false, base(m{
		"brand_id": brand, "barcode": "8990001", "uom_group_id": grp, "sales_unit_id": dus,
		"barcodes": []m{{"code": "8990012", "unit_id": dus, "note": "karton"}},
	})), 201))
	id := created["id"].(string)
	if created["base_price"] != "3500.00" || created["manage_by"] != "none" || created["track_stock"] != true || created["image_url"] != nil {
		t.Fatalf("default: %v", created)
	}
	bcs := data(t, e.want(e.do("GET", "/api/products/"+id, owner, false, nil), 200))["barcodes"].([]any)
	if len(bcs) != 1 || bcs[0].(map[string]any)["factor"] != "12.000" || bcs[0].(map[string]any)["unit"].(map[string]any)["code"] != "DUS" {
		t.Fatalf("barcodes: %v", bcs)
	}
	bcID := bcs[0].(map[string]any)["id"].(string)

	// unik lintas products.barcode dan product_barcodes.code, dua arah
	e.want(e.do("POST", "/api/products", owner, false, base(m{"sku": "S2", "barcode": "8990001"})), 422)
	if r := e.do("POST", "/api/products", owner, false, base(m{"sku": "S2", "barcode": "8990012"})); r.Code != 422 || !errField(r, "barcode") {
		t.Fatalf("barcode utama = barcode satuan produk lain: %d %v", r.Code, r.Body)
	}
	if r := e.do("POST", "/api/products", owner, false, base(m{"sku": "S2", "barcodes": []m{{"code": "8990001", "unit_id": pcs}}})); r.Code != 422 || !errField(r, "barcodes.0.code") {
		t.Fatalf("barcode satuan = barcode utama produk lain: %d %v", r.Code, r.Body)
	}
	if r := e.do("POST", "/api/products", owner, false, base(m{"sku": "S2", "barcodes": []m{{"code": "8990012", "unit_id": pcs}}})); r.Code != 422 || !errField(r, "barcodes.0.code") {
		t.Fatalf("barcode satuan dobel: %d %v", r.Code, r.Body)
	}
	if r := e.do("POST", "/api/products", owner, false, base(m{"sku": "S2", "barcode": "B1", "barcodes": []m{{"code": "B1", "unit_id": pcs}}})); r.Code != 422 || !errField(r, "barcodes.0.code") {
		t.Fatalf("barcode satuan = barcode utama sendiri: %d %v", r.Code, r.Body)
	}
	if r := e.do("POST", "/api/products", owner, false, base(m{"sku": "S2", "barcodes": []m{{"code": "X", "unit_id": pcs}, {"code": "X", "unit_id": pcs}}})); r.Code != 422 || !errField(r, "barcodes.1.code") {
		t.Fatalf("kode dobel dalam request: %d %v", r.Code, r.Body)
	}

	// lookup: barcode utama (satuan dasar, faktor 1) dan barcode satuan (Dus, faktor 12)
	lk := data(t, e.want(e.do("GET", "/api/products/lookup?code=8990001", owner, false, nil), 200))
	if lk["factor"] != "1.000" || lk["unit"].(map[string]any)["code"] != "PCS" || lk["product"].(map[string]any)["id"] != id {
		t.Fatalf("lookup utama: %v", lk)
	}
	lk = data(t, e.want(e.do("GET", "/api/products/lookup?code=8990012", owner, false, nil), 200))
	if lk["factor"] != "12.000" || lk["unit"].(map[string]any)["code"] != "DUS" || lk["product"].(map[string]any)["id"] != id {
		t.Fatalf("lookup satuan: %v", lk)
	}
	e.want(e.do("GET", "/api/products/lookup?code=tidakada", owner, false, nil), 404)
	e.want(e.do("GET", "/api/products/lookup", owner, false, nil), 422)

	// ganti isi barcodes (replace-set, boleh menukar kode); tanpa `barcodes` baris tetap
	put := func(o m) resp { return e.do("PUT", "/api/products/"+id, owner, false, base(o)) }
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "barcodes": []m{{"id": bcID, "code": "8990013", "unit_id": dus}, {"code": "8990012", "unit_id": pcs}}}), 200)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "name": "Mie Goreng"}), 200)
	got := data(t, e.want(e.do("GET", "/api/products/"+id, owner, false, nil), 200))
	if len(got["barcodes"].([]any)) != 2 || got["name"] != "Mie Goreng" {
		t.Fatalf("setelah replace: %v", got)
	}
	// group dilepas sementara barcode Dus masih ada → ditolak
	if r := put(m{"barcode": "8990001", "uom_group_id": nil, "sales_unit_id": nil}); r.Code != 422 || !errField(r, "barcodes") {
		t.Fatalf("lepas group: %d %v", r.Code, r.Body)
	}
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "barcodes": []m{}}), 200)

	// baris SAP tidak bisa diubah/dihapus
	sapID := uuid.NewString()
	e.sql("INSERT INTO product_barcodes (id, company_id, product_id, unit_id, code, source, created_at, updated_at) VALUES ($1,$2,$3,$4,'SAP1','sap',now(),now())", sapID, e.seed.Company, id, pcs)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "barcodes": []m{}}), 409)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "barcodes": []m{{"id": sapID, "code": "UBAH", "unit_id": pcs}}}), 409)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "barcodes": []m{{"id": sapID, "code": "SAP1", "unit_id": pcs}, {"code": "L1", "unit_id": dus}}}), 200)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "barcodes": []m{{"id": sapID, "code": "SAP1", "unit_id": pcs}}}), 200) // lokal dihapus, SAP utuh
	if lk := data(t, e.want(e.do("GET", "/api/products/lookup?code=SAP1", owner, false, nil), 200)); lk["product"].(map[string]any)["id"] != id {
		t.Fatalf("SAP hilang: %v", lk)
	}

	// manage_by: bebas diubah bila tidak ada batch/serial/stok dan SAP mati; terkunci bila ada
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "manage_by": "batch"}), 200)
	e.sql("UPDATE company_details SET sap_enabled=true")
	if r := put(m{"barcode": "8990001", "uom_group_id": grp, "manage_by": "serial"}); r.Code != 422 || !errField(r, "manage_by") {
		t.Fatalf("SAP aktif: %d %v", r.Code, r.Body)
	}
	e.sql("UPDATE company_details SET sap_enabled=false")
	e.sql("INSERT INTO product_batches (id, company_id, product_id, batch_number, created_at, updated_at) VALUES ($1,$2,$3,'B1',now(),now())", uuid.NewString(), e.seed.Company, id)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "manage_by": "none"}), 409)
	e.want(put(m{"barcode": "8990001", "uom_group_id": grp, "manage_by": "batch", "name": "Tetap batch"}), 200) // tidak berubah → boleh

	// daftar: filter, pencarian (sku/nama/barcode utama/barcode satuan), urutan
	e.want(e.do("POST", "/api/products", owner, false, base(m{"sku": "A0", "name": "Air", "unit_id": pak, "brand_id": nil})), 201)
	count := func(q string) float64 {
		return e.want(e.do("GET", "/api/products?"+q, owner, false, nil), 200).Body["meta"].(map[string]any)["total"].(float64)
	}
	for q, want := range map[string]float64{"": 2, "q=tetap": 1, "q=8990001": 1, "q=SAP1": 1, "q=air": 1, "brand_id=" + brand: 1, "product_group_id=" + pg: 2, "manage_by=batch": 1, "is_active=false": 0} {
		if got := count(q); got != want {
			t.Fatalf("%q: %v, want %v", q, got, want)
		}
	}
	first := e.want(e.do("GET", "/api/products?sort=sku&dir=asc", owner, false, nil), 200).Body["data"].([]any)[0].(map[string]any)
	if first["sku"] != "A0" || first["product_group"].(map[string]any)["code"] != "MKN" {
		t.Fatalf("urutan/expand: %v", first)
	}

	// hapus ditolak bila ada batch, item price list
	if r := e.want(e.do("DELETE", "/api/products/"+id, owner, false, nil), 409); !strings.Contains(r.Body["message"].(string), "batch") {
		t.Fatalf("pesan: %v", r.Body)
	}
	e.sql("DELETE FROM product_batches")
	pl := uuid.NewString()
	e.sql("INSERT INTO price_lists (id, company_id, code, name, created_at, updated_at) VALUES ($1,$2,'PL','Harga',now(),now())", pl, e.seed.Company)
	e.sql("INSERT INTO price_list_items (id, price_list_id, product_id, price, min_qty, created_at, updated_at) VALUES ($1,$2,$3,1000,1,now(),now())", uuid.NewString(), pl, id)
	if r := e.want(e.do("DELETE", "/api/products/"+id, owner, false, nil), 409); !strings.Contains(r.Body["message"].(string), "price list") {
		t.Fatalf("pesan: %v", r.Body)
	}
	e.sql("DELETE FROM price_list_items")
	e.sql("INSERT INTO promotions (id, company_id, code, name, product_id, created_at, updated_at) VALUES ($1,$2,'PR','Promo',$3,now(),now())", uuid.NewString(), e.seed.Company, id)
	e.want(e.do("DELETE", "/api/products/"+id, owner, false, nil), 409) // promosi
	e.sql("DELETE FROM promotions")

	// gambar
	img := "/api/products/" + id + "/image"
	if r := e.upload(img, owner, "x.png", []byte("bukan gambar")); r.Code != 422 || !errField(r, "image") {
		t.Fatalf("bukan gambar: %d %v", r.Code, r.Body)
	}
	if r := e.upload(img, owner, "palsu.png", append([]byte("<?php "), png1x1...)); r.Code != 422 {
		t.Fatalf("isi bukan gambar meski ekstensi png: %d", r.Code)
	}
	if r := e.upload(img, owner, "besar.png", append(append([]byte{}, png1x1...), make([]byte, 2<<20)...)); r.Code != 422 || !errField(r, "image") {
		t.Fatalf("> 2 MB: %d %v", r.Code, r.Body)
	}
	e.want(e.upload("/api/products/"+uuid.NewString()+"/image", owner, "a.png", png1x1), 404)
	up := data(t, e.want(e.upload(img, owner, "a.png", png1x1), 200))
	url1 := up["image_url"].(string)
	if !strings.HasPrefix(url1, "/media/products/") || !strings.HasSuffix(url1, ".png") {
		t.Fatalf("image_url: %v", up)
	}
	file1 := filepath.Join(e.storage, filepath.FromSlash(strings.TrimPrefix(url1, "/media/")))
	if _, err := os.Stat(file1); err != nil {
		t.Fatal("file tidak tersimpan:", err)
	}
	if e.rawGet(url1) != 200 {
		t.Fatal("gambar tidak tersaji")
	}
	for _, p := range []string{"/media/", "/media/products/", "/media/products", "/media/../../go.mod", "/media/%2e%2e/%2e%2e/go.mod", "/media/products/%2e%2e%2f%2e%2e%2fgo.mod", "/media/products/x.txt", "/media/products/tidakada.png"} {
		if c := e.rawGet(p); c != 404 && c != 301 {
			t.Fatalf("%s harus 404, dapat %d", p, c)
		}
	}
	// unggah ulang mengganti file lama
	url2 := data(t, e.want(e.upload(img, owner, "b.png", png1x1), 200))["image_url"].(string)
	if url2 == url1 || e.rawGet(url1) != 404 || e.rawGet(url2) != 200 {
		t.Fatalf("ganti gambar: %s -> %s", url1, url2)
	}
	if d := data(t, e.want(e.do("DELETE", img, owner, false, nil), 200)); d["image_url"] != nil || e.rawGet(url2) != 404 {
		t.Fatalf("hapus gambar: %v", d)
	}

	// hapus produk: barcode satuan ikut terhapus, file gambar dibersihkan
	url3 := data(t, e.want(e.upload(img, owner, "c.png", png1x1), 200))["image_url"].(string)
	e.want(e.do("DELETE", "/api/products/"+id, owner, false, nil), 204)
	if e.rawGet(url3) != 404 {
		t.Fatal("file gambar produk terhapus harus ikut hilang")
	}
	var n int
	if err := e.pool.QueryRow(t.Context(), "SELECT count(*) FROM product_barcodes").Scan(&n); err != nil || n != 0 {
		t.Fatalf("barcode yatim: %d %v", n, err)
	}
	var logs int
	_ = e.pool.QueryRow(t.Context(), "SELECT count(*) FROM activity_logs WHERE action IN ('product.created','product.updated','product.deleted','product.image_updated','product.image_removed')").Scan(&logs)
	if logs < 8 {
		t.Fatalf("audit produk: %d", logs)
	}
}

func TestProdukIsolasiCompany(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	other, g, u, p := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	e.sql("INSERT INTO company_details (id, name, created_at, updated_at) VALUES ($1,'PT Lain', now(), now())", other)
	e.sql("INSERT INTO product_groups (id, company_id, code, name, created_at, updated_at) VALUES ($1,$2,'G','G',now(),now())", g, other)
	e.sql("INSERT INTO units (id, company_id, code, name, created_at, updated_at) VALUES ($1,$2,'U','U',now(),now())", u, other)
	e.sql("INSERT INTO products (id, company_id, product_group_id, unit_id, sku, barcode, name, created_at, updated_at) VALUES ($1,$2,$3,$4,'LAIN','LAINBC','Lain',now(),now())", p, other, g, u)

	e.want(e.do("GET", "/api/products/"+p.String(), owner, false, nil), 404)
	e.want(e.do("PUT", "/api/products/"+p.String(), owner, false, m{"sku": "x"}), 404)
	e.want(e.do("DELETE", "/api/products/"+p.String(), owner, false, nil), 404)
	e.want(e.upload("/api/products/"+p.String()+"/image", owner, "a.png", png1x1), 404)
	e.want(e.do("GET", "/api/products/lookup?code=LAINBC", owner, false, nil), 404)

	// grup/satuan company lain ditolak, dan barcode company lain tidak mengganggu unik
	pcs := e.unit(owner, "PCS")
	pg := data(t, e.want(e.do("POST", "/api/product-groups", owner, false, m{"code": "MKN", "name": "Makanan"}), 201))["id"].(string)
	if r := e.do("POST", "/api/products", owner, false, m{"product_group_id": g.String(), "unit_id": pcs, "sku": "S", "name": "x"}); r.Code != 422 || !errField(r, "product_group_id") {
		t.Fatalf("grup company lain: %d %v", r.Code, r.Body)
	}
	e.want(e.do("POST", "/api/products", owner, false, m{"product_group_id": pg, "unit_id": pcs, "sku": "LAIN", "barcode": "LAINBC", "name": "x"}), 201)
}
