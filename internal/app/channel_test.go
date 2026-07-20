package app

import (
	"context"
	"testing"
)

func TestChannelToolsExposesAwait(t *testing.T) {
	tools, method, instructions, err := channelTools(context.Background(), "sess", "scope")
	if err != nil {
		t.Fatalf("channelTools() error = %v", err)
	}
	if method != channelNotifyMethod {
		t.Fatalf("notify method = %q, want %q", method, channelNotifyMethod)
	}
	if instructions != channelInstructions {
		t.Fatal("channelTools() changed the channel instructions")
	}
	if len(tools) != 1 || tools[0].Name != "await" {
		names := make([]string, len(tools))
		for i, tool := range tools {
			names[i] = tool.Name
		}
		t.Fatalf("channel tools = %v, want [await]", names)
	}
}
