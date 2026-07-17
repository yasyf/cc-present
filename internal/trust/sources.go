package trust

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/yasyf/synckit/hostregistry"
)

// appBundleTailscale is where the Tailscale macOS app ships its CLI; the
// daemon's spawn environment often lacks the shell's PATH.
const appBundleTailscale = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"

// StatePath returns the synckit mesh state file this machine's trust derives
// from.
func StatePath() (string, error) {
	path, err := hostregistry.Mesh.Path()
	if err != nil {
		return "", fmt.Errorf("resolve synckit state path: %w", err)
	}
	return path, nil
}

func loadRegistry() (registry, error) {
	g, err := hostregistry.Mesh.Load()
	if err != nil {
		return registry{}, fmt.Errorf("load synckit registry: %w", err)
	}
	return registry{Self: g.Self, Hosts: g.Hosts}, nil
}

func tailscaleStatus(ctx context.Context) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if errors.Is(err, exec.ErrNotFound) {
		out, err = exec.CommandContext(ctx, appBundleTailscale, "status", "--json").Output()
	}
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}
	return out, nil
}
