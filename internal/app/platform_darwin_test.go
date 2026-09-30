package app

import (
	"testing"

	"github.com/yasyf/daemonkit"
)

// TestDaemonSpecPinsControlToTheSignedBuild asserts the lane split the role
// collapse produced: drain and broker-handoff demand the signed cc-present
// identity, while the business lane stays on the same-EUID floor an unsigned
// dev build can meet.
func TestDaemonSpecPinsControlToTheSignedBuild(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DAEMONKIT_HOME", home)
	spec, err := daemonSpec()
	if err != nil {
		t.Fatalf("daemonSpec: %v", err)
	}
	// Pinned literally: this is the identity the control lane admits, so a typo
	// in the constants would otherwise move the gate and the assertion together.
	want := daemonkit.Requirement{TeamID: "SXKCTF23Q2", SigningIdentifier: "cc-present"}
	if spec.Trust.Control == nil || spec.Trust.Control.Digest() != want.Digest() {
		t.Fatalf("Trust.Control = %+v, want %+v", spec.Trust.Control, want)
	}
	if spec.Trust.Business != nil {
		t.Fatalf("Trust.Business = %+v, want nil (the same-EUID floor)", spec.Trust.Business)
	}
	if spec.Trust.Serving != daemonkit.ServingSameUser() {
		t.Fatalf("Trust.Serving = %+v, want ServingSameUser()", spec.Trust.Serving)
	}
}
