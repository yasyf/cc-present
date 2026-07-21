package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/daemonkit/daemonrole"
	"github.com/yasyf/daemonkit/paths"

	"github.com/yasyf/cc-present/internal/packs"
)

const markdownBlock = `{"id":"m1","type":"markdown","md":"hi"}`

func testDaemonRole(t *testing.T, dir string) daemonrole.Classifier {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	rolePath := filepath.Join(dir, "cc-present")
	if err := os.Symlink(executable, rolePath); err != nil {
		t.Fatalf("role alias: %v", err)
	}
	return daemonrole.Classifier{RoleID: "com.yasyf.cc-present.test", RolePath: rolePath}
}

func TestBuildServerDefersGenerationState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	p := paths.Paths{App: "d"}
	if _, err := BuildServer(context.Background(), p, testDaemonRole(t, home), "business-v1", "v1.0.0", "", "", packs.NewLoader(nil, nil), nil); err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	for _, path := range []string{p.DBPath(), filepath.Join(p.StateDir(), "assets")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("BuildServer acquired %s: %v", path, err)
		}
	}
}

// startTestDaemon boots a real cc-present daemon over its control socket and
// returns a typed client once it answers Health — the full RPC path, so
// dispatch applies ScopeResolve exactly as in production. HOME is a short /tmp
// dir because paths.SocketPath() derives from it and the default t.TempDir()
// (/var/folders/…) would overflow the ~104-byte sun_path limit.
func startTestDaemon(ctx context.Context, t *testing.T) *Client {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "ccp")
	if err != nil {
		t.Fatalf("temp home: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

	p := paths.Paths{App: "d"}
	role := testDaemonRole(t, home)
	ctx, cancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, p, role, "business-v1", "v1.0.0", "", "", packs.NewLoader(nil, nil), nil)
		close(errCh)
	}()
	// Closing errCh after the send lets cleanup's receive return even when the
	// health poll already drained the value on an early daemon exit.
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-errCh:
			t.Fatalf("daemon exited before becoming healthy: %v", err)
		default:
		}
		raw, err := ccd.NewClient(ctx, ccd.ClientConfig{Socket: p.SocketPath(), Build: "business-v1", LifecycleBuild: "v1.0.0"})
		if err == nil {
			t.Cleanup(func() { _ = raw.Close() })
			return NewClient(raw)
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("daemon did not become healthy within 5s")
	return nil
}

// TestArtifactResolutionIsCwdIndependent replays the incident end to end through
// dispatch: commands issued from cwds other than the one `start` ran from must
// resolve to the same window-owned artifact, never fail with "no cc-present
// artifact".
func TestArtifactResolutionIsCwdIndependent(t *testing.T) {
	ctx := context.Background()

	t.Run("mutations resolve across cwds to one subject", func(t *testing.T) {
		cl := startTestDaemon(ctx, t)
		start, err := cl.Start(ctx, "s1", "/repo-root", 0, false, "", json.RawMessage(approvalDoc))
		if err != nil {
			t.Fatalf("start: %v", err)
		}
		if start.SubjectID == "" {
			t.Fatal("start returned no subject id")
		}
		// Each op below runs from a different cwd than start; window-owned scope
		// must resolve them all onto start's artifact.
		if _, err := cl.Round(ctx, "s1", "/plugin/cache/elsewhere", 0, "Round Two"); err != nil {
			t.Fatalf("round from a foreign cwd: %v", err)
		}
		if _, err := cl.UpsertBlock(ctx, "s1", "/tmp/other", 0, json.RawMessage(markdownBlock), "", "", ""); err != nil {
			t.Fatalf("upsert-block from a foreign cwd: %v", err)
		}
		if _, err := cl.Outcomes(ctx, "s1", "/yet/another", 0); err != nil {
			t.Fatalf("outcomes from a foreign cwd: %v", err)
		}
		got, _, err := cl.Resolve(ctx, "s1", "/fifth/cwd", 0)
		if err != nil {
			t.Fatalf("resolve from a foreign cwd: %v", err)
		}
		if got != start.SubjectID {
			t.Fatalf("resolve subject = %q, want %q (same artifact across cwds)", got, start.SubjectID)
		}
		if _, err := cl.Close(ctx, "s1", "/sixth/cwd", 0, "done"); err != nil {
			t.Fatalf("close from a foreign cwd: %v", err)
		}
	})

	t.Run("pid fallback across cwd", func(t *testing.T) {
		cl := startTestDaemon(ctx, t)
		start, err := cl.Start(ctx, "s1", "/a", 4242, false, "T", nil)
		if err != nil {
			t.Fatalf("start: %v", err)
		}
		// A blank session drops resolution to the pid path; a different cwd must
		// still land on the same window's artifact.
		got, _, err := cl.Resolve(ctx, "", "/b", 4242)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if got != start.SubjectID {
			t.Fatalf("pid-fallback resolve = %q, want %q", got, start.SubjectID)
		}
	})

	t.Run("another window cannot adopt", func(t *testing.T) {
		cl := startTestDaemon(ctx, t)
		if _, err := cl.Start(ctx, "s1", "/repo-root", 0, false, "T", nil); err != nil {
			t.Fatalf("start: %v", err)
		}
		// A second session with no matching pid owns nothing here, even at the
		// identical cwd: ownership is per-window, never per-directory.
		got, _, err := cl.Resolve(ctx, "s2", "/repo-root", 0)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if got != "" {
			t.Fatalf("foreign window resolved subject %q, want empty", got)
		}
	})
}

// TestStartResumesAcrossCwd asserts a resume (fresh=false) rebinds to the same
// artifact from a new cwd, while --new detaches it and creates a distinct one
// that subsequent resolution then finds.
func TestStartResumesAcrossCwd(t *testing.T) {
	ctx := context.Background()
	cl := startTestDaemon(ctx, t)

	first, err := cl.Start(ctx, "s1", "/a", 0, false, "T", nil)
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	resume, err := cl.Start(ctx, "s1", "/b", 0, false, "T", nil)
	if err != nil {
		t.Fatalf("resume start: %v", err)
	}
	if resume.SubjectID != first.SubjectID {
		t.Fatalf("resume from a new cwd = %q, want %q (same artifact)", resume.SubjectID, first.SubjectID)
	}

	fresh, err := cl.Start(ctx, "s1", "/c", 0, true, "T", nil)
	if err != nil {
		t.Fatalf("fresh start: %v", err)
	}
	if fresh.SubjectID == first.SubjectID {
		t.Fatalf("fresh start reused subject %q instead of creating a new one", first.SubjectID)
	}

	got, _, err := cl.Resolve(ctx, "s1", "/d", 0)
	if err != nil {
		t.Fatalf("resolve after fresh: %v", err)
	}
	if got != fresh.SubjectID {
		t.Fatalf("resolve after fresh = %q, want the new subject %q", got, fresh.SubjectID)
	}
	if got == first.SubjectID {
		t.Fatal("resolve after fresh returned the detached original subject")
	}
}
