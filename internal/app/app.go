// Package app is cc-present's composition root: the state-directory layout, the
// lazy daemon launcher, the control-client constructor, and the cmd.Deps that
// wires cc-interact's substrate commands to this host. It is the echo-example
// scaffold specialized to cc-present.
package app

import (
	"context"

	"github.com/yasyf/cc-interact/channel"
	"github.com/yasyf/cc-interact/cmd"
	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/cc-interact/paths"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/procs"
	"github.com/yasyf/cc-present/internal/version"
)

const (
	// appDir is the state-dir basename under the user's home (~/.cc-present).
	appDir = ".cc-present"

	// channelNotifyMethod is the JSON-RPC method each subject event is pushed
	// under on the cc-present channel.
	channelNotifyMethod = "notifications/cc-present/channel"
)

// channelInstructions is folded into the agent's system prompt at the channel's
// MCP initialize, so a --channels session knows what cc-present traffic to
// expect and that silence is normal.
const channelInstructions = `This MCP server is the cc-present channel: a live web artifact whose every human click streams back to you. Activity reaches you as <channel source="cc-present" type="..."> tags whose inner JSON has a "type" field naming the event.

A channel.changed tag marks a connection-presence change, and a present.closed tag echoes the artifact's own close once you end it; both are lifecycle signals that carry no task and need no reply. Human interactions arrive as decision.created (approve/reject/clear on an approval block), choice.selected (a pick on a choice block), feedback.created (free-text feedback under an approval), input.submitted (an input field's value), and submit (the human pressed Submit for a revision). Handle each per the cc-present skill: reply under feedback and redraft blocks with update-block, and on submit run outcomes, apply, then start another round or close.

The channel never speaks unsolicited: outside a cc-present run it is silent, and silence needs nothing from you.`

// Paths is the state-directory layout for ~/.cc-present.
func Paths() paths.Paths { return paths.Paths{App: appDir} }

// NewClient returns a control-socket client for the daemon.
func NewClient() *ccd.Client { return ccd.NewClient(Paths().SocketPath()) }

func launcher() ccd.Launcher {
	return ccd.Launcher{Paths: Paths(), Version: version.String(), Args: []string{"daemon"}}
}

// Deps wires cc-interact's substrate commands to cc-present's host.
func Deps() cmd.Deps {
	return cmd.Deps{
		Paths:                  Paths(),
		Version:                version.String(),
		NewClient:              NewClient,
		EnsureCurrent:          func(context.Context) error { return launcher().EnsureCurrent(ccd.UpgradeTimeout) },
		EnsureCurrentIfRunning: func() error { return launcher().EnsureCurrentIfRunning() },
		ClaudePID:              procs.ClaudePID,
		WindowAlive:            procs.LiveClaude,
		TerminalEvent:          func(t string) bool { return t == ccdaemon.EventPresentClosed },
		Serve:                  serve,
		ChannelTools:           channelTools,
	}
}

// serve runs the long-lived daemon, binding and authenticating the HTTP plane
// per the host config: an absent config binds loopback with no token, exactly
// today's behavior.
func serve(ctx context.Context) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	token, err := ReadToken()
	if err != nil {
		return err
	}
	return ccdaemon.Serve(ctx, Paths(), version.String(), cfg.Bind, token)
}

// channelTools advertises zero domain tools: the cc-present channel is a pure
// notification transport, so it round-trips nothing back through a tool. The
// substrate still streams every subject event down the channel as a
// notification. channel.NewServer accepts the empty tool list.
func channelTools(context.Context, string, string) ([]channel.Tool, string, string, error) {
	return nil, channelNotifyMethod, channelInstructions, nil
}
