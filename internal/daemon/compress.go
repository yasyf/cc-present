package daemon

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

var precompressed = []struct{ coding, ext string }{{"br", ".br"}, {"gzip", ".gz"}}

func staticAssets(dist fs.FS, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(dist, name); err != nil || !strings.HasPrefix(name, "assets/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Vary", "Accept-Encoding")
		for _, p := range precompressed {
			if !accepts(r, p.coding) {
				continue
			}
			body, err := fs.ReadFile(dist, name+p.ext)
			if err != nil {
				continue
			}
			w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(name)))
			w.Header().Set("Content-Encoding", p.coding)
			//nolint:gosec // G705: body is the embedded web build, not request-derived content.
			_, _ = w.Write(body)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func accepts(r *http.Request, coding string) bool {
	for part := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		name, _, _ := strings.Cut(part, ";")
		if strings.TrimSpace(name) == coding {
			return true
		}
	}
	return false
}
