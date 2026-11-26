package httpapi_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jfebiyanto/easyPOS-Go/internal/testutil"
)

func tokenID(auth string) string {
	return strings.TrimPrefix(strings.SplitN(auth, "|", 2)[0], "Bearer ")
}

func (e *env) idle(auth, interval string) {
	e.t.Helper()
	_, err := e.pool.Exec(context.Background(),
		"UPDATE personal_access_tokens SET last_used_at = (now() at time zone 'utc') - $2::interval WHERE id = $1::bigint", tokenID(auth), interval)
	if err != nil {
		e.t.Fatal(err)
	}
}

func TestIdleLock(t *testing.T) {
	e := setup(t)
	owner, manager := e.bearer("owner"), e.bearer("manager")

	t.Run("default 15 menit di /auth/me", func(t *testing.T) {
		r := e.do("GET", "/api/auth/me", owner, false, nil)
		if r.Code != 200 || r.Body["idle_lock_minutes"] != float64(15) || r.Body["locked"] != false {
			t.Fatalf("%d %v", r.Code, r.Body)
		}
	})

	t.Run("di bawah batas idle tetap aktif", func(t *testing.T) {
		e.idle(manager, "14 minutes")
		if r := e.do("GET", "/api/settings", manager, false, nil); r.Code != 200 {
			t.Fatalf("code %d", r.Code)
		}
	})

	t.Run("lewat batas idle: 423, hanya me/unlock/logout boleh", func(t *testing.T) {
		e.idle(manager, "16 minutes")
		r := e.do("GET", "/api/settings", manager, false, nil)
		if r.Code != 423 || r.Body["locked"] != true {
			t.Fatalf("code %d %v", r.Code, r.Body)
		}
		if r := e.do("POST", "/api/auth/activity", manager, false, nil); r.Code != 423 {
			t.Fatalf("heartbeat tidak boleh membuka kunci: %d", r.Code)
		}
		if r := e.do("GET", "/api/auth/me", manager, false, nil); r.Code != 200 || r.Body["locked"] != true {
			t.Fatalf("me saat terkunci: %d %v", r.Code, r.Body)
		}
		// tetap terkunci walau last_used_at dimundurkan kembali: kunci tersimpan di server
		e.idle(manager, "0 seconds")
		if r := e.do("GET", "/api/settings", manager, false, nil); r.Code != 423 {
			t.Fatalf("kunci harus menetap: %d", r.Code)
		}
		var n int
		_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM activity_logs WHERE action='auth.locked'").Scan(&n)
		if n != 1 {
			t.Fatalf("audit auth.locked = %d, want 1", n)
		}
	})

	t.Run("unlock: password salah ditolak, benar membuka", func(t *testing.T) {
		if r := e.do("POST", "/api/auth/unlock", manager, false, map[string]string{"password": "salah"}); r.Code != 422 {
			t.Fatalf("code %d", r.Code)
		}
		if r := e.do("GET", "/api/settings", manager, false, nil); r.Code != 423 {
			t.Fatalf("masih harus terkunci: %d", r.Code)
		}
		if r := e.do("POST", "/api/auth/unlock", manager, false, map[string]string{"password": testutil.Password}); r.Code != 204 {
			t.Fatalf("code %d", r.Code)
		}
		if r := e.do("GET", "/api/settings", manager, false, nil); r.Code != 200 {
			t.Fatalf("setelah unlock: %d", r.Code)
		}
		if r := e.do("POST", "/api/auth/activity", manager, false, nil); r.Code != 204 {
			t.Fatalf("heartbeat: %d", r.Code)
		}
	})

	t.Run("logout saat terkunci mencabut sesi", func(t *testing.T) {
		auth := e.bearer("cashier")
		e.idle(auth, "1 hour")
		if r := e.do("POST", "/api/auth/logout", auth, false, nil); r.Code != 204 {
			t.Fatalf("code %d", r.Code)
		}
		if r := e.do("GET", "/api/auth/me", auth, false, nil); r.Code != 401 {
			t.Fatalf("code %d", r.Code)
		}
	})

	t.Run("batas diatur owner, berlaku untuk semua gerai/pengguna", func(t *testing.T) {
		put := func(auth string, v any) resp {
			return e.do("PUT", "/api/settings", auth, false, map[string]any{"settings": map[string]any{"security.idle_lock_minutes": v}})
		}
		if r := put(manager, 5); r.Code != 403 {
			t.Fatalf("manager tulis: %d", r.Code)
		}
		for _, bad := range []any{0, 481, 2.5, "10", nil} {
			if r := put(owner, bad); r.Code != 422 {
				t.Fatalf("nilai %v: %d", bad, r.Code)
			}
		}
		if r := put(owner, 5); r.Code != 200 {
			t.Fatalf("owner tulis: %d %v", r.Code, r.Body)
		}
		if r := e.do("GET", "/api/auth/me", manager, false, nil); r.Body["idle_lock_minutes"] != float64(5) {
			t.Fatalf("%v", r.Body)
		}
		e.idle(owner, "6 minutes") // sebelumnya aman di 15 menit
		if r := e.do("GET", "/api/settings", owner, false, nil); r.Code != 423 {
			t.Fatalf("batas baru harus berlaku: %d", r.Code)
		}
	})

	t.Run("POS: cookie pos juga terkunci", func(t *testing.T) {
		r := e.login("cashier", "pos")
		cookie := "cookie:" + r.Cookies[0].Name + "=" + r.Cookies[0].Value
		var id string
		_ = e.pool.QueryRow(context.Background(), "SELECT max(id)::text FROM personal_access_tokens").Scan(&id)
		e.idle("Bearer "+id+"|x", "1 hour")
		if r := e.do("GET", "/api/pos/context", cookie, false, nil); r.Code != 423 {
			t.Fatalf("code %d", r.Code)
		}
	})
}
