// Package app is cc-present's composition root: the state-directory layout, the
// lazy daemon launcher, the control-client constructor, and the cmd.Deps that
// wires cc-interact's substrate commands to this host. It is the echo-example
// scaffold specialized to cc-present.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/yasyf/cc-interact/channel"
	"github.com/yasyf/cc-interact/cmd"
	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/cc-interact/procs"
	"github.com/yasyf/daemonkit"
	"github.com/yasyf/daemonkit/paths"

	"github.com/yasyf/synckit/meshtrust"

	ccdaemon "github.com/yasyf/cc-present/internal/daemon"
	"github.com/yasyf/cc-present/internal/packs"
	"github.com/yasyf/cc-present/internal/version"
	"github.com/yasyf/cc-present/internal/web"
)

const (
	// appDir is the state-dir basename under the user's home (~/.cc-present).
	appDir                  = ".cc-present"
	daemonServiceLabel      = "com.yasyf.cc-present.daemon"
	daemonTeamID            = "SXKCTF23Q2"
	daemonSigningIdentifier = "cc-present"

	// channelNotifyMethod is the JSON-RPC method each subject event is pushed
	// under on the cc-present channel.
	channelNotifyMethod = "notifications/cc-present/channel"
)

// channelInstructions is folded into the agent's system prompt at the channel's
// MCP initialize, so a --channels session knows what cc-present traffic to
// expect and that silence is normal.
const channelInstructions = `This MCP server is the cc-present channel: a live web artifact whose every human click streams back to you. Activity reaches you as <channel source="cc-present" type="..."> tags whose inner JSON has a "type" field naming the event; the tag's payload is the event and is authoritative.

A channel.changed tag marks a connection-presence change, and a present.closed tag echoes the artifact's own close once you end it; both are lifecycle signals that carry no task and need no reply. Informational interactions — choice.selected, input.submitted, a decision.created that approves without a note — cost you one ledger line; submit collects their substance. Actionable interactions — feedback.created, a rejecting or change-asking decision.created, a pack.interaction whose pack demands a response — route per the cc-present skill to a short-lived delegate: one background writer for a lone event, one cc-present:present-triage worker for a burst. Never park a background handler on the board, and never write the reply or redraft prose yourself.

On submit, drain cc-present outcomes --no-doc in this session, summarize in chat, apply to the task, then start another round or close. Mid-review, a push or update-block that adds a new top-level block must declare round intent: --round current extends the review in progress, --round new advances the round — blocks not re-upserted freeze read-only. Your own mid-round authoring needs no agent lifecycle action at all.

The channel never speaks unsolicited: outside a cc-present run it is silent, and silence needs nothing from you.`

// Paths is the state-directory layout for ~/.cc-present.
func Paths() paths.Paths { return paths.Paths{App: appDir} }

// NewClient opens the daemon's business lane.
func NewClient(_ context.Context) (*ccd.Client, error) {
	l, err := launcher()
	if err != nil {
		return nil, err
	}
	return l.NewClient()
}

// daemonSpec is the one daemon identity the launcher and the daemon half both
// read: socket, state dir, and LaunchAgent all derive from its Label. The
// control lane is pinned to the signed cc-present build, so drain and
// broker-handoff admit nothing else; the business lane keeps the same-EUID
// floor a CLI's unsigned dev build has always run under.
func daemonSpec() (daemonkit.Daemon, error) {
	program, err := daemonkit.Stable()
	if err != nil {
		return daemonkit.Daemon{}, fmt.Errorf("resolve stable cc-present program: %w", err)
	}
	requirement := daemonkit.Requirement{TeamID: daemonTeamID, SigningIdentifier: daemonSigningIdentifier}
	return ccd.Spec(daemonkit.Daemon{
		Label: daemonServiceLabel, Program: program, Args: []string{"daemon"},
		Log: Paths().LogPath(), Restart: daemonkit.RestartOnFailure,
		Trust: daemonkit.Trust{Control: &requirement, Serving: daemonkit.ServingSameUser()},
	}), nil
}

func launcher() (ccd.Launcher, error) {
	spec, err := daemonSpec()
	if err != nil {
		return ccd.Launcher{}, err
	}
	return ccd.Launcher{Daemon: spec, Paths: Paths(), RuntimeBuild: version.String()}, nil
}

// usableDaemon accepts a newer incumbent instead of refusing the command. The
// refusal exists to stop a rollback, but daemonkit's handshake — not the
// release order — is what proves compatibility, so a command that only needs a
// daemon runs against the newer one; a real wire change fails at the connect.
func usableDaemon(err error) error {
	if errors.Is(err, ccd.ErrIncumbentNewer) {
		return nil
	}
	return err
}

// Deps wires cc-interact's substrate commands to cc-present's host.
func Deps() cmd.Deps {
	return cmd.Deps{
		Paths:     Paths(),
		Version:   version.String(),
		NewClient: NewClient,
		EnsureCurrent: func(ctx context.Context) error {
			l, err := launcher()
			if err != nil {
				return err
			}
			return usableDaemon(l.EnsureCurrent(ctx, ccd.UpgradeTimeout))
		},
		EnsureCurrentIfRunning: func(ctx context.Context) error {
			l, err := launcher()
			if err != nil {
				return err
			}
			return l.EnsureCurrentIfRunning(ctx)
		},
		Stop: func(ctx context.Context) error {
			l, err := launcher()
			if err != nil {
				return err
			}
			return l.Stop(ctx, ccd.UpgradeTimeout)
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
	spec, err := daemonSpec()
	if err != nil {
		return err
	}
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
	return ccdaemon.Serve(ctx, Paths(), spec, version.String(), cfg.Bind, token, loader, meshtrust.Detect())
}

// channelTools advertises no MCP tools on the cc-present channel: every subject
// event streams down as a notification and the model reacts to the tags inline.
func channelTools(_ context.Context, _, _ string) ([]channel.Tool, string, string, error) {
	return nil, channelNotifyMethod, channelInstructions, nil
}
