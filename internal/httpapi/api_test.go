package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jfebiyanto/easyPOS-Go/internal/httpapi"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/testutil"
)

type env struct {
	t       *testing.T
	pool    *pgxpool.Pool
	seed    testutil.Seed
	url     string
	storage string // akar file unggahan
}

func newServer(t *testing.T, pool *pgxpool.Pool, limit int, storage string) *httptest.Server {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{Pool: pool, Identity: identity.New(pool), Loc: loc, LoginLimit: limit, StorageDir: storage}))
	t.Cleanup(srv.Close)
	return srv
}

type resp struct {
	Code    int
	Body    map[string]any
	Cookies []*http.Cookie
}

// do mengirim request; auth: "" | "Bearer xxx" | "cookie:xxx"; csrf menambah header X-EasyPOS-Client.
func (e *env) do(method, path, auth string, csrf bool, body any) resp {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.url+path, rd)
	req.Header.Set("Content-Type", "application/json")
	switch {
	case strings.HasPrefix(auth, "cookie:"):
		name, val, _ := strings.Cut(strings.TrimPrefix(auth, "cookie:"), "=")
		req.AddCookie(&http.Cookie{Name: name, Value: val})
	case auth != "":
		req.Header.Set("Authorization", auth)
	}
	if csrf {
		req.Header.Set(httpapi.CSRFHeader, "1")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	out := resp{Code: res.StatusCode, Cookies: res.Cookies()}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

func (e *env) login(user, client string) resp {
	e.t.Helper()
	return e.do("POST", "/api/auth/login", "", false, map[string]string{"username": user, "password": testutil.Password, "client": client})
}

// token login klien api dan kembalikan header Authorization.
func (e *env) bearer(user string) string {
	e.t.Helper()
	r := e.login(user, "api")
	if r.Code != 200 {
		e.t.Fatalf("login %s: %d %v", user, r.Code, r.Body)
	}
	return "Bearer " + r.Body["token"].(string)
}

func setup(t *testing.T) *env {
	pool, _, seed := testutil.NewDB(t)
	dir := t.TempDir()
	srv := newServer(t, pool, 1000, dir)
	return &env{t: t, pool: pool, seed: seed, url: srv.URL, storage: dir}
}

func TestLogin(t *testing.T) {
	e := setup(t)

	t.Run("sukses dengan hash $2y$ buatan PHP (klien api)", func(t *testing.T) {
		r := e.login("owner", "api")
		if r.Code != 200 {
			t.Fatalf("code %d %v", r.Code, r.Body)
		}
		tok, _ := r.Body["token"].(string)
		if !strings.Contains(tok, "|") || len(tok[strings.Index(tok, "|")+1:]) != 40 {
			t.Fatalf("format token salah: %q", tok)
		}
		if len(r.Cookies) != 0 {
			t.Fatal("klien api tidak boleh dapat cookie")
		}
		var hash string
		_ = e.pool.QueryRow(context.Background(), "SELECT token FROM personal_access_tokens ORDER BY id DESC LIMIT 1").Scan(&hash)
		if len(hash) != 64 || strings.Contains(tok, hash) {
			t.Fatal("token harus tersimpan sebagai sha256 hex")
		}
		// owner tanpa store_id: gerai aktif = gerai pertama menurut kode
		var code string
		_ = e.pool.QueryRow(context.Background(), "SELECT s.code FROM personal_access_tokens p JOIN stores s ON s.id = p.active_store_id ORDER BY p.id DESC LIMIT 1").Scan(&code)
		if code != "AAA" {
			t.Fatalf("gerai aktif owner = %q, want AAA", code)
		}
	})

	t.Run("klien backoffice: token di cookie httpOnly, bukan di body", func(t *testing.T) {
		r := e.login("owner", "backoffice")
		if r.Code != 200 || r.Body["token"] != nil {
			t.Fatalf("code %d body %v", r.Code, r.Body)
		}
		if len(r.Cookies) != 1 || r.Cookies[0].Name != httpapi.CookieBO || !r.Cookies[0].HttpOnly || r.Cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatalf("cookie salah: %+v", r.Cookies)
		}
	})

	for name, tc := range map[string]struct{ user, pass string }{
		"password salah":  {"owner", "salah"},
		"user tidak ada":  {"tidakada", testutil.Password},
		"user nonaktif":   {"nonaktif", testutil.Password},
		"password kosong": {"owner", ""},
		"username kosong": {"", testutil.Password},
	} {
		t.Run(name, func(t *testing.T) {
			r := e.do("POST", "/api/auth/login", "", false, map[string]string{"username": tc.user, "password": tc.pass, "client": "api"})
			if tc.pass == "" || tc.user == "" {
				if r.Code != 422 {
					t.Fatalf("code %d", r.Code)
				}
				return
			}
			if r.Code != 422 || r.Body["message"] != "Username atau password salah, atau akun nonaktif." {
				t.Fatalf("code %d body %v", r.Code, r.Body)
			}
		})
	}

	t.Run("kasir tidak boleh login backoffice", func(t *testing.T) {
		if r := e.login("cashier", "backoffice"); r.Code != 403 {
			t.Fatalf("code %d", r.Code)
		}
		if r := e.login("cashier", "pos"); r.Code != 200 {
			t.Fatalf("kasir pos: code %d", r.Code)
		}
	})

	t.Run("activity log login dan gagal", func(t *testing.T) {
		var ok, failed int
		_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FILTER (WHERE action='auth.login'), count(*) FILTER (WHERE action='auth.login_failed') FROM activity_logs").Scan(&ok, &failed)
		if ok == 0 || failed == 0 {
			t.Fatalf("log login=%d gagal=%d", ok, failed)
		}
	})
}

