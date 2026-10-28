package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

const launchctl = "/bin/launchctl"

var launchdRunningPID = regexp.MustCompile(`(?m)^\s*pid = [1-9][0-9]*$`)

func cycleAgent(ctx context.Context) (bool, error) {
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), daemonServiceLabel)
	printed, err := exec.CommandContext(ctx, launchctl, "print", target).Output() //nolint:gosec // G204: argv to /bin/launchctl naming this daemon's own job, never a shell
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("launchctl print %s: %w", target, err)
	}
	if !launchdRunningPID.Match(printed) {
		return false, nil
	}
	if out, err := exec.CommandContext(ctx, launchctl, "kickstart", "-k", target).CombinedOutput(); err != nil { //nolint:gosec // G204: argv to /bin/launchctl naming this daemon's own job, never a shell
		return false, fmt.Errorf("launchctl kickstart -k %s: %w: %s", target, err, out)
	}
	return true, nil
}
