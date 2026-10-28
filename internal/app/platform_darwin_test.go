package app

import "testing"

func TestLaunchdRunningPIDReadsOnlyARunningJob(t *testing.T) {
	running := "gui/501/com.yasyf.cc-present.daemon = {\n\tactive count = 1\n\tstate = running\n\tpid = 63986\n\tlast exit code = (never exited)\n}\n"
	stopped := "gui/501/com.yasyf.cc-present.daemon = {\n\tactive count = 0\n\tstate = not running\n\tlast exit code = 0\n}\n"
	if !launchdRunningPID.MatchString(running) {
		t.Fatalf("launchdRunningPID missed the pid line of a running job")
	}
	if launchdRunningPID.MatchString(stopped) {
		t.Fatalf("launchdRunningPID matched a job with no pid")
	}
}
