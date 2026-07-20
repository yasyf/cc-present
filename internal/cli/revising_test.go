package cli

import (
	"bytes"
	"testing"

	"github.com/yasyf/cc-interact/cmd"
)

func TestRevisingClearUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"clear with ids", []string{"--clear", "b1"}},
		{"clear with note", []string{"--clear", "--note", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newRevisingCmd(cmd.Deps{})
			var out, errOut bytes.Buffer
			c.SetArgs(tt.args)
			c.SetOut(&out)
			c.SetErr(&errOut)
			err := c.Execute()
			if err == nil || err.Error() != "--clear takes no block ids and no --note" {
				t.Fatalf("Execute() error = %v, want %q", err, "--clear takes no block ids and no --note")
			}
		})
	}
}
