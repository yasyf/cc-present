package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
)

func ageEvents(t *testing.T, h *harness, subjectID string) {
	t.Helper()
	if _, err := h.cc.DB().Exec(`UPDATE events SET created_at=created_at-? WHERE subject_id=?`, int64(2*touchHold/time.Second), subjectID); err != nil {
		t.Fatalf("age events: %v", err)
	}
}

func touchedBoard(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(answeredCardDoc)}), doc.NoPacks, nilDisplay)
	if !start.OK {
		t.Fatalf("start not ok: %s", start.Error)
	}
	fileNote(t, h, start.SubjectID, "a1", "f1")
	return h, start.SubjectID
}

func wantHeld(t *testing.T, reply ccd.Reply, op, block string) {
	t.Helper()
	if reply.OK {
		t.Fatalf("%s of a block the human just touched went through", op)
	}
	for _, want := range []string{`"` + block + `"`, `"a1"`, EventFeedbackCreated, "retry after", "submits"} {
		if !strings.Contains(reply.Error, want) {
			t.Fatalf("%s refusal %q does not name %q", op, reply.Error, want)
		}
	}
}

func TestABlockTheHumanTouchedStaysUntilTheHoldPassesOrTheRoundIsSubmitted(t *testing.T) {
	t.Run("remove-block refuses until the hold passes", func(t *testing.T) {
		h, sub := touchedBoard(t)
		wantHeld(t, handleRemoveBlock(h.hc(body{ID: "c1"})), "remove-block", "c1")
		if got := h.eventsOfType(t, sub, EventBlockRemoved); len(got) != 0 {
			t.Fatalf("refused remove-block appended %d block.removed", len(got))
		}
		if reply := handleRemoveBlock(h.hc(body{ID: "m1"})); !reply.OK {
			t.Fatalf("remove-block of an untouched block not ok: %s", reply.Error)
		}
		ageEvents(t, h, sub)
		if reply := handleRemoveBlock(h.hc(body{ID: "c1"})); !reply.OK {
			t.Fatalf("remove-block after the hold not ok: %s", reply.Error)
		}
	})

	t.Run("a submit after the touch releases the block", func(t *testing.T) {
		h, sub := touchedBoard(t)
		submitRevision(t, h, sub, 1)
		if reply := handleRemoveBlock(h.hc(body{ID: "c1"})); !reply.OK {
			t.Fatalf("remove-block after the submit not ok: %s", reply.Error)
		}
	})

	t.Run("update-block keeps an identical block and refuses a changed one, even reopened", func(t *testing.T) {
		h, sub := touchedBoard(t)
		if reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(answeredCard)}), doc.NoPacks); !reply.OK {
			t.Fatalf("identical update-block not ok: %s", reply.Error)
		}
		edited := strings.Replace(answeredCard, "Merge?", "Merge now?", 1)
		wantHeld(t, handleUpsertBlock(h.hc(body{Block: json.RawMessage(edited)}), doc.NoPacks), "update-block", "c1")
		wantHeld(t, handleUpsertBlock(h.hc(body{Block: json.RawMessage(edited), Reopen: []string{"c1"}}), doc.NoPacks), "reopened update-block", "c1")
		wantHeld(t, handleUpsertBlock(h.hc(body{Block: json.RawMessage(`{"id":"a1","type":"approval","prompt":"Ship?"}`)}), doc.NoPacks), "child update-block", "a1")
		if got := h.eventsOfType(t, sub, EventBlockUpserted); len(got) != 1 {
			t.Fatalf("block.upserted events = %d, want only the identical one", len(got))
		}
	})

	t.Run("push refuses to drop or change a touched block, even with an owner re-ask", func(t *testing.T) {
		h, _ := touchedBoard(t)
		if rev := pushRev(t, h, answeredCardDoc); rev != 2 {
			t.Fatalf("unchanged push revision = %d, want 2", rev)
		}
		reask := Reask{PRs: []string{"8"}, Reason: "the owner asked"}
		wantHeld(t, handlePush(h.hc(body{Doc: json.RawMessage(editedCardDoc), Reask: reask}), doc.NoPacks, nilDisplay), "changing push", "c1")
		dropped := `{"version":1,"title":"Board","blocks":[{"id":"m1","type":"markdown","md":"note"}]}`
		reply := handlePush(h.hc(body{Doc: json.RawMessage(dropped)}), doc.NoPacks, nilDisplay)
		wantHeld(t, reply, "dropping push", "c1")
		if !strings.Contains(reply.Error, "drop") {
			t.Fatalf("dropping push refusal %q does not say drop", reply.Error)
		}
	})

	t.Run("a pick on a pull-request row holds the block from removal, not from a redraw", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Doc: json.RawMessage(prGroupsDoc)}), doc.NoPacks, nilDisplay)
		if !start.OK {
			t.Fatalf("start not ok: %s", start.Error)
		}
		if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
			SubjectID: start.SubjectID, Origin: ccevent.OriginHuman, Type: EventTriageDecided,
			Payload: injectIdentity(EventTriageDecided, json.RawMessage(`{"blockId":"pg","verdicts":{"8":{"verdict":"approved"}}}`)),
		}); err != nil {
			t.Fatalf("append triage: %v", err)
		}
		if rev := pushRev(t, h, strings.Replace(prGroupsDoc, `"title":"G",`, `"title":"G2",`, 1)); rev != 2 {
			t.Fatalf("redraw push revision = %d, want 2", rev)
		}
		reply := handleRemoveBlock(h.hc(body{ID: "pg"}))
		if reply.OK || !strings.Contains(reply.Error, EventTriageDecided) {
			t.Fatalf("remove-block of a picked pr-groups block: ok=%v err=%q, want a hold naming the pick", reply.OK, reply.Error)
		}
	})
}
