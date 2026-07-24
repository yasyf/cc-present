package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/daemonkit/service"
	"github.com/yasyf/daemonkit/trust"
)

func TestDaemonRuntimeUsesExactServiceAndRoles(t *testing.T) {
	l, err := launcher()
	if err != nil {
		t.Fatalf("launcher: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	wantAgent := service.Agent{
		Label: daemonServiceLabel, Program: executable, Args: []string{"daemon"},
		LogPath: Paths().LogPath(), RestartPolicy: service.RestartOnFailure,
	}
	if l.WireBuild != ccd.WireBuild {
		t.Fatalf("WireBuild = %q, want %q", l.WireBuild, ccd.WireBuild)
	}
	if !reflect.DeepEqual(l.Agent, wantAgent) {
		t.Fatalf("Agent = %+v, want %+v", l.Agent, wantAgent)
	}
	if l.Roles != daemonRoles() {
		t.Fatalf("Roles = %+v, want %+v", l.Roles, daemonRoles())
	}
}

func TestDaemonTrustPolicySeparatesLifecycleAndStopAuthority(t *testing.T) {
	roles := daemonRoles()
	if roles.Business != trust.UnprotectedRole || roles.Lifecycle == roles.StopControl {
		t.Fatalf("roles = %+v", roles)
	}
	policy, err := daemonTrustPolicy()
	if err != nil {
		t.Fatalf("daemonTrustPolicy: %v", err)
	}
	want := trust.Requirement{
		TeamID: daemonTeamID, SigningIdentifier: daemonSigningIdentifier,
		RequiredEntitlements: map[string]trust.EntitlementRequirement{},
	}
	for _, role := range []trust.PeerRole{roles.Lifecycle, roles.StopControl} {
		got, ok := policy.Requirement(role)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("requirement[%q] = %+v, %v; want %+v", role, got, ok, want)
		}
	}
	if !policy.AllowsUnprotected() || !policy.AllowsReceipt(roles.Lifecycle) || !policy.AllowsReadiness(roles.Lifecycle) {
		t.Fatalf("lifecycle policy does not grant exact business, receipt, and readiness authority")
	}
	if policy.AllowsStop(roles.Lifecycle) || !policy.AllowsStop(roles.StopControl) {
		t.Fatalf("stop authority is not isolated to %q", roles.StopControl)
	}
}
