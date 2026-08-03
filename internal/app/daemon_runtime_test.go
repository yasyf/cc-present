package app

import (
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/daemonkit"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/version"
)

// TestDaemonSpecCarriesAWholeDocument pins the real body ceiling, which is not
// MaxFrame: a frame spends 4 bytes per 3 of payload beside a 4 KiB envelope
// reserve, so the largest document the wire moves is MaxDetail(MaxFrame). The
// frame itself comes from ccd.Spec's default, so a change to cc-interact's
// payload budget or to daemonkit's reserve would otherwise clip a legal
// document silently rather than fail.
func TestDaemonSpecCarriesAWholeDocument(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DAEMONKIT_HOME", home)
	spec, err := daemonSpec()
	if err != nil {
		t.Fatalf("daemonSpec: %v", err)
	}
	if got := daemonkit.MaxDetail(spec.MaxFrame); got < doc.MaxDocBytes {
		t.Fatalf("MaxDetail(MaxFrame=%d) = %d, want >= MaxDocBytes %d", spec.MaxFrame, got, doc.MaxDocBytes)
	}
}

func TestDaemonSpecDeclaresLabelSchemaAndLaunchAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DAEMONKIT_HOME", home)
	spec, err := daemonSpec()
	if err != nil {
		t.Fatalf("daemonSpec: %v", err)
	}
	// Pinned literally, not against the constant: the label names the LaunchAgent
	// and derives the socket and state dir, so editing it orphans a running
	// daemon and its plist rather than renaming them.
	if spec.Label != "com.yasyf.cc-present.daemon" {
		t.Fatalf("Label = %q, want com.yasyf.cc-present.daemon", spec.Label)
	}
	if len(spec.Schemas) == 0 || spec.Schemas[0] != ccd.WireBuild {
		t.Fatalf("Schemas = %v, want %q first (the identity must be built through ccd.Spec)", spec.Schemas, ccd.WireBuild)
	}
	if spec.Log != Paths().LogPath() {
		t.Fatalf("Log = %q, want %q", spec.Log, Paths().LogPath())
	}
	if spec.Restart != daemonkit.RestartOnFailure {
		t.Fatalf("Restart = %v, want RestartOnFailure", spec.Restart)
	}
	if got := spec.Args; len(got) != 1 || got[0] != "daemon" {
		t.Fatalf("Args = %v, want [daemon]", got)
	}
	if err := spec.ValidateForClient(); err != nil {
		t.Fatalf("ValidateForClient: %v", err)
	}
}

// TestLauncherCarriesEveryRequiredField exercises ccd.Launcher.validate, which
// demands Paths and RuntimeBuild beside the identity. Nothing else in the suite
// reaches it, so without this a dropped field would surface only at runtime, on
// a user's first command. NewClient validates without dialing, so no daemon
// need be running.
func TestLauncherCarriesEveryRequiredField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DAEMONKIT_HOME", home)
	l, err := launcher()
	if err != nil {
		t.Fatalf("launcher: %v", err)
	}
	client, err := l.NewClient()
	if err != nil {
		t.Fatalf("Launcher.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if l.Paths != Paths() {
		t.Fatalf("Paths = %+v, want %+v", l.Paths, Paths())
	}
	if l.RuntimeBuild != version.String() {
		t.Fatalf("RuntimeBuild = %q, want %q", l.RuntimeBuild, version.String())
	}
}

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
