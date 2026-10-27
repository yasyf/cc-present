package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticAssetsServeThePrecompressedCopyTheClientAccepts(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":        {Data: []byte("shell")},
		"assets/app.js":     {Data: []byte("raw")},
		"assets/app.js.br":  {Data: []byte("brotli")},
		"assets/app.js.gz":  {Data: []byte("gzip")},
		"assets/font.woff2": {Data: []byte("font")},
	}
	h := staticAssets(dist, http.FileServerFS(dist))
	tests := []struct {
		path, accept, body, coding, cache string
	}{
		{"/assets/app.js", "gzip, deflate, br", "brotli", "br", "public, max-age=31536000, immutable"},
		{"/assets/app.js", "gzip", "gzip", "gzip", "public, max-age=31536000, immutable"},
		{"/assets/app.js", "", "raw", "", "public, max-age=31536000, immutable"},
		{"/assets/font.woff2", "br", "font", "", "public, max-age=31536000, immutable"},
		{"/assets/missing.js", "br", "404 page not found\n", "", ""},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.path, nil)
		r.Header.Set("Accept-Encoding", tt.accept)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Body.String() != tt.body || w.Header().Get("Content-Encoding") != tt.coding || w.Header().Get("Cache-Control") != tt.cache {
			t.Errorf("%s [%s] = %q encoding %q cache %q, want %q encoding %q cache %q", tt.path, tt.accept,
				w.Body, w.Header().Get("Content-Encoding"), w.Header().Get("Cache-Control"), tt.body, tt.coding, tt.cache)
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
