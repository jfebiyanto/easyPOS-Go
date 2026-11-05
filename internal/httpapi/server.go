// Package httpapi wires the HTTP router, sessions, auth and role middleware.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/catalog"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
	"github.com/jfebiyanto/easyPOS-Go/internal/reporting"
	"github.com/jfebiyanto/easyPOS-Go/internal/returns"
	"github.com/jfebiyanto/easyPOS-Go/internal/sales"
)

type Server struct {
	pool     *pgxpool.Pool
	identity *identity.Service
	catalog  *catalog.Service
	sales    *sales.Service
	reports  *reporting.Service
	loyalty  *loyalty.Service
	returns  *returns.Service
	sessions *scs.SessionManager
	web      fs.FS
}

func New(pool *pgxpool.Pool, secureCookies bool, web fs.FS) *Server {
	sm := scs.New()
	sm.Store = pgxstore.New(pool)
	sm.Lifetime = 12 * time.Hour
	sm.Cookie.Name = "easypos_session"
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = secureCookies
	sm.Cookie.SameSite = http.SameSiteLaxMode
	return &Server{pool: pool, identity: identity.NewService(pool), catalog: catalog.NewService(pool),
		sales: sales.NewService(pool), reports: reporting.NewService(pool), loyalty: loyalty.NewService(pool),
		returns: returns.NewService(pool), sessions: sm, web: web}
}

func (s *Server) Identity() *identity.Service { return s.identity }

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer)
	r.Use(s.sessions.LoadAndSave)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_down"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		// FR-A1-05: 5 login attempts per minute per IP.
		r.With(httprate.LimitByIP(5, time.Minute)).Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/auth/me", s.me)
			// Admin area: cashiers are rejected server-side (FR-A1-03).
			r.Route("/admin", func(r chi.Router) {
				r.Use(RequireRole(identity.RoleOwner, identity.RoleStoreManager))
				s.mountAdmin(r)
			})
			// POS: Owner has no store, so staff roles only; stores are enforced by requireStore.
			r.Route("/pos", func(r chi.Router) {
				r.Use(RequireRole(identity.RoleCashier, identity.RoleStoreManager))
				s.mountPOS(r)
			})
		})
	})

	r.Handle("/*", s.spa())
	return r
}

type ctxKey struct{}

func CurrentUser(ctx context.Context) *identity.User {
	u, _ := ctx.Value(ctxKey{}).(*identity.User)
	return u
}

// requireAuth reloads the user on every request so deactivation takes effect immediately.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := s.sessions.GetString(r.Context(), "user_id")
		if id == "" {
			writeErr(w, http.StatusUnauthorized, "belum login")
			return
		}
		u, err := s.identity.Get(r.Context(), id)
		if err != nil || !u.IsActive {
			_ = s.sessions.Destroy(r.Context())
			writeErr(w, http.StatusUnauthorized, "sesi tidak valid")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := CurrentUser(r.Context())
			for _, role := range roles {
				if u != nil && u.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeErr(w, http.StatusForbidden, "tidak punya akses")
		})
	}
}

type userDTO struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	User    string  `json:"username"`
	Role    string  `json:"role"`
	StoreID *string `json:"store_id"`
}

func toDTO(u *identity.User) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, User: u.Username, Role: u.Role, StoreID: u.StoreID}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "permintaan tidak valid")
		return
	}
	u, err := s.identity.Login(r.Context(), in.Username, in.Password)
	if errors.Is(err, identity.ErrInvalidCredentials) {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "terjadi kesalahan")
		return
	}
	if err := s.sessions.RenewToken(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, "terjadi kesalahan")
		return
	}
	s.sessions.Put(r.Context(), "user_id", u.ID)
	s.auditLogin(r.Context(), u)
	writeJSON(w, http.StatusOK, toDTO(u))
}

func (s *Server) auditLogin(ctx context.Context, u *identity.User) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if audit.Record(ctx, tx, audit.Entry{CompanyID: u.CompanyID, Actor: audit.Actor{ID: u.ID, Name: u.Name},
		Action: "auth.login", SubjectType: "user", SubjectID: u.ID}) == nil {
		_ = tx.Commit(ctx)
	}
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	_ = s.sessions.Destroy(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toDTO(CurrentUser(r.Context())))
}

// spa serves the embedded frontend, falling back to index.html for client routes.
func (s *Server) spa() http.Handler {
	files := http.FileServerFS(s.web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if len(p) > 1 {
			if f, err := s.web.Open(p[1:]); err == nil {
				f.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
