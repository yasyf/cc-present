package cli

import (
	"testing"

	"github.com/yasyf/cc-interact/agent"
)

func TestSoleRunningAgent(t *testing.T) {
	tests := []struct {
		name    string
		roster  []agent.Info
		want    string
		wantErr string
	}{
		{
			name:    "no agents",
			roster:  nil,
			wantErr: "no running handler agent",
		},
		{
			name:    "only a stopped agent",
			roster:  []agent.Info{{AgentID: "a1", Status: agent.StatusDone}},
			wantErr: "no running handler agent",
		},
		{
			name:   "sole running agent",
			roster: []agent.Info{{AgentID: "a1", Status: agent.StatusRunning}},
			want:   "a1",
		},
		{
			name: "running agent alongside a stopped one",
			roster: []agent.Info{
				{AgentID: "done1", Status: agent.StatusDone},
				{AgentID: "run1", Status: agent.StatusRunning},
			},
			want: "run1",
		},
		{
			name: "several running agents",
			roster: []agent.Info{
				{AgentID: "a1", Status: agent.StatusRunning},
				{AgentID: "a2", Status: agent.StatusRunning},
			},
			wantErr: "direct needs --agent: running agents are a1, a2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := soleRunningAgent(tt.roster)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("soleRunningAgent() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("soleRunningAgent() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("soleRunningAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}
