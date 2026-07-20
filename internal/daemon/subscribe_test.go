package daemon

import (
	"slices"
	"strings"
	"testing"

	"github.com/yasyf/cc-interact/agent"
	"github.com/yasyf/cc-interact/subject"
)

func TestPresentSubscribe(t *testing.T) {
	handler := []string{
		EventDecisionCreated, EventChoiceSelected, EventFeedbackCreated,
		EventInputSubmitted, EventPackInteraction, EventSubmit,
	}
	tests := []struct {
		name      string
		agentType string
		want      []string
	}{
		{"handler exact", "cc-present:present-handler", handler},
		{"handler with plugin prefix", "plugin:cc-present:present-handler", handler},
		{"other agent type", "Explore", nil},
		{"empty agent type", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := presentSubscribe(subject.Subject{}, agent.Info{AgentType: tt.agentType})
			if !slices.Equal(got, tt.want) {
				t.Fatalf("presentSubscribe(%q) = %v, want %v", tt.agentType, got, tt.want)
			}
			// Substrate contract: a teed type must never be an agent.* type, or the
			// directive would re-tee itself.
			for _, typ := range got {
				if strings.HasPrefix(typ, "agent.") {
					t.Fatalf("subscribed to agent-plane type %q", typ)
				}
			}
		})
	}
}

func TestAgentGreeting(t *testing.T) {
	got := agentGreeting(agent.Info{AgentID: "h7", AgentType: presentHandlerType})
	if !strings.Contains(got, "h7") {
		t.Fatalf("handler greeting %q omits the agent id", got)
	}
	if !strings.Contains(got, "await") {
		t.Fatalf("handler greeting %q omits the await instruction", got)
	}
	if other := agentGreeting(agent.Info{AgentID: "x1", AgentType: "Explore"}); other != "" {
		t.Fatalf("non-handler greeting = %q, want empty", other)
	}
}
