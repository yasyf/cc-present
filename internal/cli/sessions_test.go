package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestSessionRows(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		sessions []sessionInfo
		want     [][]string
	}{
		{
			name: "empty listing",
			port: 8080,
			want: [][]string{},
		},
		{
			name: "bound and detached sessions",
			port: 8080,
			sessions: []sessionInfo{
				{Subject: "sub1", Slug: "board--a", SessionID: "s-1", Status: "open", EventCount: 3},
				{Subject: "sub2", Slug: "board--b", SessionID: "", Status: "closed", EventCount: 7},
			},
			want: [][]string{
				{"sub1", "board--a", "s-1", "open", "3", "http://127.0.0.1:8080/p/board--a"},
				{"sub2", "board--b", "-", "closed", "7", "http://127.0.0.1:8080/p/board--b"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sessionRows(tt.port, tt.sessions)
			if len(got) != len(tt.want) {
				t.Fatalf("sessionRows() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if !slices.Equal(got[i], tt.want[i]) {
					t.Fatalf("sessionRows()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRenderSessionsEmpty(t *testing.T) {
	var buf bytes.Buffer
	renderSessions(&buf, "v9.9.9", 8080, nil)
	want := "cc-present daemon v9.9.9 · port 8080\n\nno artifacts\n"
	if got := buf.String(); got != want {
		t.Fatalf("renderSessions() = %q, want %q", got, want)
	}
}

func TestRenderSessionsPopulated(t *testing.T) {
	var buf bytes.Buffer
	renderSessions(&buf, "v9.9.9", 8080, []sessionInfo{
		{Subject: "sub1", Slug: "board--a", SessionID: "s-1", Status: "open", EventCount: 3},
	})
	got := buf.String()
	if !strings.HasPrefix(got, "cc-present daemon v9.9.9 · port 8080\n\n") {
		t.Fatalf("missing header, got %q", got)
	}
	for _, want := range []string{"SUBJECT", "URL", "sub1", "http://127.0.0.1:8080/p/board--a"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q, got %q", want, got)
		}
	}
}
