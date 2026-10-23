// Package assets is cc-present's content-addressed store for images and
// reference snippets. A pushed image whose src is a local file is uploaded here
// and referenced as asset:<sha256>; a resolved code, record, or commit reference
// stores its hover snippet here and links it by sha — so the event log and its
// SSE replay stay small while the browser fetches the bytes by hash.
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// MaxBytes caps a single stored asset.
const MaxBytes = 5 << 20 // 5 MiB

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ErrNotFound reports a missing or malformed asset.
var ErrNotFound = errors.New("asset not found")

// ErrUnsupported reports bytes that are neither an image nor a Snippet. The
// store holds only those two; anything else (notably HTML) would be served back
// same-origin from GET /assets/{sha}.
var ErrUnsupported = errors.New("asset is neither an image nor a snippet")

// SnippetType is the content type a stored Snippet is served under.
const SnippetType = "application/json"

// Snippet is a resolved reference's hover payload: a code excerpt (kind code),
// a cc-notes record (kind record), or a commit (kind commit).
type Snippet struct {
	Kind      string   `json:"kind"`
	Label     string   `json:"label"`
	Title     string   `json:"title,omitempty"`
	URL       string   `json:"url,omitempty"`
	Path      string   `json:"path,omitempty"`
	Sha       string   `json:"sha,omitempty"`
	Lang      string   `json:"lang,omitempty"`
	Highlight string   `json:"highlight,omitempty"`
	Chunks    []Chunk  `json:"chunks,omitempty"`
	Body      string   `json:"body,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Author    string   `json:"author,omitempty"`
	Date      string   `json:"date,omitempty"`
}

// Chunk is a contiguous run of source lines starting at line Start.
type Chunk struct {
	Start int    `json:"start"`
	Code  string `json:"code"`
}

var snippetKinds = map[string]bool{"code": true, "record": true, "commit": true}

func isSnippet(b []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var s Snippet
	if dec.Decode(&s) != nil || !errors.Is(dec.Decode(new(json.RawMessage)), io.EOF) {
		return false
	}
	return snippetKinds[s.Kind] && s.Label != ""
}

// Store is a content-addressed blob store under a directory: each asset is a
// file named by the lowercase hex sha256 of its bytes.
type Store struct{ dir string }

// New returns a Store rooted at dir without touching the filesystem.
func New(dir string) *Store { return &Store{dir: dir} }

// Prepare creates the store directory before the daemon begins serving.
func (s *Store) Prepare() error {
	dir := s.dir
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create asset dir: %w", err)
	}
	return nil
}

// SHA is the content address (lowercase hex sha256) of b.
func SHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Valid reports whether sha is a well-formed content address (64 lowercase hex).
func Valid(sha string) bool { return shaPattern.MatchString(sha) }

// Put stores b and returns its sha256, rejecting bytes that neither sniff as an
// image nor decode as a Snippet. Storing is idempotent: identical bytes map to the same file, so a
// repeated Put writes nothing — but it refreshes the file's mtime, so Sweep's
// grace window protects a just-re-referenced asset even when the bytes are old.
func (s *Store) Put(b []byte) (string, error) {
	if len(b) > MaxBytes {
		return "", fmt.Errorf("asset is %d bytes, exceeds %d", len(b), MaxBytes)
	}
	if ct := http.DetectContentType(b); !strings.HasPrefix(ct, "image/") && !isSnippet(b) {
		return "", fmt.Errorf("%w: detected %s", ErrUnsupported, ct)
	}
	sha := SHA(b)
	path := filepath.Join(s.dir, sha)
	if _, err := os.Stat(path); err == nil {
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			return "", fmt.Errorf("refresh asset %s mtime: %w", sha, err)
		}
		return sha, nil
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("write asset %s: %w", sha, err)
	}
	return sha, nil
}

// Get returns a stored asset's bytes and content type: the detected image type,
// or SnippetType for a snippet. A malformed sha
// is ErrNotFound before any filesystem lookup, so a path traversal can never
// reach the disk.
func (s *Store) Get(sha string) ([]byte, string, error) {
	if !shaPattern.MatchString(sha) {
		return nil, "", ErrNotFound
	}
	//nolint:gosec // G304: sha is validated to 64 lowercase hex above, so no user-controlled path element reaches the join.
	b, err := os.ReadFile(filepath.Join(s.dir, sha))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("read asset %s: %w", sha, err)
	}
	if ct := http.DetectContentType(b); strings.HasPrefix(ct, "image/") {
		return b, ct, nil
	}
	return b, SnippetType, nil
}

// Sweep deletes every stored asset absent from keep whose mtime predates
// now-grace, returning the shas it deleted. The grace window guards a fresh
// upload that lands between a caller computing keep and this pass: a just-written
// file is younger than grace, so it survives even while still unreferenced.
// Names that are not content addresses are left untouched, and the sweep fails
// fast on the first deletion error rather than swallowing it.
func (s *Store) Sweep(keep map[string]bool, grace time.Duration) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read asset dir: %w", err)
	}
	cutoff := time.Now().Add(-grace)
	var deleted []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !shaPattern.MatchString(name) || keep[name] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return deleted, fmt.Errorf("stat asset %s: %w", name, err)
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, name)); err != nil {
			return deleted, fmt.Errorf("delete asset %s: %w", name, err)
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}
