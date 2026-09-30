package app

import (
	"context"

	"github.com/yasyf/daemonkit"
)

// daemonTrust rests every lane on the same-EUID floor: linux has no code-signing
// verifier, so a signed requirement would fail at Open rather than admit anyone.
func daemonTrust() daemonkit.Trust {
	return daemonkit.Trust{Serving: daemonkit.ServingSameUser()}
}

// Supervise runs the foreground supervisor that owns the cc-present daemon on
// linux, where the workspace stands in for launchd. It returns once ctx ends
// and the daemon has drained.
func Supervise(ctx context.Context) error {
	return daemonkit.Supervise(ctx, daemonServiceLabel)
}
