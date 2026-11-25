// Package httpapi: router chi, middleware, dan handler.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
)

const (
	CookieBO     = "easypos_bo_token"
	CookiePOS    = "easypos_pos_token"
	CSRFHeader   = "X-EasyPOS-Client"
	loginMessage = "Terlalu banyak percobaan login. Coba lagi sebentar lagi."
)

type Deps struct {
	Pool         *pgxpool.Pool
	Identity     *identity.Service
	Log          *slog.Logger
	Loc          *time.Location // zona waktu aplikasi (penomoran)
	CookieSecure bool
	LoginLimit   int // percobaan/menit per IP+username; default 5
	Web          WebFS
}

type server struct{ Deps }

type ctxKey struct{}

func sessionFrom(ctx context.Context) *identity.Session {
	s, _ := ctx.Value(ctxKey{}).(*identity.Session)
	return s
}

func New(d Deps) http.Handler {
	if d.LoginLimit == 0 {
		d.LoginLimit = 5
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	s := &server{d}
	r := chi.NewRouter()
	r.Use(httpx.Logging(d.Log))

	r.Route("/api", func(r chi.Router) {
		r.With(httprate.Limit(d.LoginLimit, time.Minute,
			httprate.WithKeyFuncs(loginKey), httprate.WithLimitHandler(func(w http.ResponseWriter, _ *http.Request) {
				httpx.Error(w, http.StatusTooManyRequests, loginMessage)
			}))).Post("/auth/login", s.login)

		authRoutes := func(r chi.Router) {
			r.Get("/auth/me", s.me)
			r.Post("/auth/logout", s.logout)
			r.Get("/stores/accessible", s.accessibleStores)
			r.Post("/session/store", s.switchStore)
		}
		// Klien POS memakai prefix /api/pos (cookie POS); rute yang butuh gerai memakai requireStore.
		r.Route("/pos", func(r chi.Router) {
			r.Use(s.authenticate)
			authRoutes(r)
			r.With(requireStore).Get("/context", s.posContext)
		})

		r.Group(func(r chi.Router) {
			r.Use(s.authenticate)
			authRoutes(r)

			r.With(s.route("admin.settings-view.index")).Get("/settings", s.getSettings)
			r.With(s.route("admin.settings.update")).Put("/settings", s.putSettings)
			r.With(s.route("admin.numberings.index")).Get("/numberings", s.getNumberings)
			r.With(s.route("admin.numberings.update")).Put("/numberings", s.putNumberings)
			r.With(s.route("admin.numberings.next")).Post("/numberings/{type}/next", s.nextNumber)
		})
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) { httpx.Error(w, http.StatusNotFound, "Tidak ditemukan.") })
	})
	mountWeb(r, d.Web)
	return r
}

// loginKey: kunci rate limit = IP + username (body dibaca ulang agar handler tetap bisa decode).
func loginKey(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	r.Body = io.NopCloser(bytes.NewReader(body))
	var in struct {
		Username string `json:"username"`
	}
	_ = json.Unmarshal(body, &in)
	return host + "|" + in.Username, nil
}

// CookieFor memilih nama cookie berdasarkan jenis klien.
func CookieFor(kind string) string {
	if kind == "pos" {
		return CookiePOS
	}
	return CookieBO
}

func bearer(r *http.Request) (token string, viaCookie bool) {
	if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
		return h[7:], false
	}
	name := CookieBO
	if strings.HasPrefix(r.URL.Path, "/api/pos/") {
		name = CookiePOS
	}
	if c, err := r.Cookie(name); err == nil {
		return c.Value, true
	}
	return "", false
}

// requireStore menolak (403) rute POS bila sesi tidak punya gerai aktif.
func requireStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sess := sessionFrom(r.Context()); sess == nil || sess.ActiveStore == nil {
			httpx.Error(w, http.StatusForbidden, "Akun belum punya gerai aktif.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate: token dari header Bearer atau cookie; cookie + non-GET wajib header CSRF.
func (s *server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, viaCookie := bearer(r)
		if raw == "" {
			httpx.Error(w, http.StatusUnauthorized, "Tidak terautentikasi.")
			return
		}
		sess, err := s.Identity.Authenticate(r.Context(), raw)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "Tidak terautentikasi.")
			return
		}
		if viaCookie && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions &&
			r.Header.Get(CSRFHeader) != "1" {
			httpx.Error(w, http.StatusForbidden, "Header "+CSRFHeader+" wajib untuk request ini.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, sess)))
	})
}

// route membatasi endpoint dengan matriks akses (route key gaya Laravel).
func (s *server) route(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := sessionFrom(r.Context())
			if sess == nil || !identity.Allows(sess.User.Role, key, r.Method) {
				httpx.Error(w, http.StatusForbidden, "Anda tidak punya akses ke halaman ini.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
