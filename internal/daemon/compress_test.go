package daemon

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticAssetsGzipScriptsAndCacheHashedFilesAsImmutable(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":        {Data: []byte("shell")},
		"assets/app.js":     {Data: []byte("raw")},
		"assets/font.woff2": {Data: []byte("font")},
	}
	h := staticAssets(dist, http.FileServerFS(dist))
	immutable := "public, max-age=31536000, immutable"
	tests := []struct {
		path, accept, body, coding, cache string
	}{
		{"/assets/app.js", "gzip, deflate, br", "raw", "gzip", immutable},
		{"/assets/app.js", "gzip;q=0", "raw", "", immutable},
		{"/assets/app.js", "", "raw", "", immutable},
		{"/assets/font.woff2", "gzip", "font", "", immutable},
		{"/assets/missing.js", "gzip", "404 page not found\n", "", ""},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.path, nil)
		r.Header.Set("Accept-Encoding", tt.accept)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		body := w.Body.Bytes()
		if w.Header().Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			if body, err = io.ReadAll(zr); err != nil {
				t.Fatal(err)
			}
		}
		if string(body) != tt.body || w.Header().Get("Content-Encoding") != tt.coding || w.Header().Get("Cache-Control") != tt.cache {
			t.Errorf("%s [%s] = %q encoding %q cache %q, want %q encoding %q cache %q", tt.path, tt.accept,
				body, w.Header().Get("Content-Encoding"), w.Header().Get("Cache-Control"), tt.body, tt.coding, tt.cache)
		}
	}
}

func TestStateRevalidatesToNotModifiedUntilTheBoardChanges(t *testing.T) {
	h := newRestHarness(t)
	r := httptest.NewRequest(http.MethodGet, "/api/state?session=board--abcd0000", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.mux().ServeHTTP(w, r)
	etag := w.Header().Get("ETag")
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" || etag == "" {
		t.Fatalf("load = %d encoding %q etag %q, want 200 gzip with an etag", w.Code, w.Header().Get("Content-Encoding"), etag)
	}
	revalidate := func() int {
		r := httptest.NewRequest(http.MethodGet, "/api/state?session=board--abcd0000", nil)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		h.mux().ServeHTTP(w, r)
		return w.Code
	}
	if code := revalidate(); code != http.StatusNotModified {
		t.Fatalf("revalidate an unchanged board = %d, want 304", code)
	}
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1"]}}`); w.Code != http.StatusOK {
		t.Fatalf("tap = %d %s", w.Code, w.Body)
	}
	if code := revalidate(); code != http.StatusOK {
		t.Fatalf("revalidate after a tap = %d, want 200", code)
	}
}
