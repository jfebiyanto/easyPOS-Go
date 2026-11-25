package httpapi

import (
	"database/sql"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// WebFS adalah isi web/dist (boleh nil di tes API).
type WebFS = fs.FS

func nullTime(t time.Time) sql.NullTime { return sql.NullTime{Time: t.UTC(), Valid: true} }

// mountWeb menyajikan SPA: /backoffice/* dan /pos/* jatuh ke index.html masing-masing.
func mountWeb(r chi.Router, web WebFS) {
	if web == nil {
		return
	}
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/backoffice/", http.StatusFound)
	})
	files := http.FileServer(http.FS(web))
	spa := func(app string) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			name := strings.TrimPrefix(path.Clean(req.URL.Path), "/")
			if st, err := fs.Stat(web, name); err == nil && !st.IsDir() {
				files.ServeHTTP(w, req)
				return
			}
			http.ServeFileFS(w, req, web, app+"/index.html")
		}
	}
	r.Get("/backoffice", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/backoffice/", http.StatusMovedPermanently)
	})
	r.Get("/pos", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/pos/", http.StatusMovedPermanently)
	})
	r.Get("/backoffice/*", spa("backoffice"))
	r.Get("/pos/*", spa("pos"))
	r.Get("/assets/*", files.ServeHTTP)
}
