package assets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	b := append([]byte("\x89PNG\r\n\x1a\n"), []byte("hello")...)
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

func TestPutRefreshesMtimeSoSweepSpares(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b := append([]byte("\x89PNG\r\n\x1a\n"), []byte("re-referenced")...)
	sha, err := s.Put(b)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	p := filepath.Join(dir, sha)
	stale := time.Now().Add(-time.Hour)
	if err := os.Chtimes(p, stale, stale); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if _, err := s.Put(b); err != nil {
		t.Fatalf("Put again: %v", err)
	}
	deleted, err := s.Sweep(map[string]bool{}, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want none: a re-Put asset must sit inside the grace window", deleted)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("asset gone after sweep: %v", err)
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

func TestPutRejectsNonImage(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		name string
		b    []byte
	}{
		{"html document", []byte("<!doctype html><script>alert(1)</script>")},
		{"plain text", []byte("hello")},
		{"svg", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Put(tt.b); !errors.Is(err, ErrNotImage) {
				t.Fatalf("Put err = %v, want ErrNotImage", err)
			}
		})
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

func TestSweep(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	young := time.Now()
	keptRef := strings.Repeat("a", 64)
	oldUnref := strings.Repeat("b", 64)
	youngUnref := strings.Repeat("c", 64)
	const stray = "not-a-content-address"
	write := func(name string, mtime time.Time) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}
	write(keptRef, old)
	write(oldUnref, old)
	write(youngUnref, young)
	write(stray, old)

	deleted, err := s.Sweep(map[string]bool{keptRef: true}, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != oldUnref {
		t.Fatalf("deleted = %v, want [%s]", deleted, oldUnref)
	}

	cases := []struct {
		name       string
		wantExists bool
	}{
		{keptRef, true},    // in the keep set, so retained regardless of mtime
		{oldUnref, false},  // unreferenced and older than grace
		{youngUnref, true}, // unreferenced but within the grace window
		{stray, true},      // not a content address, so never a sweep target
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, statErr := os.Stat(filepath.Join(dir, tc.name))
			if exists := statErr == nil; exists != tc.wantExists {
				t.Fatalf("%s exists=%v, want %v", tc.name, exists, tc.wantExists)
			}
		})
	}
}
