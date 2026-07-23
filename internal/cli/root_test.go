package cli

import (
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
)

func TestRootRegistersHiddenStopController(t *testing.T) {
	command, _, err := NewRootCmd().Find([]string{ccd.StopControlCommand})
	if err != nil {
		t.Fatalf("find stop controller: %v", err)
	}
	if command.Name() != ccd.StopControlCommand || !command.Hidden {
		t.Fatalf("stop controller = %q hidden=%v", command.Name(), command.Hidden)
	}
}
