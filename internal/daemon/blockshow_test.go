package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
)

const routeDoc = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","title":"Route?","summary":"Pick a layer.","children":[{"id":"q1","type":"choice","prompt":"Which?","options":[{"id":"a","label":"A","recommended":true},{"id":"b","label":"B"}]}]},{"id":"m1","type":"markdown","md":"note"}]}`

func blockShow(t *testing.T, h *harness, id string) BlockShow {
	t.Helper()
	reply := handleBlockShow(h.hc(body{ID: id}))
	if !reply.OK {
		t.Fatalf("block-show %s not ok: %s", id, reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	var show BlockShow
	if err := json.Unmarshal(res.State, &show); err != nil {
		t.Fatalf("decode block show: %v", err)
	}
	return show
}

func TestBlockShowReadsARemovedChildFromTheLog(t *testing.T) {
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(routeDoc)}), doc.NoPacks, nilDisplay)
	if !start.OK {
		t.Fatalf("start not ok: %s", start.Error)
	}
	child := `{"id":"q1","type":"choice","prompt":"Which route?","options":[{"id":"a","label":"A","recommended":true},{"id":"b","label":"B"}]}`
	if reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(child)}), doc.NoPacks); !reply.OK {
		t.Fatalf("update-block not ok: %s", reply.Error)
	}
	for _, p := range []string{`{"blockId":"q1","optionIds":[],"other":"same layer"}`, `{"blockId":"q1","optionIds":["b"]}`} {
		if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
			SubjectID: start.SubjectID, Origin: ccevent.OriginHuman, Type: EventChoiceSelected,
			Payload: injectIdentity(EventChoiceSelected, json.RawMessage(p)),
		}); err != nil {
			t.Fatalf("append choice: %v", err)
		}
	}
	if show := blockShow(t, h, "q1"); !show.OnBoard {
		t.Fatal("q1 on the board reads as removed")
	}
	ageEvents(t, h, start.SubjectID)
	if reply := handleRemoveBlock(h.hc(body{ID: "c1"})); !reply.OK {
		t.Fatalf("remove-block not ok: %s", reply.Error)
	}

	show := blockShow(t, h, "q1")
	if show.OnBoard {
		t.Fatal("removed q1 reads as on the board")
	}
	top, err := doc.DecodeStoredBlock(show.Top)
	if err != nil {
		t.Fatalf("decode top: %v", err)
	}
	card, ok := top.(*doc.Card)
	if !ok || card.Title != "Route?" || card.Children[0].(*doc.Choice).Prompt != "Which route?" {
		t.Fatalf("top = %s, want card c1 holding the updated q1", show.Top)
	}
	if len(show.Events) != 2 || !strings.Contains(string(show.Events[0].Payload), `"b"`) || !strings.Contains(string(show.Events[1].Payload), "same layer") {
		t.Fatalf("events = %+v, want both picks newest first", show.Events)
	}
	if got := blockShow(t, h, "c1"); len(got.Events) != 2 {
		t.Fatalf("card events = %d, want its child's 2", len(got.Events))
	}
	if reply := handleBlockShow(h.hc(body{ID: "zz"})); reply.OK || !strings.Contains(reply.Error, `"zz"`) {
		t.Fatalf("unknown block: ok=%v err=%q", reply.OK, reply.Error)
	}
}
