package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/httpapi"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/database"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("EASYPOS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("EASYPOS_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE sessions, activity_logs, users, stores, companies CASCADE`); err != nil {
		t.Fatal(err)
	}
	return pool
}

type fixture struct {
	pool      *pgxpool.Pool
	srv       http.Handler
	svc       *identity.Service
	companyID string
	storeID   string
}

func setup(t *testing.T) fixture {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	f := fixture{pool: pool}
	if err := pool.QueryRow(ctx, `INSERT INTO companies (name) VALUES ('T') RETURNING id`).Scan(&f.companyID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO stores (company_id, code, name) VALUES ($1,'S1','Gerai 1') RETURNING id`, f.companyID).Scan(&f.storeID); err != nil {
		t.Fatal(err)
	}
	s := httpapi.New(pool, false, fstest.MapFS{"index.html": {Data: []byte("<html>spa</html>")}})
	f.svc = s.Identity()
	f.srv = s.Handler()
	return f
}

func (f fixture) mk(t *testing.T, username, role string) *identity.User {
	t.Helper()
	var store *string
	if role != identity.RoleOwner {
		store = &f.storeID
	}
	u, err := f.svc.Create(context.Background(), audit.Actor{Name: "test"}, identity.CreateParams{
		CompanyID: f.companyID, Name: username, Username: username, Password: "secret-pass", Role: role, StoreID: store})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (f fixture) login(t *testing.T, username, password string) (*httptest.ResponseRecorder, []*http.Cookie) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	req.RemoteAddr = "10.0.0." + username + ":1"
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, req)
	return rec, rec.Result().Cookies()
}

func (f fixture) get(path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, req)
	return rec
}

func TestCashierForbiddenFromAdmin(t *testing.T) {
	f := setup(t)
	f.mk(t, "1", identity.RoleCashier)
	f.mk(t, "2", identity.RoleOwner)

	rec, ck := f.login(t, "1", "secret-pass")
	if rec.Code != 200 {
		t.Fatalf("cashier login = %d", rec.Code)
	}
	if got := f.get("/api/admin/ping", ck).Code; got != http.StatusForbidden {
		t.Fatalf("cashier on admin route = %d, want 403", got)
	}
	_, ck = f.login(t, "2", "secret-pass")
	if got := f.get("/api/admin/ping", ck).Code; got != 200 {
		t.Fatalf("owner on admin route = %d, want 200", got)
	}
	if got := f.get("/api/admin/ping", nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("anonymous on admin route = %d, want 401", got)
	}
}

func TestInactiveAccountRejectedAndSessionRevoked(t *testing.T) {
	f := setup(t)
	f.mk(t, "9", identity.RoleOwner)
	m := f.mk(t, "3", identity.RoleStoreManager)
	_, ck := f.login(t, "3", "secret-pass")
	if f.get("/api/auth/me", ck).Code != 200 {
		t.Fatal("me should work before deactivation")
	}
	if err := f.svc.Update(context.Background(), audit.Actor{Name: "t"}, m.ID, identity.RoleStoreManager, false); err != nil {
		t.Fatal(err)
	}
	if got := f.get("/api/auth/me", ck).Code; got != http.StatusUnauthorized {
		t.Fatalf("deactivated user's session = %d, want 401", got)
	}
	if rec, _ := f.login(t, "3", "secret-pass"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("inactive login = %d, want 401", rec.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	f := setup(t)
	f.mk(t, "4", identity.RoleOwner)
	var last int
	for i := 0; i < 6; i++ {
		rec, _ := f.login(t, "4", "wrong")
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("6th attempt = %d, want 429", last)
	}
}

func TestLastOwnerCannotBeRemoved(t *testing.T) {
	f := setup(t)
	o := f.mk(t, "5", identity.RoleOwner)
	ctx, a := context.Background(), audit.Actor{Name: "t"}
	if err := f.svc.Update(ctx, a, o.ID, identity.RoleOwner, false); err != identity.ErrLastOwner {
		t.Fatalf("deactivate last owner: %v", err)
	}
	if err := f.svc.Update(ctx, a, o.ID, identity.RoleStoreManager, true); err != identity.ErrStoreRequired && err != identity.ErrLastOwner {
		t.Fatalf("demote last owner: %v", err)
	}
}

func TestConcurrentOwnerDeactivationLeavesOne(t *testing.T) {
	f := setup(t)
	a, b := f.mk(t, "6", identity.RoleOwner), f.mk(t, "7", identity.RoleOwner)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = f.svc.Update(context.Background(), audit.Actor{Name: "t"}, id, identity.RoleOwner, false)
		}()
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("expected exactly one deactivation to succeed, got %d (%v)", ok, errs)
	}
}

func TestStaffRequiresStore(t *testing.T) {
	f := setup(t)
	_, err := f.svc.Create(context.Background(), audit.Actor{}, identity.CreateParams{
		CompanyID: f.companyID, Name: "x", Username: "x", Password: "p", Role: identity.RoleCashier})
	if err != identity.ErrStoreRequired {
		t.Fatalf("got %v, want ErrStoreRequired", err)
	}
}

func TestMutationsAreAudited(t *testing.T) {
	f := setup(t)
	f.mk(t, "8", identity.RoleOwner)
	rec, _ := f.login(t, "8", "secret-pass")
	var dto map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&dto)
	if dto["role"] != "owner" {
		t.Fatalf("login dto: %v", dto)
	}
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM activity_logs WHERE action IN ('user.create','auth.login')`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("audit rows = %d, err %v; want 2", n, err)
	}
}

func TestSPAFallback(t *testing.T) {
	f := setup(t)
	rec := f.get("/some/client/route", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "spa") {
		t.Fatalf("spa fallback = %d %q", rec.Code, rec.Body.String())
	}
}
