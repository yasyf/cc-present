package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
)

func TestUsableDaemon(t *testing.T) {
	other := errors.New("daemon: probe incumbent: connection refused")
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"a healthy ensure stays healthy", nil, nil},
		{"a newer incumbent is usable", ccd.ErrIncumbentNewer, nil},
		{
			"the wrapped refusal a version-skewed client actually sees is usable",
			fmt.Errorf("%w: %s supersedes this build %s", ccd.ErrIncumbentNewer, "0.33.5", "0.33.0"),
			nil,
		},
		{"every other failure still fails", other, other},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usableDaemon(tt.err); !errors.Is(got, tt.want) {
				t.Fatalf("usableDaemon(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestWedgeRecovery(t *testing.T) {
	wedged := errors.New("daemon: ensure: no daemon is listening: context deadline exceeded")
	refused := errors.New("daemon: probe incumbent: handshake rejected")
	probeFailed := errors.New("daemonkit: open: invalid label")
	cycleFailed := errors.New("launchctl kickstart -k: exit status 1")
	tests := []struct {
		name          string
		first         error
		silent        bool
		probeErr      error
		cycled        bool
		cycleErr      error
		reensure      error
		wantProbes    int
		wantCycles    int
		wantReensures int
		wantErr       error
	}{
		{name: "a healthy ensure is left alone", first: nil, wantErr: nil},
		{name: "a newer incumbent is never cycled", first: ccd.ErrIncumbentNewer, silent: true, cycled: true, wantErr: ccd.ErrIncumbentNewer},
		{name: "a daemon that still answers health is never cycled", first: refused, silent: false, cycled: true, wantProbes: 1, wantErr: refused},
		{name: "a parked daemon launchd still runs is cycled and re-ensured", first: wedged, silent: true, cycled: true, wantProbes: 1, wantCycles: 1, wantReensures: 1, wantErr: nil},
		{name: "a parked daemon the probe reads as absent is cycled", first: ccd.ErrNoPeer, silent: true, cycled: true, wantProbes: 1, wantCycles: 1, wantReensures: 1, wantErr: nil},
		{name: "proven absence with nothing under launchd stays absent", first: ccd.ErrNoPeer, silent: true, cycled: false, wantProbes: 1, wantCycles: 1, wantErr: ccd.ErrNoPeer},
		{name: "a failed probe reports both failures", first: wedged, probeErr: probeFailed, wantProbes: 1, wantErr: probeFailed},
		{name: "a failed cycle reports both failures", first: wedged, silent: true, cycleErr: cycleFailed, wantProbes: 1, wantCycles: 1, wantErr: cycleFailed},
		{name: "a re-ensure that still fails reports its own failure", first: wedged, silent: true, cycled: true, reensure: wedged, wantProbes: 1, wantCycles: 1, wantReensures: 1, wantErr: wedged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probes, cycles, reensures := 0, 0, 0
			r := wedgeRecovery{
				silent: func(context.Context) (bool, error) {
					probes++
					return tt.silent, tt.probeErr
				},
				cycle: func(context.Context) (bool, error) {
					cycles++
					return tt.cycled, tt.cycleErr
				},
				reensure: func(context.Context) error {
					reensures++
					return tt.reensure
				},
			}
			if err := r.after(context.Background(), tt.first); !errors.Is(err, tt.wantErr) {
				t.Fatalf("after(%v) = %v, want %v", tt.first, err, tt.wantErr)
			}
			if probes != tt.wantProbes || cycles != tt.wantCycles || reensures != tt.wantReensures {
				t.Fatalf("probes, cycles, reensures = %d, %d, %d, want %d, %d, %d",
					probes, cycles, reensures, tt.wantProbes, tt.wantCycles, tt.wantReensures)
			}
		})
	}
}
