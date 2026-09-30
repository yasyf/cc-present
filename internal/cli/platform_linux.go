package cli

import (
	"github.com/spf13/cobra"

	"github.com/yasyf/cc-present/internal/app"
)

func platformCmds() []*cobra.Command { return []*cobra.Command{newSuperviseCmd()} }

// newSuperviseCmd runs the daemon's foreground supervisor. The workspace starts
// it and keeps it running; every other command converges the daemon through it.
func newSuperviseCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "supervise",
		Short:  "Supervise the cc-present daemon in the foreground",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return app.Supervise(c.Context())
		},
	}
}
