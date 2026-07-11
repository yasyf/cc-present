package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/packs"
)

const packManifest = `host_api = 1
name = "example"
version = "0.2.0"
description = "Example blocks."
entry = "dist/pack.js"
styles = "dist/pack.css"
reference = "reference/blocks.md"

[blocks.callout]
description = "Toned admonition."
schema = "schema/callout.json"
examples = ["examples/callout.json"]

[blocks.rating]
description = "1-5 rating."
schema = "schema/rating.json"
interaction = "schema/rating.interaction.json"
examples = ["examples/rating.json"]
`

const ratingInteractionSchema = `{"type":"object","required":["value"],"properties":{"value":{"type":"integer","minimum":1,"maximum":5}},"additionalProperties":false}`

// packSeed carries every shape the pack.interaction edge branches on: an
// interactive pack block, a non-interactive one, a builtin block, and a pack
// block whose pack is no longer installed.
const packSeed = `{"version":1,"title":"T","blocks":[
  {"id":"rate1","type":"example.rating","value":3},
  {"id":"cal1","type":"example.callout","tone":"warn"},
  {"id":"a1","type":"approval"},
  {"id":"gone1","type":"ghost.thing"}
]}`

func writePackTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"cc-present.toml":                packManifest,
		"dist/pack.js":                   "export default { hostApi: 1, blocks: {} }\n",
		"dist/pack.css":                  ".callout{}\n",
		"reference/blocks.md":            "# blocks\n",
		"schema/callout.json":            `{"type":"object"}`,
		"schema/rating.json":             `{"type":"object"}`,
		"schema/rating.interaction.json": ratingInteractionSchema,
		"examples/callout.json":          `{"id":"c","type":"example.callout"}`,
		"examples/rating.json":           `{"id":"r","type":"example.rating","value":3}`,
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// packLoader builds a loader over a single dev pack dir with an isolated Claude
// config dir, so a test never scans the developer's real installed plugins.
func packLoader(t *testing.T, dir string) *packs.Loader {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	return packs.NewLoader([]string{dir}, nil)
}

func TestPacksAPI(t *testing.T) {
	rs := &restServer{packs: packLoader(t, writePackTree(t))}
	req := httptest.NewRequest(http.MethodGet, "/api/packs", nil)
	w := httptest.NewRecorder()
	rs.handlePacks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		HostAPI int `json:"hostApi"`
		Packs   []struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Bundle      string `json:"bundle"`
			Styles      string `json:"styles"`
			Blocks      []struct {
				Type        string          `json:"type"`
				Interactive bool            `json:"interactive"`
				Schema      json.RawMessage `json:"schema"`
				Interaction json.RawMessage `json:"interaction"`
			} `json:"blocks"`
		} `json:"packs"`
		Dropped []struct {
			Dir, Reason string
		} `json:"dropped"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.HostAPI != packs.HostAPIVersion {
		t.Fatalf("hostApi = %d, want %d", resp.HostAPI, packs.HostAPIVersion)
	}
	if len(resp.Packs) != 1 {
		t.Fatalf("packs = %d, want 1", len(resp.Packs))
	}
	p := resp.Packs[0]
	if p.Name != "example" || p.Version != "0.2.0" {
		t.Fatalf("name/version = %q/%q", p.Name, p.Version)
	}
	if p.Bundle != "/packs/example/dist/pack.js?v=0.2.0" {
		t.Fatalf("bundle = %q", p.Bundle)
	}
	if p.Styles != "/packs/example/dist/pack.css?v=0.2.0" {
		t.Fatalf("styles = %q", p.Styles)
	}
	if len(p.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(p.Blocks))
	}
	// name-sorted: callout (non-interactive), rating (interactive)
	if p.Blocks[0].Type != "example.callout" || p.Blocks[0].Interactive {
		t.Fatalf("block[0] = %+v, want example.callout non-interactive", p.Blocks[0])
	}
	if p.Blocks[1].Type != "example.rating" || !p.Blocks[1].Interactive {
		t.Fatalf("block[1] = %+v, want example.rating interactive", p.Blocks[1])
	}
	if len(p.Blocks[1].Interaction) == 0 {
		t.Fatal("rating interaction schema not inlined")
	}
	if string(p.Blocks[0].Schema) != `{"type":"object"}` {
		t.Fatalf("callout schema = %s, want raw inline", p.Blocks[0].Schema)
	}
	if len(resp.Dropped) != 0 {
		t.Fatalf("dropped = %v, want none", resp.Dropped)
	}
}

// TestPacksAPIDroppedDirBaseName asserts the /api/packs response reports a
// dropped candidate by its directory base name, never the absolute path, so it
// can't disclose the host's home directory.
func TestPacksAPIDroppedDirBaseName(t *testing.T) {
	dir := writePackTree(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	rs := &restServer{packs: packs.NewLoader([]string{dir}, []string{"example"})}
	req := httptest.NewRequest(http.MethodGet, "/api/packs", nil)
	w := httptest.NewRecorder()
	rs.handlePacks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Dropped []struct{ Dir, Reason string } `json:"dropped"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Dropped) != 1 {
		t.Fatalf("dropped = %d, want 1", len(resp.Dropped))
	}
	if got := resp.Dropped[0].Dir; got != filepath.Base(dir) {
		t.Fatalf("dropped dir = %q, want base name %q", got, filepath.Base(dir))
	}
	if strings.ContainsRune(resp.Dropped[0].Dir, os.PathSeparator) {
		t.Fatalf("dropped dir %q leaks a path separator", resp.Dropped[0].Dir)
	}
}

