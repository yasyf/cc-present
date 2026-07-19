package web

import (
	"testing"
	"testing/fstest"
)

func TestValidateDist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dist    fstest.MapFS
		wantErr string
	}{
		{
			name: "consistent build",
			dist: fstest.MapFS{
				"index.html":     {Data: []byte(`<link rel="stylesheet" href="/assets/app.css"><script type="module" src="/assets/app.js"></script>`)},
				"assets/app.css": {Data: []byte("body {}")},
				"assets/app.js":  {Data: []byte("export {}")},
			},
		},
		{
			name:    "missing index",
			dist:    fstest.MapFS{},
			wantErr: "no web build embedded — run 'task build': open index.html: file does not exist",
		},
		{
			name: "shell without asset references",
			dist: fstest.MapFS{
				"index.html": {Data: []byte(`<html><body>cc-present</body></html>`)},
			},
			wantErr: "embedded web build is not a Vite build output",
		},
		{
			name: "missing referenced asset",
			dist: fstest.MapFS{
				"index.html": {Data: []byte(`<script type="module" src="/assets/missing.js"></script>`)},
			},
			wantErr: `embedded web build is internally inconsistent: missing "assets/missing.js": open assets/missing.js: file does not exist`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateDist(tt.dist)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateDist() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateDist() error = nil, want %q", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("validateDist() error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}
