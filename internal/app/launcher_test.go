package app

import (
	"errors"
	"fmt"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
)

func TestUsableDaemon(t *testing.T) {
	other := errors.New("daemon: probe incumbent: connection refused")
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"a healthy ensure stays healthy", nil, nil},
		{"a newer incumbent is usable", ccd.ErrIncumbentNewer, nil},
		{
			"the wrapped refusal a version-skewed client actually sees is usable",
			fmt.Errorf("%w: %s supersedes this build %s", ccd.ErrIncumbentNewer, "0.33.5", "0.33.0"),
			nil,
		},
		{"every other failure still fails", other, other},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usableDaemon(tt.err); !errors.Is(got, tt.want) {
				t.Fatalf("usableDaemon(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
