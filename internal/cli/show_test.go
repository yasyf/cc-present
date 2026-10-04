package cli

import (
	"encoding/json"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

func TestShowDoc(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		title  string
		height int
		want   string
	}{
		{
			name: "html",
			path: "/tmp/chart.html",
			want: `{"blocks":[{"id":"artifact","kind":"html","source":"body","type":"display.artifact"}],"title":"chart.html","version":1}`,
		},
		{
			name:   "svg with title and height",
			path:   "diagram.SVG",
			title:  "Flow",
			height: 320,
			want:   `{"blocks":[{"height":320,"id":"artifact","kind":"svg","source":"body","type":"display.artifact"}],"title":"Flow","version":1}`,
		},
		{
			name: "markdown",
			path: "notes.md",
			want: `{"blocks":[{"id":"page","md":"body","type":"display.page"}],"title":"notes.md","version":1}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := showDoc(tt.path, []byte("body"), tt.title, tt.height)
			if err != nil {
				t.Fatalf("showDoc: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("showDoc =\n%s\nwant\n%s", got, tt.want)
			}
			d := &doc.Doc{}
			if err := json.Unmarshal(got, d); err != nil {
				t.Fatalf("decode doc: %v", err)
			}
		})
	}
}

func TestShowDocRejects(t *testing.T) {
	tests := []struct {
		name, path string
		height     int
		want       string
	}{
		{"unknown extension", "app.js", 0, `show: unsupported file type ".js" (want .html, .htm, .svg, .md, or .markdown)`},
		{"no extension", "README", 0, `show: unsupported file type "" (want .html, .htm, .svg, .md, or .markdown)`},
		{"height on markdown", "notes.markdown", 200, "show: --height applies to HTML and SVG files, not .markdown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := showDoc(tt.path, []byte("x"), "", tt.height)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
