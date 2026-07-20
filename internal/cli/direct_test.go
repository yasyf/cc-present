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
			name:    "only a stopped handler",
			roster:  []agent.Info{{AgentID: "a1", AgentType: "cc-present:present-handler", Status: agent.StatusDone}},
			wantErr: "no running handler agent",
		},
		{
			name:   "sole running handler",
			roster: []agent.Info{{AgentID: "a1", AgentType: "cc-present:present-handler", Status: agent.StatusRunning}},
			want:   "a1",
		},
		{
			name: "running handler alongside a stopped one",
			roster: []agent.Info{
				{AgentID: "done1", AgentType: "cc-present:present-handler", Status: agent.StatusDone},
				{AgentID: "run1", AgentType: "cc-present:present-handler", Status: agent.StatusRunning},
			},
			want: "run1",
		},
		{
			name:    "sole running non-handler is not a target",
			roster:  []agent.Info{{AgentID: "explore1", AgentType: "Explore", Status: agent.StatusRunning}},
			wantErr: "no running handler agent",
		},
		{
			name: "handler picked over a running non-handler",
			roster: []agent.Info{
				{AgentID: "explore1", AgentType: "Explore", Status: agent.StatusRunning},
				{AgentID: "h1", AgentType: "cc-present:present-handler", Status: agent.StatusRunning},
			},
			want: "h1",
		},
		{
			name: "several running handlers",
			roster: []agent.Info{
				{AgentID: "a1", AgentType: "cc-present:present-handler", Status: agent.StatusRunning},
				{AgentID: "a2", AgentType: "cc-present:present-handler", Status: agent.StatusRunning},
			},
			wantErr: "direct needs --agent: running handlers are a1, a2",
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
