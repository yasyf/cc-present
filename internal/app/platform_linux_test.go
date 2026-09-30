package app

import (
	"testing"

	"github.com/yasyf/daemonkit"
)

// TestDaemonSpecRestsEveryLaneOnTheSameUserFloor asserts linux states no signed
// requirement on any lane: daemonkit has no verifier there, so one would fail
// every Open instead of narrowing who is admitted.
func TestDaemonSpecRestsEveryLaneOnTheSameUserFloor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DAEMONKIT_HOME", home)
	spec, err := daemonSpec()
	if err != nil {
		t.Fatalf("daemonSpec: %v", err)
	}
	if spec.Trust.Control != nil {
		t.Fatalf("Trust.Control = %+v, want nil (the same-EUID floor)", spec.Trust.Control)
	}
	if spec.Trust.Business != nil {
		t.Fatalf("Trust.Business = %+v, want nil (the same-EUID floor)", spec.Trust.Business)
	}
	if spec.Trust.Serving != daemonkit.ServingSameUser() {
		t.Fatalf("Trust.Serving = %+v, want ServingSameUser()", spec.Trust.Serving)
	}
}
