package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/yasyf/daemonkit"
)

const (
	daemonTeamID            = "SXKCTF23Q2"
	daemonSigningIdentifier = "cc-present"
)

// daemonTrust pins the control lane to the signed cc-present build, so drain
// and broker-handoff admit nothing else; the business lane keeps the same-EUID
// floor a CLI's unsigned dev build has always run under.
func daemonTrust() daemonkit.Trust {
	requirement := daemonkit.Requirement{TeamID: daemonTeamID, SigningIdentifier: daemonSigningIdentifier}
	return daemonkit.Trust{Control: &requirement, Serving: daemonkit.ServingSameUser()}
}

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
