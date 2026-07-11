package daemon

import (
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yasyf/cc-present/internal/packs"
)

// packsResponse is the GET /api/packs body: the installed packs with their raw
// schemas inline, the compat-gate host API version, and the dropped candidates.
type packsResponse struct {
	HostAPI int           `json:"hostApi"`
	Packs   []packInfo    `json:"packs"`
	Dropped []droppedInfo `json:"dropped"`
}

type packInfo struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	Description string      `json:"description"`
	Bundle      string      `json:"bundle"`
	Styles      string      `json:"styles,omitempty"`
	Blocks      []blockInfo `json:"blocks"`
}

type blockInfo struct {
	Type        string          `json:"type"`
	Interactive bool            `json:"interactive"`
	Schema      json.RawMessage `json:"schema"`
	Interaction json.RawMessage `json:"interaction,omitempty"`
}

// droppedInfo is a dropped candidate as reported to HTTP clients: Dir is the
// candidate directory's base name, never its absolute path, so the response can't
// disclose the host's home directory. The absolute path stays in the slog warning
// and in `cc-present pack list`.
type droppedInfo struct {
	Dir    string `json:"dir"`
	Reason string `json:"reason"`
}

// handlePacks reports the installed packs and dropped candidates so the SPA can
// dynamic-import each bundle and enumerate the pack block types.
func (rs *restServer) handlePacks(w http.ResponseWriter, _ *http.Request) {
	reg := rs.packs.Current()
	resp := packsResponse{HostAPI: packs.HostAPIVersion, Packs: []packInfo{}, Dropped: []droppedInfo{}}
	for _, p := range reg.Packs() {
		info := packInfo{
			Name:        p.Name,
			Version:     p.Version,
			Description: p.Description,
			Bundle:      packURL(p.Name, p.Entry, p.Version),
			Blocks:      []blockInfo{},
		}
		if p.Styles != "" {
			info.Styles = packURL(p.Name, p.Styles, p.Version)
		}
		for _, bt := range p.Blocks {
			info.Blocks = append(info.Blocks, blockInfo{
				Type:        bt.FullType(),
				Interactive: bt.Interactive(),
				Schema:      bt.SchemaBytes,
				Interaction: bt.InteractionBytes,
			})
		}
		resp.Packs = append(resp.Packs, info)
	}
	for _, d := range reg.Dropped {
		resp.Dropped = append(resp.Dropped, droppedInfo{Dir: filepath.Base(d.Dir), Reason: d.Reason})
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePackFile serves a pack's prebuilt bundle assets. It resolves the pack by
// name, then serves only files under the pack's dist/ subtree through os.OpenRoot
// so no symlink or .. component can escape the pack root. An unknown pack, a path
// outside dist/, or a missing file is a 404 — never an SPA fallthrough.
func (rs *restServer) handlePackFile(w http.ResponseWriter, r *http.Request) {
	pk, ok := packByName(rs.packs.Current(), r.PathValue("pack"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	file := r.PathValue("file")
	if !fs.ValidPath(file) || !strings.HasPrefix(file, "dist/") {
		http.NotFound(w, r)
		return
	}
	root, err := os.OpenRoot(pk.Dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(file)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if ct := packContentType(file); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	//nolint:gosec // G703: file is fs.ValidPath-checked, dist/-prefixed, and served through os.OpenRoot, which structurally refuses any .. or symlink escape.
	http.ServeFileFS(w, r, root.FS(), file)
}

func packByName(reg *packs.Registry, name string) (*packs.Pack, bool) {
	for _, p := range reg.Packs() {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}

// packURL builds the versioned bundle/styles URL for a pack-relative file; the
// ?v= query cache-busts on a version bump.
func packURL(name, rel, version string) string {
	return "/packs/" + name + "/" + rel + "?v=" + version
}

// packContentType pins the Content-Type for the bundle's own extensions so a .js
// bundle always loads as an ES module; other extensions fall back to the mime db.
func packContentType(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	default:
		return mime.TypeByExtension(path.Ext(file))
	}
}
