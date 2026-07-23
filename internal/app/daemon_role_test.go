package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
)

func TestDaemonRoleUsesStableCommandAlias(t *testing.T) {
	bin := t.TempDir()
	first, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	second := "/bin/sh"
	alias := filepath.Join(bin, "cc-present")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	role, err := daemonRole()
	if err != nil {
		t.Fatalf("daemonRole: %v", err)
	}
	if role.RoleID != daemonRoleID {
		t.Fatalf("RoleID = %q, want %q", role.RoleID, daemonRoleID)
	}
	if role.RolePath != alias {
		t.Fatalf("RolePath = %q, want stable alias %q", role.RolePath, alias)
	}

	next := filepath.Join(bin, ".cc-present.next")
	if err := os.Symlink(second, next); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, alias); err != nil {
		t.Fatal(err)
	}
	upgraded, err := daemonRole()
	if err != nil {
		t.Fatalf("daemonRole after retarget: %v", err)
	}
	if upgraded != role {
		t.Fatalf("role changed across alias retarget: got %+v, want %+v", upgraded, role)
	}

	l, err := launcher()
	if err != nil {
		t.Fatalf("launcher: %v", err)
	}
	if l.WireBuild != ccd.WireBuild {
		t.Fatalf("WireBuild = %q, want %q", l.WireBuild, ccd.WireBuild)
	}
	if !reflect.DeepEqual(l.StopArgs, []string{ccd.StopControlCommand}) {
		t.Fatalf("StopArgs = %q, want %q", l.StopArgs, []string{ccd.StopControlCommand})
	}
}
