package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"
)

func TestRoundFlagUsage(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(cmd.Deps) *cobra.Command
		args []string
		want string
	}{
		{"push rejects an unknown round", newPushCmd, []string{"--round", "bogus", "-"}, `--round must be "current" or "new"`},
		{"push rejects round-title without new", newPushCmd, []string{"--round-title", "Two", "-"}, `--round-title requires --round new`},
		{"push rejects round-title with current", newPushCmd, []string{"--round", "current", "--round-title", "Two", "-"}, `--round-title requires --round new`},
		{"update-block rejects an unknown round", newUpdateBlockCmd, []string{"--round", "bogus", "-"}, `--round must be "current" or "new"`},
		{"update-block rejects round-title without new", newUpdateBlockCmd, []string{"--round-title", "Two", "-"}, `--round-title requires --round new`},
		{"update-block rejects round-title with current", newUpdateBlockCmd, []string{"--round", "current", "--round-title", "Two", "-"}, `--round-title requires --round new`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.cmd(cmd.Deps{})
			var out, errOut bytes.Buffer
			c.SetArgs(tt.args)
			c.SetOut(&out)
			c.SetErr(&errOut)
			err := c.Execute()
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Execute() error = %v, want %q", err, tt.want)
			}
		})
	}
}
