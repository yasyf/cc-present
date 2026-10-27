package daemon

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
)

const answeredCardDoc = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","title":"Merge?","children":[{"id":"a1","type":"approval"}]},{"id":"m1","type":"markdown","md":"note"}]}`

const editedCardDoc = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","title":"Merge now?","children":[{"id":"a1","type":"approval"}]},{"id":"m1","type":"markdown","md":"note"}]}`

const answeredCard = `{"id":"c1","type":"card","title":"Merge?","children":[{"id":"a1","type":"approval"}]}`

func answer(t *testing.T, h *harness, subjectID, blockID string) {
	t.Helper()
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: subjectID, Origin: ccevent.OriginHuman, Type: EventDecisionCreated,
		Payload: injectIdentity(EventDecisionCreated, json.RawMessage(`{"blockId":"`+blockID+`","verdict":"approved"}`)),
	}); err != nil {
		t.Fatalf("append decision: %v", err)
	}
}

func answeredBoard(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(answeredCardDoc)}), doc.NoPacks, nilDisplay)
	if !start.OK {
		t.Fatalf("start not ok: %s", start.Error)
	}
	answer(t, h, start.SubjectID, "a1")
	return h, start.SubjectID
}

func outcome(t *testing.T, h *harness) result {
	t.Helper()
	reply := handleOutcomes(h.hc(body{}))
	if !reply.OK {
		t.Fatalf("outcomes not ok: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return res
}

func TestAnsweredBlocksNeverComeBack(t *testing.T) {
	t.Run("update-block refuses an answered card until reopened", func(t *testing.T) {
		h, sub := answeredBoard(t)
		reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(answeredCard)}), doc.NoPacks)
		if reply.OK {
			t.Fatal("update-block re-asked an answered card")
		}
		for _, want := range []string{`"c1"`, `"a1"`, "answered at", "--reopen c1"} {
			if !strings.Contains(reply.Error, want) {
				t.Fatalf("refusal %q does not name %q", reply.Error, want)
			}
		}
		if got := h.eventsOfType(t, sub, EventBlockUpserted); len(got) != 0 {
			t.Fatalf("refused update-block appended %d block.upserted", len(got))
		}
		if reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(answeredCard), Reopen: []string{"c1"}}), doc.NoPacks); !reply.OK {
			t.Fatalf("reopened update-block not ok: %s", reply.Error)
		}
	})

	t.Run("push keeps an unchanged answered card but refuses to re-ask it", func(t *testing.T) {
		h, _ := answeredBoard(t)
		if rev := pushRev(t, h, answeredCardDoc); rev != 2 {
			t.Fatalf("unchanged push revision = %d, want 2", rev)
		}
		reply := handlePush(h.hc(body{Doc: json.RawMessage(editedCardDoc)}), doc.NoPacks, nilDisplay)
		if reply.OK || !strings.Contains(reply.Error, "--reopen c1") {
			t.Fatalf("edited push: ok=%v err=%q, want a --reopen c1 refusal", reply.OK, reply.Error)
		}
		if reply := handlePush(h.hc(body{Doc: json.RawMessage(editedCardDoc), Reopen: []string{"a1"}}), doc.NoPacks, nilDisplay); !reply.OK {
			t.Fatalf("reopened push not ok: %s", reply.Error)
		}
	})

	t.Run("remove-block on an answered card retires it from later writes", func(t *testing.T) {
		h, sub := answeredBoard(t)
		if reply := handleRemoveBlock(h.hc(body{ID: "c1"})); !reply.OK {
			t.Fatalf("remove-block not ok: %s", reply.Error)
		}
		if res := outcome(t, h); !slices.Equal(res.Retired, []string{"c1"}) {
			t.Fatalf("retired = %v, want [c1]", res.Retired)
		}
		reply := handlePush(h.hc(body{Doc: json.RawMessage(answeredCardDoc)}), doc.NoPacks, nilDisplay)
		if !reply.OK {
			t.Fatalf("push over a retired card not ok: %s", reply.Error)
		}
		var res result
		if err := json.Unmarshal(reply.Body, &res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `"c1"`) {
			t.Fatalf("warnings = %v, want one naming c1", res.Warnings)
		}
		docs := h.eventsOfType(t, sub, EventDocReplaced)
		if strings.Contains(string(docs[len(docs)-1].Payload), `"c1"`) {
			t.Fatal("the pushed document kept the retired card")
		}
		upsert := handleUpsertBlock(h.hc(body{Block: json.RawMessage(answeredCard)}), doc.NoPacks)
		if !upsert.OK || len(h.eventsOfType(t, sub, EventBlockUpserted)) != 0 {
			t.Fatalf("update-block of a retired card = %+v, want a warning and no write", upsert)
		}
		if reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(answeredCard), Reopen: []string{"c1"}, Round: "current"}), doc.NoPacks); !reply.OK {
			t.Fatalf("reopened update-block not ok: %s", reply.Error)
		}
		if res := outcome(t, h); len(res.Retired) != 0 {
			t.Fatalf("retired after reopen = %v, want none", res.Retired)
		}
	})

	t.Run("removing an unanswered block leaves no tombstone", func(t *testing.T) {
		h, _ := answeredBoard(t)
		if reply := handleRemoveBlock(h.hc(body{ID: "m1"})); !reply.OK {
			t.Fatalf("remove-block not ok: %s", reply.Error)
		}
		if res := outcome(t, h); len(res.Retired) != 0 {
			t.Fatalf("retired = %v, want none", res.Retired)
		}
		if reply := handlePush(h.hc(body{Doc: json.RawMessage(answeredCardDoc), Round: "current"}), doc.NoPacks, nilDisplay); !reply.OK {
			t.Fatalf("push restoring m1 not ok: %s", reply.Error)
		}
	})
}
