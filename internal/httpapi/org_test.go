package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type m = map[string]any

func data(t *testing.T, r resp) m {
	t.Helper()
	d, ok := r.Body["data"].(map[string]any)
	if !ok {
		t.Fatalf("tidak ada data: %d %v", r.Code, r.Body)
	}
	return d
}

func (e *env) want(r resp, code int) resp {
	e.t.Helper()
	if r.Code != code {
		e.t.Fatalf("code %d, want %d: %v", r.Code, code, r.Body)
	}
	return r
}

func errField(r resp, field string) bool {
	errs, _ := r.Body["errors"].(map[string]any)
	_, ok := errs[field]
	return ok
}

// otherCompany membuat company kedua dengan satu tax group, untuk uji isolasi.
func (e *env) otherCompany() (company, taxGroup uuid.UUID) {
	e.t.Helper()
	company, taxGroup = uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO company_details (id, name, created_at, updated_at) VALUES ($1,'PT Lain', now(), now())", []any{company}},
		{"INSERT INTO tax_groups (id, company_id, code, name, rate, created_at, updated_at) VALUES ($1,$2,'LAIN','Lain',5, now(), now())", []any{taxGroup, company}},
	} {
		if _, err := e.pool.Exec(context.Background(), q.sql, q.args...); err != nil {
			e.t.Fatal(err)
		}
	}
	return
}

func TestOrgAkses(t *testing.T) {
	e := setup(t)
	owner, manager, cashier := e.bearer("owner"), e.bearer("manager"), e.bearer("cashier")
	for _, p := range []string{"tax-groups", "currencies", "warehouses", "stores", "users"} {
		e.want(e.do("GET", "/api/"+p, manager, false, nil), 200)
		e.want(e.do("POST", "/api/"+p, manager, false, m{}), 403)
		e.want(e.do("GET", "/api/"+p, cashier, false, nil), 403)
		e.want(e.do("POST", "/api/"+p, cashier, false, m{}), 403)
		e.want(e.do("GET", "/api/"+p, "", false, nil), 401)
		e.want(e.do("GET", "/api/"+p, owner, false, nil), 200)
	}
	// profil perusahaan: tertutup total bagi store_manager
	e.want(e.do("GET", "/api/company", manager, false, nil), 403)
	e.want(e.do("PUT", "/api/company", manager, false, m{}), 403)
	e.want(e.do("GET", "/api/company", owner, false, nil), 200)
}

func TestTaxGroups(t *testing.T) {
	e := setup(t)
	owner, manager := e.bearer("owner"), e.bearer("manager")
	_, otherTG := e.otherCompany()

	r := e.want(e.do("POST", "/api/tax-groups", owner, false, m{"code": "PPN", "name": "PPN 11%", "rate": "11"}), 201)
	d := data(t, r)
	id := d["id"].(string)
	if d["rate"] != "11.00" || d["is_active"] != true {
		t.Fatalf("rate harus string 2 desimal: %v", d)
	}
	e.want(e.do("POST", "/api/tax-groups", owner, false, m{"code": "PPN", "name": "dobel", "rate": "1"}), 422)
	if r := e.do("POST", "/api/tax-groups", owner, false, m{"name": "x", "rate": "101"}); r.Code != 422 || !errField(r, "code") || !errField(r, "rate") {
		t.Fatalf("validasi: %d %v", r.Code, r.Body)
	}
	e.want(e.do("PUT", "/api/tax-groups/"+id, owner, false, m{"code": "PPN", "name": "PPN baru", "rate": "12.5"}), 200)
	if g := data(t, e.do("GET", "/api/tax-groups/"+id, manager, false, nil)); g["name"] != "PPN baru" || g["rate"] != "12.50" {
		t.Fatalf("setelah ubah: %v", g)
	}
	list := e.want(e.do("GET", "/api/tax-groups?q=ppn", owner, false, nil), 200)
	if meta := list.Body["meta"].(map[string]any); meta["total"] != float64(1) || meta["per_page"] != float64(25) {
		t.Fatalf("meta: %v", meta)
	}

	// isolasi company: id milik company lain = 404, bukan 403
	e.want(e.do("GET", "/api/tax-groups/"+otherTG.String(), owner, false, nil), 404)
	e.want(e.do("PUT", "/api/tax-groups/"+otherTG.String(), owner, false, m{"code": "LAIN", "name": "x", "rate": "1"}), 404)
	e.want(e.do("DELETE", "/api/tax-groups/"+otherTG.String(), owner, false, nil), 404)
	if n := len(e.do("GET", "/api/tax-groups", owner, false, nil).Body["data"].([]any)); n != 1 {
		t.Fatalf("list bocor lintas company: %d", n)
	}

	// dipakai company: tidak bisa dihapus
	e.want(e.do("PUT", "/api/company", owner, false, m{"name": "PT Uji", "base_currency_code": "IDR", "timezone": "Asia/Jakarta"}), 422) // IDR belum ada
	e.want(e.do("POST", "/api/currencies", owner, false, m{"code": "IDR", "symbol": "Rp", "decimal_places": 0}), 201)
	e.want(e.do("PUT", "/api/company", owner, false, m{"name": "PT Uji", "base_currency_code": "IDR", "timezone": "Asia/Jakarta", "ppn_enabled": true, "tax_group_id": id}), 200)
	if r := e.want(e.do("DELETE", "/api/tax-groups/"+id, owner, false, nil), 409); r.Body["message"] == nil {
		t.Fatal("pesan 409 wajib ada")
	}
	e.want(e.do("PUT", "/api/company", owner, false, m{"name": "PT Uji", "base_currency_code": "IDR", "timezone": "Asia/Jakarta", "ppn_enabled": false, "tax_group_id": nil}), 200)
	e.want(e.do("DELETE", "/api/tax-groups/"+id, owner, false, nil), 204)
	e.want(e.do("GET", "/api/tax-groups/"+id, owner, false, nil), 404)

	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM activity_logs WHERE action IN ('tax_group.created','tax_group.updated','tax_group.deleted')").Scan(&n)
	if n != 3 {
		t.Fatalf("audit tax_group = %d, want 3", n)
	}
}