func TestPackFileServing(t *testing.T) {
	dir := writePackTree(t)
	rs := &restServer{packs: packLoader(t, dir)}

	t.Run("happy path serves the bundle", func(t *testing.T) {
		w := servePackFile(rs, "example", "dist/pack.js")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "export default") {
			t.Fatalf("body = %q", w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
			t.Fatalf("content-type = %q, want text/javascript", ct)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing nosniff")
		}
		if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Fatalf("cache-control = %q, want immutable", cc)
		}
	})

	t.Run("css served with its type", func(t *testing.T) {
		w := servePackFile(rs, "example", "dist/pack.css")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
			t.Fatalf("content-type = %q, want text/css", ct)
		}
	})

	refused := []struct {
		name string
		pack string
		file string
	}{
		{"unknown pack", "nope", "dist/pack.js"},
		{"non-dist manifest", "example", "cc-present.toml"},
		{"non-dist schema", "example", "schema/callout.json"},
		{"parent traversal", "example", "dist/../cc-present.toml"},
		{"deep traversal", "example", "dist/../../etc/passwd"},
		{"bare dist dir", "example", "dist"},
		{"missing file", "example", "dist/missing.js"},
		{"absolute", "example", "/etc/passwd"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			w := servePackFile(rs, tt.pack, tt.file)
			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestPackFileSymlinkEscape(t *testing.T) {
	dir := writePackTree(t)
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "dist", "escape.js")); err != nil {
		t.Fatal(err)
	}
	rs := &restServer{packs: packLoader(t, dir)}
	w := servePackFile(rs, "example", "dist/escape.js")
	if w.Code != http.StatusNotFound {
		t.Fatalf("symlink escape status = %d, want 404", w.Code)
	}
	if strings.Contains(w.Body.String(), "TOPSECRET") {
		t.Fatal("symlink escape leaked the secret")
	}
}

// TestPackFileEncodedTraversal drives the real ServeMux so an encoded %2e%2e can
// never resolve a file outside dist/ end-to-end.
func TestPackFileEncodedTraversal(t *testing.T) {
	dir := writePackTree(t)
	rs := &restServer{packs: packLoader(t, dir)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /packs/{pack}/{file...}", rs.handlePackFile)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/packs/example/dist/pack.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("happy path status = %d, want 200", resp.StatusCode)
	}

	resp2, err := http.Get(srv.URL + "/packs/example/dist/%2e%2e/cc-present.toml")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Fatalf("encoded traversal status = %d, want non-200", resp2.StatusCode)
	}
	if strings.Contains(string(body), "host_api") {
		t.Fatal("encoded traversal leaked the manifest")
	}
}

func servePackFile(rs *restServer, pack, file string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/packs/x/y", nil)
	req.SetPathValue("pack", pack)
	req.SetPathValue("file", file)
	w := httptest.NewRecorder()
	rs.handlePackFile(w, req)
	return w
}

func TestPackInteractionValidation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"happy path", `{"subject":"board--abcd0000","nonce":"p1","interaction":{"type":"pack.interaction","blockId":"rate1","payload":{"value":4}}}`, 200, ""},
		{"unknown block", `{"subject":"board--abcd0000","nonce":"p2","interaction":{"type":"pack.interaction","blockId":"zzz","payload":{"value":4}}}`, 400, "unknown block"},
		{"builtin block", `{"subject":"board--abcd0000","nonce":"p3","interaction":{"type":"pack.interaction","blockId":"a1","payload":{"value":4}}}`, 400, "not a pack block"},
		{"non-interactive type", `{"subject":"board--abcd0000","nonce":"p4","interaction":{"type":"pack.interaction","blockId":"cal1","payload":{"value":4}}}`, 400, "not interactive"},
		{"uninstalled type", `{"subject":"board--abcd0000","nonce":"p5","interaction":{"type":"pack.interaction","blockId":"gone1","payload":{"value":4}}}`, 400, "not installed"},
		{"schema-invalid payload", `{"subject":"board--abcd0000","nonce":"p6","interaction":{"type":"pack.interaction","blockId":"rate1","payload":{"value":9}}}`, 400, "pack interaction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, packSeed, packLoader(t, writePackTree(t)))
			w := h.post(t, tt.body)
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantErr != "" && !strings.Contains(w.Body.String(), tt.wantErr) {
				t.Fatalf("body = %q, want %q", w.Body.String(), tt.wantErr)
			}
		})
	}
}

func TestPackInteractionClosedRound(t *testing.T) {
	h := newRestHarnessWith(t, packSeed, packLoader(t, writePackTree(t)))
	// The seed blocks all carry round 1, so a submit closes round 1 and advances.
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"sub","interaction":{"type":"submit","revision":1}}`); w.Code != http.StatusOK {
		t.Fatalf("submit status = %d (%s)", w.Code, w.Body.String())
	}
	w := h.post(t, `{"subject":"board--abcd0000","nonce":"pc","interaction":{"type":"pack.interaction","blockId":"rate1","payload":{"value":2}}}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "closed round") {
		t.Fatalf("closed-round status = %d, body %q, want 400 'closed round'", w.Code, w.Body.String())
	}
}
