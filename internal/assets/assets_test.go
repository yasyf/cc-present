package assets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPutGetRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// A PNG magic header so DetectContentType returns image/png, not octet-stream.
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	sha, err := s.Put(png)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if sha != SHA(png) {
		t.Fatalf("Put sha = %q, want %q", sha, SHA(png))
	}
	got, ct, err := s.Get(sha)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("Get bytes mismatch: got %d bytes", len(got))
	}
	if ct != "image/png" {
		t.Fatalf("content type = %q, want image/png", ct)
	}
}

func TestPutIdempotent(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b := []byte("hello")
	first, err := s.Put(b)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	second, err := s.Put(b)
	if err != nil {
		t.Fatalf("Put again: %v", err)
	}
	if first != second {
		t.Fatalf("idempotent Put sha mismatch: %q vs %q", first, second)
	}
}

func TestPutCap(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.Put(bytes.Repeat([]byte{1}, MaxBytes+1)); err == nil {
		t.Fatal("Put over cap = nil error, want failure")
	} else if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Put over cap error = %q, want 'exceeds'", err.Error())
	}
}

func TestGetErrors(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		name string
		sha  string
	}{
		{"malformed sha", "not-a-sha"},
		{"uppercase hex rejected", strings.Repeat("A", 64)},
		{"path traversal", "../../etc/passwd"},
		{"well-formed but absent", strings.Repeat("a", 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := s.Get(tt.sha); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get(%q) err = %v, want ErrNotFound", tt.sha, err)
			}
		})
	}
	// A traversal target must never be read even if it exists on disk.
	outside := filepath.Join(dir, "secret")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get("../secret"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get traversal err = %v, want ErrNotFound", err)
	}
}
