package daemon

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
)

type gzippedAsset struct {
	once sync.Once
	body []byte
	err  error
}

func staticAssets(dist fs.FS, next http.Handler) http.Handler {
	var gzipped sync.Map
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(dist, name); err != nil || !strings.HasPrefix(name, "assets/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Vary", "Accept-Encoding")
		ext := path.Ext(name)
		if (ext != ".js" && ext != ".css" && ext != ".svg") || !accepts(r, "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		cached, _ := gzipped.LoadOrStore(name, &gzippedAsset{})
		asset := cached.(*gzippedAsset)
		asset.once.Do(func() {
			raw, err := fs.ReadFile(dist, name)
			if err != nil {
				asset.err = err
				return
			}
			asset.body, asset.err = gzipBytes(raw, gzip.BestCompression)
		})
		if asset.err != nil {
			http.Error(w, asset.err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(ext))
		w.Header().Set("Content-Encoding", "gzip")
		//nolint:gosec // G705: body is the embedded web build, not request-derived content.
		_, _ = w.Write(asset.body)
	})
}

func accepts(r *http.Request, coding string) bool {
	for part := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(part, ";")
		if strings.TrimSpace(name) == coding {
			return strings.ReplaceAll(params, " ", "") != "q=0"
		}
	}
	return false
}

func gzipBytes(b []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
