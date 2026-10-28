package app

import (
	"context"

	"github.com/yasyf/daemonkit"
)

// Supervise runs the foreground supervisor that owns the cc-present daemon on
// linux, where the workspace stands in for launchd. It returns once ctx ends
// and the daemon has drained.
func Supervise(ctx context.Context) error {
	return daemonkit.Supervise(ctx, daemonServiceLabel)
}

func cycleAgent(context.Context) (bool, error) { return false, nil }
