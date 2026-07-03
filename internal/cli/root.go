// Package cli builds the cobra command tree: cc-present's artifact commands
// (start, push, update-block, remove-block, reply, outcomes, close) layered on
// cc-interact's reusable substrate commands (daemon, watch, status, stop,
// session-record, guard-edit, channel-ack, channel).
package cli

import (
	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/app"
	"github.com/yasyf/cc-present/internal/version"
)

// NewRootCmd builds the root command and registers its subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "cc-present",
		Short:         "Ad-hoc live web artifacts for Claude sessions — approval boards, choices, and rich content whose every click streams back to the agent.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	d := app.Deps()
	root.AddCommand(
		// Substrate commands from cc-interact.
		cmd.DaemonCmd(d),
		cmd.WatchCmd(d),
		cmd.StatusCmd(d),
		cmd.StopCmd(d),
		cmd.SessionRecordCmd(d),
		cmd.GuardEditCmd(d),
		cmd.ChannelAckCmd(d),
		cmd.ChannelCmd(d),
		// cc-present artifact commands.
		newStartCmd(d),
		newPushCmd(d),
		newUpdateBlockCmd(d),
		newRemoveBlockCmd(d),
		newReplyCmd(d),
		newOutcomesCmd(d),
		newCloseCmd(d),
	)
	return root
}