func TestRateLimitLogin(t *testing.T) {
	pool, _, _ := testutil.NewDB(t)
	e := &env{t: t, pool: pool, url: newServer(t, pool, 5, t.TempDir()).URL}
	for i := 1; i <= 6; i++ {
		r := e.do("POST", "/api/auth/login", "", false, map[string]string{"username": "owner", "password": "salah", "client": "api"})
		want := 422
		if i == 6 {
			want = 429
		}
		if r.Code != want {
			t.Fatalf("percobaan %d: code %d, want %d", i, r.Code, want)
		}
	}
	// username lain pada IP yang sama tidak terkena limit yang sama
	if r := e.do("POST", "/api/auth/login", "", false, map[string]string{"username": "manager", "password": "salah", "client": "api"}); r.Code != 422 {
		t.Fatalf("username lain: code %d", r.Code)
	}
}

func TestTokenDanSesi(t *testing.T) {
	e := setup(t)

	t.Run("me dengan bearer", func(t *testing.T) {
		r := e.do("GET", "/api/auth/me", e.bearer("manager"), false, nil)
		if r.Code != 200 || r.Body["role"] != "store_manager" || r.Body["username"] != "manager" {
			t.Fatalf("code %d %v", r.Code, r.Body)
		}
		as := r.Body["active_store"].(map[string]any)
		if as["code"] != "AAA" || r.Body["company"].(map[string]any)["name"] != "PT Uji" {
			t.Fatalf("active_store/company salah: %v", r.Body)
		}
		perm := r.Body["permissions"].(map[string]any)["admin.numberings.index"].(map[string]any)
		if perm["visible"] != false {
			t.Fatal("manager tidak boleh melihat numberings")
		}
	})

	t.Run("tanpa token atau token palsu = 401", func(t *testing.T) {
		for _, a := range []string{"", "Bearer 1|salah", "Bearer abc", "Bearer 999999|xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"} {
			if r := e.do("GET", "/api/auth/me", a, false, nil); r.Code != 401 {
				t.Fatalf("auth %q: code %d", a, r.Code)
			}
		}
	})

	t.Run("logout mencabut token dan menghapus cookie", func(t *testing.T) {
		auth := e.bearer("owner")
		r := e.do("POST", "/api/auth/logout", auth, false, nil)
		if r.Code != 204 {
			t.Fatalf("logout code %d", r.Code)
		}
		if r := e.do("GET", "/api/auth/me", auth, false, nil); r.Code != 401 {
			t.Fatalf("token masih berlaku setelah logout: %d", r.Code)
		}
	})

	t.Run("token kedaluwarsa", func(t *testing.T) {
		auth := e.bearer("owner")
		id := strings.TrimPrefix(strings.SplitN(auth, "|", 2)[0], "Bearer ")
		if _, err := e.pool.Exec(context.Background(), "UPDATE personal_access_tokens SET expires_at = now() - interval '1 hour' WHERE id = $1::bigint", id); err != nil {
			t.Fatal(err)
		}
		if r := e.do("GET", "/api/auth/me", auth, false, nil); r.Code != 401 {
			t.Fatalf("code %d", r.Code)
		}
	})

	t.Run("user dinonaktifkan setelah login = 401", func(t *testing.T) {
		auth := e.bearer("cashier")
		_, _ = e.pool.Exec(context.Background(), "UPDATE users SET is_active=false WHERE id=$1", e.seed.Cashier)
		defer e.pool.Exec(context.Background(), "UPDATE users SET is_active=true WHERE id=$1", e.seed.Cashier) //nolint:errcheck
		if r := e.do("GET", "/api/auth/me", auth, false, nil); r.Code != 401 {
			t.Fatalf("code %d", r.Code)
		}
	})

	t.Run("cookie: GET boleh, POST tanpa header CSRF = 403, dengan header sukses", func(t *testing.T) {
		login := e.login("owner", "backoffice")
		ck := "cookie:" + login.Cookies[0].Name + "=" + login.Cookies[0].Value
		if r := e.do("GET", "/api/auth/me", ck, false, nil); r.Code != 200 {
			t.Fatalf("GET cookie: %d", r.Code)
		}
		if r := e.do("POST", "/api/auth/logout", ck, false, nil); r.Code != 403 {
			t.Fatalf("POST tanpa CSRF: %d", r.Code)
		}
		if r := e.do("GET", "/api/auth/me", ck, false, nil); r.Code != 200 {
			t.Fatal("sesi harus tetap hidup setelah 403 CSRF")
		}
		r := e.do("POST", "/api/auth/logout", ck, true, nil)
		if r.Code != 204 || len(r.Cookies) == 0 || r.Cookies[0].MaxAge >= 0 {
			t.Fatalf("logout dengan CSRF: %d cookies=%v", r.Code, r.Cookies)
		}
	})

	t.Run("bearer tidak butuh header CSRF", func(t *testing.T) {
		if r := e.do("POST", "/api/session/store", e.bearer("owner"), false, map[string]string{"store_id": e.seed.StoreB.String()}); r.Code != 200 {
			t.Fatalf("code %d %v", r.Code, r.Body)
		}
	})
}

