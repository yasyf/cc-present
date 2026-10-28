package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	ccevent "github.com/yasyf/cc-interact/event"
)

const snapshotDoc = `{"version":1,"title":"T","blocks":[
  {"id":"k1","type":"card","title":"Rollout","children":[{"id":"ka1","type":"approval","prompt":"Ship it?"},{"id":"km1","type":"markdown","md":"why"}]},
  {"id":"m1","type":"markdown","md":"top"},
  {"id":"p1","type":"example.callout","tone":"warn"}
]}`

func closedRoundHarness(t *testing.T) (*restHarness, int64) {
	t.Helper()
	h := newRestHarnessWith(t, snapshotDoc, emptyPackLoader(t))
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"fb","interaction":{"type":"feedback.created","id":"f1","blockId":"ka1","text":"why now?"}}`); w.Code != http.StatusOK {
		t.Fatalf("feedback: %d %s", w.Code, w.Body.String())
	}
	seq, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: h.id, Origin: ccevent.OriginAgent, Type: EventRoundStarted,
		Payload: injectIdentity(EventRoundStarted, json.RawMessage(`{"title":"Next"}`)),
	})
	if err != nil {
		t.Fatalf("append round.started: %v", err)
	}
	return h, seq
}

func (h *restHarness) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.mux().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestStateSnapshotStubsClosedRounds(t *testing.T) {
	h, seq := closedRoundHarness(t)
	w := h.get(t, "/api/state?session=board--abcd0000")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var got struct {
		Seq      int64 `json:"seq"`
		Revision int   `json:"revision"`
		State    struct {
			Doc    json.RawMessage `json:"doc"`
			Rounds struct {
				Current int                          `json:"current"`
				History []map[string]json.RawMessage `json:"history"`
			} `json:"rounds"`
		} `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Seq != seq || got.Revision != 1 || got.State.Rounds.Current != 2 {
		t.Fatalf("seq=%d revision=%d current=%d, want %d, 1, 2", got.Seq, got.Revision, got.State.Rounds.Current, seq)
	}
	if len(got.State.Rounds.History) != 1 {
		t.Fatalf("history has %d rounds, want 1", len(got.State.Rounds.History))
	}
	stub := got.State.Rounds.History[0]
	if _, ok := stub["blocks"]; ok {
		t.Fatalf("stub carries frozen blocks: %s", stub["blocks"])
	}
	var outline []outlineBlock
	if err := json.Unmarshal(stub["outline"], &outline); err != nil {
		t.Fatalf("decode outline: %v", err)
	}
	want := []outlineBlock{
		{ID: "k1", Type: "card", Label: "Rollout", Children: []string{"ka1", "km1"}},
		{ID: "m1", Type: "markdown", Label: "Markdown"},
		{ID: "p1", Type: "example.callout", Label: "example.callout"},
	}
	if !slices.EqualFunc(outline, want, func(a, b outlineBlock) bool {
		return a.ID == b.ID && a.Type == b.Type && a.Label == b.Label && slices.Equal(a.Children, b.Children)
	}) {
		t.Fatalf("outline = %+v, want %+v", outline, want)
	}
	if string(stub["feedback"]) != `{"ka1":[{"id":"f1","text":"why now?"}]}` {
		t.Fatalf("feedback = %s", stub["feedback"])
	}
}

