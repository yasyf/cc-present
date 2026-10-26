package cli

import (
	"strings"
	"testing"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
)

func TestVerdictMoves(t *testing.T) {
	tests := []struct {
		name    string
		from    []string
		to      []string
		want    []ccdaemon.VerdictMove
		wantErr string
	}{
		{
			name: "pairs by position and splits at the first slash",
			from: []string{"merge-triage/row-1", "old/a/b"},
			to:   []string{"merge-prs/42785", "merge-prs/42786"},
			want: []ccdaemon.VerdictMove{
				{From: ccdaemon.VerdictRef{Block: "merge-triage", Row: "row-1"}, To: ccdaemon.VerdictRef{Block: "merge-prs", Row: "42785"}, Keep: true},
				{From: ccdaemon.VerdictRef{Block: "old", Row: "a/b"}, To: ccdaemon.VerdictRef{Block: "merge-prs", Row: "42786"}, Keep: true},
			},
		},
		{name: "unpaired", from: []string{"a/1"}, wantErr: "one --to for each --from"},
		{name: "none", wantErr: "one --to for each --from"},
		{name: "no row", from: []string{"a"}, to: []string{"b/1"}, wantErr: `"a" is not <block>/<row>`},
		{name: "empty row", from: []string{"a/1"}, to: []string{"b/"}, wantErr: `"b/" is not <block>/<row>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verdictMoves(tt.from, tt.to, true)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("moves = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("move %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