func TestSwitchStore(t *testing.T) {
	e := setup(t)
	a, b := e.seed.StoreA.String(), e.seed.StoreB.String()
	active := func(auth string) string {
		r := e.do("GET", "/api/auth/me", auth, false, nil)
		return r.Body["active_store"].(map[string]any)["code"].(string)
	}
	count := func(auth string) int {
		r := e.do("GET", "/api/stores/accessible", auth, false, nil)
		return len(r.Body["stores"].([]any))
	}

	owner := e.bearer("owner")
	if count(owner) != 2 {
		t.Fatal("owner harus melihat 2 gerai")
	}
	if r := e.do("POST", "/api/session/store", owner, false, map[string]string{"store_id": b}); r.Code != 200 || active(owner) != "BBB" {
		t.Fatalf("owner pindah ke B: %d", r.Code)
	}
	if r := e.do("POST", "/api/session/store", owner, false, map[string]string{"store_id": "bukan-uuid"}); r.Code != 422 {
		t.Fatalf("uuid salah: %d", r.Code)
	}
	if r := e.do("POST", "/api/session/store", owner, false, map[string]string{"store_id": "00000000-0000-0000-0000-000000000001"}); r.Code != 403 {
		t.Fatalf("gerai tak ada: %d", r.Code)
	}
	for _, u := range []string{"manager", "cashier"} {
		auth := e.bearer(u)
		if count(auth) != 1 {
			t.Fatalf("%s hanya boleh melihat 1 gerai", u)
		}
		if r := e.do("POST", "/api/session/store", auth, false, map[string]string{"store_id": b}); r.Code != 403 {
			t.Fatalf("%s pindah ke B harus 403, dapat %d", u, r.Code)
		}
		if r := e.do("POST", "/api/session/store", auth, false, map[string]string{"store_id": a}); r.Code != 200 || active(auth) != "AAA" {
			t.Fatalf("%s ke gerainya sendiri: %d", u, r.Code)
		}
	}
}

