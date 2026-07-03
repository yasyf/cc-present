// Package assets is cc-present's content-addressed image store. A pushed image
// whose src is a local file is uploaded here, stored under the sha256 of its
// bytes, and referenced as asset:<sha256> — so the event log and its SSE replay
// stay small while the browser fetches the bytes by hash.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

// MaxBytes caps a single stored asset.
const MaxBytes = 5 << 20 // 5 MiB

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ErrNotFound reports a missing or malformed asset.
var ErrNotFound = errors.New("asset not found")

// Store is a content-addressed blob store under a directory: each asset is a
// file named by the lowercase hex sha256 of its bytes.
type Store struct{ dir string }

// New returns a Store rooted at dir, creating it (0700) if missing.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create asset dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// SHA is the content address (lowercase hex sha256) of b.
func SHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Valid reports whether sha is a well-formed content address (64 lowercase hex).
func Valid(sha string) bool { return shaPattern.MatchString(sha) }

// Put stores b and returns its sha256. Storing is idempotent: identical bytes
// map to the same file, so a repeated Put writes nothing.
func (s *Store) Put(b []byte) (string, error) {
	if len(b) > MaxBytes {
		return "", fmt.Errorf("asset is %d bytes, exceeds %d", len(b), MaxBytes)
	}
	sha := SHA(b)
	path := filepath.Join(s.dir, sha)
	if _, err := os.Stat(path); err == nil {
		return sha, nil
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("write asset %s: %w", sha, err)
	}
	return sha, nil
}

// Get returns a stored asset's bytes and detected content type. A malformed sha
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
	return b, http.DetectContentType(b), nil
}
