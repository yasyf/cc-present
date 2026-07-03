package state_test

import (
	"embed"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

//go:embed testdata/*.json
var fixturesFS embed.FS

type fixture struct {
	Name     string          `json:"name"`
	Events   []state.Event   `json:"events"`
	Expected json.RawMessage `json:"expected"`
}

// TestFixtures drives the reducer entirely from the language-neutral JSON
// fixtures in testdata; Phase 2's TypeScript reducer consumes the same files.
// An expected state may omit an interaction map that stays empty: initMaps
// normalizes it to the empty map the reducer always produces.
func TestFixtures(t *testing.T) {
	entries, err := fixturesFS.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no fixtures found in testdata")
	}
	for _, entry := range entries {
		name := "testdata/" + entry.Name()
		data, err := fixturesFS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var fx fixture
		if err := json.Unmarshal(data, &fx); err != nil {
			t.Fatalf("unmarshal fixture %s: %v", name, err)
		}
		if fx.Name == "" {
			t.Fatalf("fixture %s has no name", name)
		}
		t.Run(fx.Name, func(t *testing.T) {
			got, err := state.Reduce(fx.Events)
			if err != nil {
				t.Fatalf("Reduce() error = %v", err)
			}
			var want state.State
			if err := json.Unmarshal(fx.Expected, &want); err != nil {
				t.Fatalf("unmarshal expected: %v", err)
			}
			initMaps(&want)
			assertStateEqual(t, got, want)
		})
	}
}

func TestReduceErrors(t *testing.T) {
	tests := []struct {
		name    string
		events  []state.Event
		wantErr string
	}{
		{
			name:    "unknown event type",
			events:  []state.Event{{Origin: "agent", Type: "bogus.event", Seq: 1, Payload: []byte(`{}`)}},
			wantErr: "unknown event type",
		},
		{
			name: "event after close is terminal",
			events: []state.Event{
				{Origin: "agent", Type: "present.closed", Seq: 1, Payload: []byte(`{}`)},
				{Origin: "human", Type: "submit", Seq: 2, Payload: []byte(`{"revision":1}`)},
			},
			wantErr: "terminal",
		},
		{
			name:    "invalid verdict",
			events:  []state.Event{{Origin: "human", Type: "decision.created", Seq: 1, Payload: []byte(`{"blockId":"a1","verdict":"maybe"}`)}},
			wantErr: "invalid verdict",
		},
		{
			name:    "malformed block on upsert",
			events:  []state.Event{{Origin: "agent", Type: "block.upserted", Seq: 1, Payload: []byte(`{"block":{"id":"x","type":"bogus"}}`)}},
			wantErr: "unknown type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := state.Reduce(tt.events)
			if err == nil {
				t.Fatalf("Reduce() error = nil, want substring %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Reduce() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func initMaps(s *state.State) {
	if s.Interactions.Decisions == nil {
		s.Interactions.Decisions = map[string]state.Decision{}
	}
	if s.Interactions.Choices == nil {
		s.Interactions.Choices = map[string]state.Selection{}
	}
	if s.Interactions.Inputs == nil {
		s.Interactions.Inputs = map[string]state.InputValue{}
	}
	if s.Interactions.Feedback == nil {
		s.Interactions.Feedback = map[string][]state.Feedback{}
	}
	if s.Interactions.Replies == nil {
		s.Interactions.Replies = map[string][]state.Reply{}
	}
	if s.Doc == nil {
		s.Doc = &doc.Doc{Version: 1, Blocks: []doc.Block{}}
	}
}

func assertStateEqual(t *testing.T, got, want state.State) {
	t.Helper()
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	var gotAny, wantAny any
	if err := json.Unmarshal(gotJSON, &gotAny); err != nil {
		t.Fatalf("re-unmarshal got: %v", err)
	}
	if err := json.Unmarshal(wantJSON, &wantAny); err != nil {
		t.Fatalf("re-unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(gotAny, wantAny) {
		t.Errorf("state mismatch\n--- got ---\n%s\n--- want ---\n%s", gotJSON, wantJSON)
	}
}