func TestSettings(t *testing.T) {
	e := setup(t)
	owner, manager, cashier := e.bearer("owner"), e.bearer("manager"), e.bearer("cashier")

	r := e.do("GET", "/api/settings", manager, false, nil)
	if r.Code != 200 || r.Body["settings"].(map[string]any)["pos.serial_not_in_stock"] != "reject" {
		t.Fatalf("manager baca default: %d %v", r.Code, r.Body)
	}
	if r := e.do("GET", "/api/settings", cashier, false, nil); r.Code != 403 {
		t.Fatalf("kasir baca: %d", r.Code)
	}
	put := func(auth string, v any) resp {
		return e.do("PUT", "/api/settings", auth, false, map[string]any{"settings": map[string]any{"pos.serial_not_in_stock": v}})
	}
	if r := put(manager, "allow"); r.Code != 403 {
		t.Fatalf("manager tulis: %d", r.Code)
	}
	for _, bad := range []any{"sembarang", 5, nil, ""} {
		if r := put(owner, bad); r.Code != 422 {
			t.Fatalf("nilai %v: %d", bad, r.Code)
		}
	}
	if r := e.do("PUT", "/api/settings", owner, false, map[string]any{"settings": map[string]any{"kunci.asing": "x"}}); r.Code != 422 {
		t.Fatalf("kunci asing: %d", r.Code)
	}
	for _, ok := range []string{"manager_approval", "allow", "reject"} {
		r := put(owner, ok)
		if r.Code != 200 || r.Body["settings"].(map[string]any)["pos.serial_not_in_stock"] != ok {
			t.Fatalf("simpan %s: %d %v", ok, r.Code, r.Body)
		}
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM activity_logs WHERE action='settings.updated'").Scan(&n)
	if n != 3 {
		t.Fatalf("activity log settings.updated = %d, want 3", n)
	}
}

func TestNumberingsOwnerOnly(t *testing.T) {
	e := setup(t)
	owner, manager := e.bearer("owner"), e.bearer("manager")
	if r := e.do("GET", "/api/numberings", manager, false, nil); r.Code != 403 {
		t.Fatalf("manager: %d", r.Code)
	}
	if r := e.do("POST", "/api/numberings/sales/next", manager, false, nil); r.Code != 403 {
		t.Fatalf("manager next: %d", r.Code)
	}
	r := e.do("GET", "/api/numberings", owner, false, nil)
	if r.Code != 200 || len(r.Body["numberings"].([]any)) != 13 {
		t.Fatalf("owner list: %d %v", r.Code, r.Body)
	}
	body := map[string]any{"items": []map[string]any{{"transaction_type": "sales", "prefix": "INV", "padding": 5, "separator": "-", "reset_period": "never"}}}
	if r := e.do("PUT", "/api/numberings", owner, false, body); r.Code != 200 {
		t.Fatalf("put: %d %v", r.Code, r.Body)
	}
	if r := e.do("POST", "/api/numberings/sales/next", owner, false, nil); r.Code != 200 || r.Body["number"] != "INV-00001" {
		t.Fatalf("next: %d %v", r.Code, r.Body)
	}
	bad := map[string]any{"items": []map[string]any{{"transaction_type": "sales", "prefix": "X", "padding": 0, "separator": "-", "reset_period": "never"}}}
	if r := e.do("PUT", "/api/numberings", owner, false, bad); r.Code != 422 {
		t.Fatalf("padding 0: %d", r.Code)
	}
	if r := e.do("POST", "/api/numberings/xxx/next", owner, false, nil); r.Code != 404 {
		t.Fatalf("tipe asing: %d", r.Code)
	}
}

func TestCookieTerpisahBackofficeDanPOS(t *testing.T) {
	e := setup(t)
	bo, pos := e.login("owner", "backoffice"), e.login("owner", "pos")
	if bo.Cookies[0].Name != httpapi.CookieBO || pos.Cookies[0].Name != httpapi.CookiePOS {
		t.Fatalf("nama cookie salah: %v %v", bo.Cookies, pos.Cookies)
	}
	boC := "cookie:" + httpapi.CookieBO + "=" + bo.Cookies[0].Value
	posC := "cookie:" + httpapi.CookiePOS + "=" + pos.Cookies[0].Value
	if r := e.do("GET", "/api/auth/me", boC, false, nil); r.Code != 200 {
		t.Fatalf("bo di /api: %d", r.Code)
	}
	if r := e.do("GET", "/api/pos/auth/me", posC, false, nil); r.Code != 200 {
		t.Fatalf("pos di /api/pos: %d", r.Code)
	}
	// keduanya tetap valid; cookie yang salah rute tidak dipakai
	if r := e.do("GET", "/api/pos/auth/me", boC, false, nil); r.Code != 401 {
		t.Fatalf("cookie bo di /api/pos harus 401, dapat %d", r.Code)
	}
	if r := e.do("GET", "/api/auth/me", posC, false, nil); r.Code != 401 {
		t.Fatalf("cookie pos di /api harus 401, dapat %d", r.Code)
	}
	if r := e.do("GET", "/api/auth/me", boC, false, nil); r.Code != 200 {
		t.Fatal("sesi bo harus tetap hidup")
	}
	// Bearer didahulukan dan berlaku di kedua prefix
	if r := e.do("GET", "/api/pos/auth/me", e.bearer("owner"), false, nil); r.Code != 200 {
		t.Fatalf("bearer di /api/pos: %d", r.Code)
	}
	// logout POS tidak menghapus sesi backoffice
	if r := e.do("POST", "/api/pos/auth/logout", posC, true, nil); r.Code != 204 || r.Cookies[0].Name != httpapi.CookiePOS {
		t.Fatalf("logout pos: %d %v", r.Code, r.Cookies)
	}
	if r := e.do("GET", "/api/auth/me", boC, false, nil); r.Code != 200 {
		t.Fatal("sesi bo harus tetap hidup setelah logout pos")
	}
}

func TestRevalidasiGeraiAktif(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	active := func(auth string) any { return e.do("GET", "/api/auth/me", auth, false, nil).Body["active_store"] }

	mgr := e.bearer("manager")
	if _, err := e.pool.Exec(ctx, "UPDATE users SET store_id=$1 WHERE id=$2", e.seed.StoreB, e.seed.Manager); err != nil {
		t.Fatal(err)
	}
	if as, _ := active(mgr).(map[string]any); as == nil || as["code"] != "BBB" {
		t.Fatalf("gerai aktif manager harus mengikuti users.store_id: %v", active(mgr))
	}
	_, _ = e.pool.Exec(ctx, "UPDATE users SET store_id=NULL WHERE id=$1", e.seed.Manager)
	if active(mgr) != nil {
		t.Fatal("user tanpa gerai: active_store harus null")
	}
	if r := e.do("GET", "/api/pos/context", mgr, false, nil); r.Code != 403 {
		t.Fatalf("rute butuh gerai tanpa gerai: %d", r.Code)
	}
	owner := e.bearer("owner")
	if r := e.do("GET", "/api/pos/context", owner, false, nil); r.Code != 200 {
		t.Fatalf("owner dengan gerai: %d", r.Code)
	}
	_, _ = e.pool.Exec(ctx, "UPDATE stores SET is_active=false WHERE id=$1", e.seed.StoreA)
	if active(owner) != nil {
		t.Fatal("owner: gerai nonaktif harus dikosongkan")
	}
	var n int
	_ = e.pool.QueryRow(ctx, "SELECT count(*) FROM personal_access_tokens WHERE active_store_id IS NOT NULL AND tokenable_id=$1", e.seed.Owner).Scan(&n)
	if n != 0 {
		t.Fatalf("active_store_id owner harus NULL di DB, ada %d", n)
	}
}

func TestRevokeAllMenghapusTokenUser(t *testing.T) {
	e := setup(t)
	a, b := e.bearer("cashier"), e.bearer("cashier")
	if err := identity.RevokeAll(context.Background(), e.pool, e.seed.Cashier); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{a, b} {
		if r := e.do("GET", "/api/auth/me", tok, false, nil); r.Code != 401 {
			t.Fatalf("token harus tercabut: %d", r.Code)
		}
	}
}
