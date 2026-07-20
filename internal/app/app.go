// Package app is cc-present's composition root: the state-directory layout, the
// lazy daemon launcher, the control-client constructor, and the cmd.Deps that
// wires cc-interact's substrate commands to this host. It is the echo-example
// scaffold specialized to cc-present.
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/yasyf/cc-interact/channel"
	"github.com/yasyf/cc-interact/cmd"
	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/cc-interact/procs"
	"github.com/yasyf/daemonkit/paths"

	"github.com/yasyf/synckit/meshtrust"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/packs"
	"github.com/yasyf/cc-present/internal/version"
	"github.com/yasyf/cc-present/internal/web"
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

A channel.changed tag marks a connection-presence change, and a present.closed tag echoes the artifact's own close once you end it; both are lifecycle signals that carry no task and need no reply. Human interactions arrive as decision.created (approve/reject/clear on an approval block), choice.selected (a pick on a choice block), feedback.created (free-text feedback under an approval), input.submitted (an input field's value), pack.interaction (a pack block's declared interaction payload), and submit (the human pressed Submit for a revision). Handle each per the cc-present skill: reply under feedback and redraft blocks with update-block, and on submit run outcomes, apply, then start another round or close.

The channel never speaks unsolicited: outside a cc-present run it is silent, and silence needs nothing from you.

When a human decision reshapes later steps, announce first — cc-present revising <block-ids...> --note "why" — then upsert the same ids with cc-present update-block; put conditional logic in revisions, not in step prose.`

// Paths is the state-directory layout for ~/.cc-present.
func Paths() paths.Paths { return paths.Paths{App: appDir} }

// NewClient opens an exact-build persistent control session.
func NewClient(ctx context.Context) (*ccd.Client, error) { return launcher().NewClient(ctx) }

func launcher() ccd.Launcher {
	return ccd.Launcher{Paths: Paths(), Version: version.String(), Args: []string{"daemon"}}
}

// Deps wires cc-interact's substrate commands to cc-present's host.
func Deps() cmd.Deps {
	return cmd.Deps{
		Paths:     Paths(),
		Version:   version.String(),
		NewClient: NewClient,
		EnsureCurrent: func(ctx context.Context) error {
			return launcher().EnsureCurrent(ctx, ccd.UpgradeTimeout)
		},
		EnsureCurrentIfRunning: func(ctx context.Context) error {
			return launcher().EnsureCurrentIfRunning(ctx)
		},
		ClaudePID:     procs.ClaudePID,
		WindowAlive:   procs.LiveClaude,
		TerminalEvent: func(t string) bool { return t == ccdaemon.EventPresentClosed },
		Serve:         serve,
		ChannelTools:  channelTools,
	}
}

// serve runs the long-lived daemon, binding and authenticating the HTTP plane
// per the host config: an absent config binds loopback with no token. When
// the mesh state exists, its hosts are additionally trusted by their
// tailnet addresses (meshtrust.Detect).
func serve(ctx context.Context) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	token, err := ReadToken()
	if err != nil {
		return err
	}
	loader := packs.NewLoader(cfg.PackDirs, cfg.DisabledPacks)
	if err := web.Validate(); err != nil {
		return fmt.Errorf("validate embedded web build: %w", err)
	}
	return ccdaemon.Serve(ctx, Paths(), version.String(), cfg.Bind, token, loader, meshtrust.Detect())
}

// awaitTimeout is the await tool's default long-poll window, under typical HTTP
// idle limits.
const awaitTimeout = 4 * time.Minute

// channelTools advertises the await MCP tool: a handler agent long-polls it for
// operator directives and teed human interactions addressed to its agent_id.
// Every subject event still streams down the channel as a notification.
func channelTools(_ context.Context, session, scope string) ([]channel.Tool, string, string, error) {
	await := channel.NewAwaitTool(channel.AwaitSpec{
		Resolve: func(ctx context.Context) (string, int, error) {
			raw, err := NewClient(ctx)
			if err != nil {
				return "", 0, err
			}
			defer func() { _ = raw.Close() }()
			cl := ccdaemon.NewClient(raw)
			for {
				subjectID, port, err := cl.Resolve(ctx, session, scope, procs.ClaudePID())
				if err != nil {
					return "", 0, err
				}
				if subjectID != "" {
					return subjectID, port, nil
				}
				select {
				case <-ctx.Done():
					return "", 0, ctx.Err()
				case <-time.After(time.Second):
				}
			}
		},
		Timeout: awaitTimeout,
	})
	return []channel.Tool{await}, channelNotifyMethod, channelInstructions, nil
}