func TestCurrenciesDanDenominasi(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")

	for _, bad := range []string{"us", "USDX", "U$D", "12A"} {
		if r := e.do("POST", "/api/currencies", owner, false, m{"code": bad, "symbol": "$"}); r.Code != 422 {
			t.Fatalf("kode %q: %d", bad, r.Code)
		}
	}
	d := data(t, e.want(e.do("POST", "/api/currencies", owner, false, m{"code": "usd", "symbol": "$", "decimal_places": 2}), 201))
	if d["code"] != "USD" {
		t.Fatalf("kode harus uppercase: %v", d["code"])
	}
	id := d["id"].(string)
	e.want(e.do("POST", "/api/currencies", owner, false, m{"code": "USD", "symbol": "$"}), 422) // unik
	if r := e.do("POST", "/api/currencies", owner, false, m{"code": "EUR", "symbol": "E", "decimal_places": 5}); r.Code != 422 || !errField(r, "decimal_places") {
		t.Fatalf("decimal_places: %d %v", r.Code, r.Body)
	}

	for _, v := range []string{"100", "5", "50"} {
		e.want(e.do("POST", "/api/currencies/"+id+"/denominations", owner, false, m{"value": v}), 201)
	}
	e.want(e.do("POST", "/api/currencies/"+id+"/denominations", owner, false, m{"value": "50"}), 422)    // unik
	e.want(e.do("POST", "/api/currencies/"+id+"/denominations", owner, false, m{"value": "0.001"}), 422) // < 0.01
	det := data(t, e.want(e.do("GET", "/api/currencies/"+id, owner, false, nil), 200))
	dens := det["denominations"].([]any)
	if len(dens) != 3 || dens[0].(map[string]any)["value"] != "5.00" || dens[2].(map[string]any)["value"] != "100.00" {
		t.Fatalf("denominasi harus terurut naik: %v", dens)
	}
	e.want(e.do("DELETE", "/api/currencies/"+id+"/denominations/"+dens[0].(map[string]any)["id"].(string), owner, false, nil), 204)
	e.want(e.do("DELETE", "/api/currencies/"+id+"/denominations/"+uuid.NewString(), owner, false, nil), 404)

	// dipakai gerai: tidak bisa dihapus
	e.want(e.do("POST", "/api/stores", owner, false, m{"code": "S1", "name": "Satu", "pic_name": "Budi", "currency": "USD"}), 201)
	if r := e.want(e.do("DELETE", "/api/currencies/"+id, owner, false, nil), 409); r.Body["message"] == nil {
		t.Fatal("pesan 409 wajib ada")
	}
}

