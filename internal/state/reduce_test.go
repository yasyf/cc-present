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

// TestReducePackInteraction covers the pack.interaction reducer case with inline
// events; the language-neutral testdata fixtures for it land in Phase 3 lockstep
// with the TypeScript reducer.
func TestReducePackInteraction(t *testing.T) {
	const packDoc = `{"doc":{"version":1,"title":"T","blocks":[{"id":"r1","type":"example.rating","value":3}]},"revision":1}`

	t.Run("last write wins per block", func(t *testing.T) {
		st, err := state.Reduce([]state.Event{
			{Origin: "agent", Type: "doc.replaced", Seq: 1, Payload: []byte(packDoc)},
			{Origin: "human", Type: "pack.interaction", Seq: 2, Payload: []byte(`{"blockId":"r1","payload":{"value":3}}`)},
			{Origin: "human", Type: "pack.interaction", Seq: 3, Payload: []byte(`{"blockId":"r1","payload":{"value":5}}`)},
		})
		if err != nil {
			t.Fatalf("Reduce: %v", err)
		}
		if got := packValue(t, st.Interactions.Packs["r1"]); got != 5 {
			t.Fatalf("packs[r1].value = %d, want 5 (last write wins)", got)
		}
	})

	t.Run("snapshot into the closed round on submit", func(t *testing.T) {
		st, err := state.Reduce([]state.Event{
			{Origin: "agent", Type: "doc.replaced", Seq: 1, Payload: []byte(packDoc)},
			{Origin: "human", Type: "pack.interaction", Seq: 2, Payload: []byte(`{"blockId":"r1","payload":{"value":4}}`)},
			{Origin: "human", Type: "submit", Seq: 3, Payload: []byte(`{"revision":1}`)},
		})
		if err != nil {
			t.Fatalf("Reduce: %v", err)
		}
		if len(st.Rounds.History) != 1 {
			t.Fatalf("history = %d rounds, want 1", len(st.Rounds.History))
		}
		if got := packValue(t, st.Rounds.History[0].Packs["r1"]); got != 4 {
			t.Fatalf("round packs[r1].value = %d, want 4", got)
		}
		// The live interaction persists past the snapshot, mirroring choices/decisions.
		if got := packValue(t, st.Interactions.Packs["r1"]); got != 4 {
			t.Fatalf("live packs[r1].value = %d, want 4", got)
		}
	})
}

func packValue(t *testing.T, v state.PackValue) int {
	t.Helper()
	var p struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(v.Payload, &p); err != nil {
		t.Fatalf("decode pack payload %q: %v", v.Payload, err)
	}
	return p.Value
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
	if s.Interactions.Packs == nil {
		s.Interactions.Packs = map[string]state.PackValue{}
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
	if s.Rounds.Current == 0 {
		s.Rounds.Current = 1
	}
	if s.Rounds.BlockRounds == nil {
		s.Rounds.BlockRounds = map[string]int{}
	}
	if s.Rounds.History == nil {
		s.Rounds.History = []state.RoundRecord{}
	}
	if s.Revising.BlockIDs == nil {
		s.Revising.BlockIDs = []string{}
	}
	for i := range s.Rounds.History {
		if s.Rounds.History[i].Packs == nil {
			s.Rounds.History[i].Packs = map[string]state.PackValue{}
		}
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