func TestRoundsServesFrozenBlocks(t *testing.T) {
	h, _ := closedRoundHarness(t)
	w := h.get(t, "/api/rounds?session=board--abcd0000")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var got struct {
		History []struct {
			Number int `json:"number"`
			Blocks []struct {
				ID string `json:"id"`
			} `json:"blocks"`
		} `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.History) != 1 || got.History[0].Number != 1 || len(got.History[0].Blocks) != 3 {
		t.Fatalf("history = %+v, want round 1 with 3 frozen blocks", got.History)
	}
}

func TestSnapshotRoutesRejectAnUnknownSubject(t *testing.T) {
	h := newRestHarness(t)
	for _, path := range []string{"/api/state?session=nope", "/api/rounds?session=nope"} {
		if w := h.get(t, path); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, w.Code)
		}
	}
}

func TestStateServesTheCachedBoardCaughtUpWithNewEvents(t *testing.T) {
	h := newRestHarness(t)
	if w := h.get(t, "/api/state?session=board--abcd0000"); w.Code != http.StatusOK {
		t.Fatalf("first load = %d %s", w.Code, w.Body)
	}
	if _, err := h.cc.DB().Exec(`UPDATE events SET payload='{' WHERE subject_id=? AND seq=1`, h.id); err != nil {
		t.Fatal(err)
	}
	seq, err := appendEvent(t.Context(), h.rs.append, &ccevent.Event{
		SubjectID: h.id, Origin: ccevent.OriginAgent, Type: EventBlockRemoved, Payload: mustJSON(map[string]string{"id": "a1"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if w := h.get(t, "/api/rounds?session=board--abcd0000"); w.Code != http.StatusOK {
		t.Fatalf("rounds = %d %s, want 200 without re-decoding the history the first load reduced", w.Code, w.Body)
	}
	w := h.get(t, "/api/state?session=board--abcd0000")
	if w.Code != http.StatusOK {
		t.Fatalf("second load = %d %s, want 200 without re-decoding the history the first load reduced", w.Code, w.Body)
	}
	var got struct {
		Seq   int64 `json:"seq"`
		State struct {
			Doc json.RawMessage `json:"doc"`
		} `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Seq != seq || strings.Contains(string(got.State.Doc), `"a1"`) {
		t.Fatalf("seq=%d doc=%s, want seq %d without the removed a1", got.Seq, got.State.Doc, seq)
	}
}

func TestWarmBoardsCatchesUpEveryOpenBoardAtBoot(t *testing.T) {
	h := newRestHarness(t)
	if err := warmBoards(t.Context(), h.cc.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.cc.DB().Exec(`UPDATE events SET payload='{' WHERE subject_id=? AND seq=1`, h.id); err != nil {
		t.Fatal(err)
	}
	if w := h.get(t, "/api/state?session=board--abcd0000"); w.Code != http.StatusOK {
		t.Fatalf("first load = %d %s, want 200 from the board warmed at boot", w.Code, w.Body)
	}
}

func TestOutcomeViewsCutTheStateTheirReadersSkip(t *testing.T) {
	h, _ := closedRoundHarness(t)
	board, err := caughtUp(t.Context(), h.cc.DB(), h.id)
	if err != nil {
		t.Fatal(err)
	}
	defer board.mu.Unlock()
	views := map[OutcomeView]struct {
		doc     bool
		history string
	}{
		OutcomeFull:  {doc: true, history: `"blocks":[`},
		OutcomeNoDoc: {doc: false, history: `"blockIds":["k1","m1","p1"]`},
		OutcomeLive:  {doc: true, history: `null`},
	}
	for view, want := range views {
		raw, err := outcomeState(&board.st, view)
		if err != nil {
			t.Fatalf("%q: %v", view, err)
		}
		var got struct {
			Doc    json.RawMessage `json:"doc"`
			Rounds struct {
				History json.RawMessage `json:"history"`
			} `json:"rounds"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%q: %v", view, err)
		}
		if (got.Doc != nil) != want.doc || !strings.Contains(string(got.Rounds.History), want.history) {
			t.Errorf("%q: doc=%s history=%s, want doc %v and history holding %s", view, got.Doc, got.Rounds.History, want.doc, want.history)
		}
		if view == OutcomeNoDoc && strings.Contains(string(got.Rounds.History), `"blocks"`) {
			t.Errorf("no-doc history keeps frozen blocks: %s", got.Rounds.History)
		}
	}
}
