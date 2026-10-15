package cli

import (
	"testing"

	"github.com/yasyf/cc-interact/cmd"
)

func TestWindowOf(t *testing.T) {
	d := cmd.Deps{ClaudePID: func() int { return 4242 }}
	tests := []struct {
		name     string
		env      string
		session  string
		wantSess string
		wantPID  int
	}{
		{name: "empty flag takes the env session and the pid", env: "own", wantSess: "own", wantPID: 4242},
		{name: "own session passed explicitly keeps the pid", env: "own", session: "own", wantSess: "own", wantPID: 4242},
		{name: "another session drops the pid", env: "own", session: "other", wantSess: "other", wantPID: 0},
		{name: "a session with no env drops the pid", session: "other", wantSess: "other", wantPID: 0},
		{name: "no flag and no env keeps the pid", wantSess: "", wantPID: 4242},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SESSION_ID", tt.env)
			sess, pid := windowOf(d, tt.session)
			if sess != tt.wantSess || pid != tt.wantPID {
				t.Fatalf("windowOf(%q) = (%q, %d), want (%q, %d)", tt.session, sess, pid, tt.wantSess, tt.wantPID)
			}
		})
	}
}