func TestWarehousesDanStores(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	e.want(e.do("POST", "/api/currencies", owner, false, m{"code": "IDR", "symbol": "Rp", "decimal_places": 0}), 201)

	wh := data(t, e.want(e.do("POST", "/api/warehouses", owner, false, m{"code": "WH1", "name": "Gudang 1"}), 201))
	if wh["is_retail"] != true || wh["is_active"] != true {
		t.Fatalf("default: %v", wh)
	}
	e.want(e.do("POST", "/api/warehouses", owner, false, m{"code": "WH1", "name": "dobel"}), 422)
	whID := wh["id"].(string)
	for k := range wh {
		if strings.HasPrefix(k, "sap_") {
			t.Fatalf("kolom sap_* tidak boleh tampil: %s", k)
		}
	}

	store := m{"code": "S1", "name": "Gerai Satu", "pic_name": "Budi", "currency": "IDR", "warehouse_id": whID, "opening_float": "100000"}
	s := data(t, e.want(e.do("POST", "/api/stores", owner, false, store), 201))
	sid := s["id"].(string)
	if s["opening_float"] != "100000.00" || s["ppn_rate"] != "11.00" {
		t.Fatalf("angka store: %v", s)
	}
	if s["warehouse"].(map[string]any)["code"] != "WH1" {
		t.Fatalf("expand warehouse: %v", s["warehouse"])
	}
	// gudang unik antar store
	dup := m{"code": "S2", "name": "Dua", "pic_name": "Ani", "currency": "IDR", "warehouse_id": whID}
	if r := e.do("POST", "/api/stores", owner, false, dup); r.Code != 422 || !errField(r, "warehouse_id") {
		t.Fatalf("gudang dobel: %d %v", r.Code, r.Body)
	}
	for field, bad := range map[string]m{
		"pic_name": {"code": "S3", "name": "x", "currency": "IDR"},
		"currency": {"code": "S3", "name": "x", "pic_name": "a", "currency": "ZZZ"},
		"ppn_rate": {"code": "S3", "name": "x", "pic_name": "a", "currency": "IDR", "ppn_rate": "101"},
	} {
		if r := e.do("POST", "/api/stores", owner, false, bad); r.Code != 422 || !errField(r, field) {
			t.Fatalf("%s: %d %v", field, r.Code, r.Body)
		}
	}
	e.want(e.do("GET", "/api/stores/"+sid, owner, false, nil), 200)

	e.want(e.do("DELETE", "/api/warehouses/"+whID, owner, false, nil), 409) // dipakai gerai
	e.want(e.do("DELETE", "/api/stores/"+sid, owner, false, nil), 204)
	e.want(e.do("DELETE", "/api/warehouses/"+whID, owner, false, nil), 204)
}

