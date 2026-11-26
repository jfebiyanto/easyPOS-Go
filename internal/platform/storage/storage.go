// Package storage: file unggahan di disk (gambar produk, dll.) dan penyajiannya lewat /media.
package storage

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// served: hanya ekstensi gambar yang disajikan.
var served = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// Join menggabungkan root dengan path relatif berpemisah "/" dan menolak apa pun yang keluar dari root.
func Join(root, rel string) (string, bool) {
	if strings.ContainsAny(rel, "\\:\x00") {
		return "", false
	}
	clean := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if clean == "" {
		return "", false
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	full := filepath.Join(abs, filepath.FromSlash(clean))
	if !strings.HasPrefix(full, abs+string(filepath.Separator)) {
		return "", false
	}
	return full, true
}

// Remove menghapus file bila ada (best-effort; path tak aman diabaikan).
func Remove(root, rel string) {
	if full, ok := Join(root, rel); ok {
		_ = os.Remove(full)
	}
}

// Handler menyajikan file regular di bawah root (pasang dengan StripPrefix("/media/")). Tanpa directory listing.
func Handler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || !served[strings.ToLower(path.Ext(r.URL.Path))] {
			http.NotFound(w, r)
			return
		}
		full, ok := Join(root, r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // nama file unik per unggahan
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
	})
}
