package state_test

import (
	"reflect"
	"testing"

	"github.com/yasyf/cc-present/internal/state"
)

func TestRemovedAnswersOutliveTheirBlocks(t *testing.T) {
	card := func(id, title, child string) string {
		return `{"id":"` + id + `","type":"card","title":"` + title + `","children":[` + child + `]}`
	}
	route := card("c1", "Route?", `{"id":"q1","type":"choice","prompt":"Which?","options":[{"id":"a","label":"A"}]}`)
	ship := card("c2", "Ship it?", `{"id":"q2","type":"approval","prompt":"Ship?"}`)
	ev := func(seq int64, origin, typ, body string) state.Event {
		return state.Event{Origin: origin, Type: typ, Seq: seq, Payload: eventPayload(t, typ, body)}
	}
	events := []state.Event{
		ev(1, "agent", "doc.replaced", `{"revision":1,"doc":{"version":1,"title":"T","blocks":[`+route+`,`+ship+`]}}`),
		ev(2, "human", "choice.selected", `{"blockId":"q1","optionIds":[],"other":"same layer"}`),
		ev(3, "human", "decision.created", `{"blockId":"q2","verdict":"approved"}`),
		ev(4, "human", "submit", `{"revision":1}`),
		ev(5, "agent", "block.removed", `{"id":"c1"}`),
		ev(6, "agent", "doc.replaced", `{"revision":2,"doc":{"version":1,"title":"T","blocks":[{"id":"m1","type":"markdown","md":"next"}]}}`),
	}
	st, err := state.Reduce(events)
	if err != nil {
		t.Fatalf("Reduce() error = %v", err)
	}
	want := map[string]state.Removed{
		"q1": {Top: "c1", Title: "Route?", Round: 2},
		"q2": {Top: "c2", Title: "Ship it?", Round: 2},
	}
	if got := st.RemovedAnswers(); !reflect.DeepEqual(got, want) {
		t.Fatalf("RemovedAnswers() = %v, want %v", got, want)
	}
	for seq, round := range map[int64]int{2: 1, 4: 1, 5: 2} {
		if got := st.RoundStarts().At(seq); got != round {
			t.Fatalf("RoundStarts().At(%d) = %d, want %d", seq, got, round)
		}
	}

	if err := st.Apply([]state.Event{ev(7, "agent", "block.upserted", `{"block":`+route+`}`)}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	delete(want, "q1")
	if got := st.RemovedAnswers(); !reflect.DeepEqual(got, want) {
		t.Fatalf("RemovedAnswers() after c1 came back = %v, want %v", got, want)
	}
}