func TestUsers(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	cashierTok := e.bearer("cashier")

	newUser := m{"name": "Kasir Baru", "username": "kasir_baru", "role": "cashier", "store_id": e.seed.StoreA.String(), "password": "Rahasia12345"}
	r := e.want(e.do("POST", "/api/users", owner, false, newUser), 201)
	d := data(t, r)
	if _, ok := d["password"]; ok {
		t.Fatal("password tidak boleh ada di JSON")
	}
	uid := d["id"].(string)
	var hash string
	_ = e.pool.QueryRow(context.Background(), "SELECT password FROM users WHERE id=$1::uuid", uid).Scan(&hash)
	if len(hash) != 60 || hash[4:7] != "12$" {
		t.Fatalf("bcrypt cost 12: %q", hash)
	}

	for field, bad := range map[string]m{
		"password": {"name": "x", "username": "u1", "role": "owner", "password": "pendek1"},
		"store_id": {"name": "x", "username": "u2", "role": "cashier", "password": "Rahasia12345"},
		"username": {"name": "x", "username": "KASIR_BARU", "role": "owner", "password": "Rahasia12345"}, // unik tanpa kapital
		"role":     {"name": "x", "username": "u3", "role": "admin", "password": "Rahasia12345"},
	} {
		if r := e.do("POST", "/api/users", owner, false, bad); r.Code != 422 || !errField(r, field) {
			t.Fatalf("%s: %d %v", field, r.Code, r.Body)
		}
	}
	if r := e.do("POST", "/api/users", owner, false, m{"name": "x", "username": "u4", "role": "owner", "password": "hurufsajasaja"}); r.Code != 422 {
		t.Fatalf("password tanpa angka: %d", r.Code)
	}

	// nonaktifkan = cabut semua token
	e.want(e.do("GET", "/api/auth/me", cashierTok, false, nil), 200)
	cashier := m{"name": "cashier", "username": "cashier", "role": "cashier", "store_id": e.seed.StoreA.String(), "is_active": false}
	e.want(e.do("PUT", "/api/users/"+e.seed.Cashier.String(), owner, false, cashier), 200)
	e.want(e.do("GET", "/api/auth/me", cashierTok, false, nil), 401)
	var tokens int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM personal_access_tokens WHERE tokenable_id=$1", e.seed.Cashier).Scan(&tokens)
	if tokens != 0 {
		t.Fatalf("token tersisa: %d", tokens)
	}

	// reset password: validasi, konfirmasi, lalu token dicabut dan password baru berlaku
	mgrTok := e.bearer("manager")
	rp := "/api/users/" + e.seed.Manager.String() + "/reset-password"
	e.want(e.do("POST", rp, owner, false, m{"password": "pendek", "password_confirmation": "pendek"}), 422)
	e.want(e.do("POST", rp, owner, false, m{"password": "Baru123456", "password_confirmation": "beda"}), 422)
	e.want(e.do("POST", rp, owner, false, m{"password": "Baru123456", "password_confirmation": "Baru123456"}), 204)
	e.want(e.do("GET", "/api/auth/me", mgrTok, false, nil), 401)
	e.want(e.do("POST", "/api/auth/login", "", false, m{"username": "manager", "password": "Baru123456", "client": "api"}), 200)

	// owner tidak boleh menghapus/menonaktifkan diri sendiri, dan owner aktif terakhir dilindungi
	me := e.seed.Owner.String()
	e.want(e.do("DELETE", "/api/users/"+me, owner, false, nil), 409)
	e.want(e.do("PUT", "/api/users/"+me, owner, false, m{"name": "owner", "username": "owner", "role": "owner", "is_active": false}), 409)
	e.want(e.do("PUT", "/api/users/"+me, owner, false, m{"name": "owner", "username": "owner", "role": "store_manager", "store_id": e.seed.StoreA.String()}), 409)
	e.want(e.do("DELETE", "/api/users/"+uid, owner, false, nil), 204)
}

func TestCompanyProfile(t *testing.T) {
	e := setup(t)
	owner := e.bearer("owner")
	e.want(e.do("POST", "/api/currencies", owner, false, m{"code": "IDR", "symbol": "Rp", "decimal_places": 0}), 201)
	base := func(over m) m {
		b := m{"name": "PT Uji Jaya", "base_currency_code": "IDR", "timezone": "Asia/Jakarta", "return_window_days": 30}
		for k, v := range over {
			b[k] = v
		}
		return b
	}
	d := data(t, e.want(e.do("PUT", "/api/company", owner, false, base(m{"email": "a@b.co", "npwp_number": "01.2345.678901.2345"})), 200))
	if d["name"] != "PT Uji Jaya" || d["email"] != "a@b.co" {
		t.Fatalf("%v", d)
	}
	if r := e.do("PUT", "/api/company", owner, false, base(m{"email": "a@b"})); r.Code != 422 || !errField(r, "email") {
		t.Fatalf("email tanpa titik harus ditolak: %d %v", r.Code, r.Body)
	}
	if d["npwp_number"] != "0123456789012345" {
		t.Fatalf("npwp harus tersimpan 16 digit tanpa pemisah: %v", d["npwp_number"])
	}
	for field, over := range map[string]m{
		"npwp_number":        {"npwp_number": "01.234"},
		"name":               {"name": ""},
		"email":              {"email": "bukan-email"},
		"timezone":           {"timezone": "Mars/Olympus"},
		"base_currency_code": {"base_currency_code": "XXX"},
		"tax_group_id":       {"ppn_enabled": true},
		"return_window_days": {"return_window_days": 4000},
	} {
		if r := e.do("PUT", "/api/company", owner, false, base(over)); r.Code != 422 || !errField(r, field) {
			t.Fatalf("%s: %d %v", field, r.Code, r.Body)
		}
	}
	if got := data(t, e.want(e.do("GET", "/api/company", owner, false, nil), 200))["name"]; got != "PT Uji Jaya" {
		t.Fatalf("setelah gagal validasi data tidak boleh berubah: %v", got)
	}
}
