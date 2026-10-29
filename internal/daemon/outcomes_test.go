package daemon

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
)

func appendFrom(t *testing.T, h *harness, subjectID, origin, eventType, payload string) int64 {
	t.Helper()
	seq, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: subjectID, Origin: origin, Type: eventType,
		Payload: injectIdentity(eventType, json.RawMessage(payload)),
	})
	if err != nil {
		t.Fatalf("append %s: %v", eventType, err)
	}
	return seq
}

func readOutcomes(t *testing.T, h *harness, since string) result {
	t.Helper()
	reply := handleOutcomes(h.hc(body{View: OutcomeNoDoc, Since: since}), h.cursors)
	if !reply.OK {
		t.Fatalf("outcomes since %q not ok: %s", since, reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return res
}

func readDelta(t *testing.T, h *harness, since string) Delta {
	t.Helper()
	res := readOutcomes(t, h, since)
	if res.Delta == nil {
		t.Fatalf("outcomes since %q returned the whole state, want a delta", since)
	}
	var d Delta
	if err := json.Unmarshal(res.Delta, &d); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	return d
}

func deltaSeqs(d Delta) []int64 {
	out := make([]int64, len(d.Interactions))
	for i, in := range d.Interactions {
		out[i] = in.Seq
	}
	return out
}

func startApproval(t *testing.T, h *harness) string {
	t.Helper()
	start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
	if !start.OK {
		t.Fatalf("start not ok: %s", start.Error)
	}
	return start.SubjectID
}

func TestOutcomesDefaultIsTheWholeStateUntilTheFirstSubmit(t *testing.T) {
	h := newHarness(t)
	sub := startApproval(t, h)
	appendFrom(t, h, sub, ccevent.OriginHuman, EventDecisionCreated, `{"blockId":"a1","verdict":"approved"}`)

	if res := readOutcomes(t, h, SinceAuto); res.Delta != nil || !strings.Contains(string(res.State), `"approved"`) {
		t.Fatalf("pre-submit default = delta %s state %s, want the whole state", res.Delta, res.State)
	}

	appendFrom(t, h, sub, ccevent.OriginHuman, EventSubmit, `{"revision":0,"viewed":["a1"]}`)
	if got := deltaSeqs(readDelta(t, h, SinceAuto)); len(got) != 2 {
		t.Fatalf("after one submit the default delta = %v, want the decision and the submit", got)
	}
	if res := readOutcomes(t, h, SinceAll); res.Delta != nil || res.State == nil {
		t.Fatalf("--all after a submit = delta %s, want the whole state", res.Delta)
	}
}

func TestOutcomesSinceLastSubmitReadsWhatTheNewestSubmitDelivered(t *testing.T) {
	h := newHarness(t)
	sub := startApproval(t, h)
	appendFrom(t, h, sub, ccevent.OriginHuman, EventDecisionCreated, `{"blockId":"a1","verdict":"approved"}`)
	first := appendFrom(t, h, sub, ccevent.OriginHuman, EventSubmit, `{"revision":0,"viewed":["a1"]}`)
	appendFrom(t, h, sub, ccevent.OriginAgent, EventChoiceSelected, `{"blockId":"q1","optionIds":["a"]}`)
	for _, text := range []string{"1 for", "1 for now", "1 for now, then options"} {
		appendFrom(t, h, sub, ccevent.OriginHuman, EventChoiceSelected, `{"blockId":"q1","optionIds":[],"other":"`+text+`"}`)
	}
	appendFrom(t, h, sub, ccevent.OriginHuman, EventFeedbackCreated, `{"id":"f1","blockId":"a1","text":"one"}`)
	appendFrom(t, h, sub, ccevent.OriginHuman, EventFeedbackCreated, `{"id":"f2","blockId":"a1","text":"two"}`)
	newest := appendFrom(t, h, sub, ccevent.OriginHuman, EventSubmit, `{"revision":0,"viewed":["q1"]}`)
	after := appendFrom(t, h, sub, ccevent.OriginHuman, EventInputSubmitted, `{"blockId":"i1","text":"late"}`)

	for _, since := range []string{SinceAuto, SinceLastSubmit} {
		d := readDelta(t, h, since)
		if d.Since != first || d.Through != after {
			t.Fatalf("%s window = (%d, %d], want (%d, %d]", since, d.Since, d.Through, first, after)
		}
		var types []string
		for _, in := range d.Interactions {
			types = append(types, in.Type+":"+in.BlockID)
		}
		want := []string{EventChoiceSelected + ":q1", EventFeedbackCreated + ":a1", EventFeedbackCreated + ":a1", EventSubmit + ":", EventInputSubmitted + ":i1"}
		if strings.Join(types, " ") != strings.Join(want, " ") {
			t.Fatalf("%s interactions = %v, want %v", since, types, want)
		}
		choice := d.Interactions[0]
		if string(choice.Payload) != `{"optionIds":[],"other":"1 for now, then options"}` {
			t.Fatalf("write-in payload = %s, want only its final text without identity keys", choice.Payload)
		}
		if choice.At.IsZero() || time.Since(choice.At) > time.Minute {
			t.Fatalf("write-in at = %v, want when it landed", choice.At)
		}
		if d.Interactions[3].Seq != newest || !strings.Contains(string(d.Interactions[3].Payload), `"viewed":["q1"]`) {
			t.Fatalf("submit row = %+v, want the newest submit with its viewed blocks", d.Interactions[3])
		}
	}
}

func TestOutcomesSinceASeqOrATime(t *testing.T) {
	h := newHarness(t)
	sub := startApproval(t, h)
	old := appendFrom(t, h, sub, ccevent.OriginHuman, EventFeedbackCreated, `{"id":"f0","blockId":"a1","text":"early"}`)
	appendFrom(t, h, sub, ccevent.OriginHuman, EventDecisionCreated, `{"blockId":"a1","verdict":"rejected"}`)
	pick := appendFrom(t, h, sub, ccevent.OriginHuman, EventChoiceSelected, `{"blockId":"q1","optionIds":["b"]}`)
	last := appendFrom(t, h, sub, ccevent.OriginHuman, EventDecisionCreated, `{"blockId":"a1","verdict":"approved"}`)
	if _, err := h.cc.DB().Exec(`UPDATE events SET created_at=? WHERE subject_id=? AND seq<=?`, time.Date(2020, 1, 1, 15, 0, 0, 0, time.UTC).Unix(), sub, old); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	want := func(name string, got []int64, seqs ...int64) {
		t.Helper()
		if len(got) != len(seqs) {
			t.Fatalf("%s = %v, want %v", name, got, seqs)
		}
		for i := range seqs {
			if got[i] != seqs[i] {
				t.Fatalf("%s = %v, want %v", name, got, seqs)
			}
		}
	}

	want("since 0", deltaSeqs(readDelta(t, h, "0")), old, pick, last)
	want("since the note's seq", deltaSeqs(readDelta(t, h, strconv.FormatInt(old, 10))), pick, last)
	want("since a second after the note", deltaSeqs(readDelta(t, h, "2020-01-01T15:00:01Z")), pick, last)
	want("since the note's instant at -08:00", deltaSeqs(readDelta(t, h, "2020-01-01T07:00:00-08:00")), old, pick, last)
	if reply := handleOutcomes(h.hc(body{Since: "yesterday"}), h.cursors); reply.OK || !strings.Contains(reply.Error, `"yesterday"`) {
		t.Fatalf("since yesterday: ok=%v err=%q, want an error naming the value", reply.OK, reply.Error)
	}
}

func TestOutcomesNewDrainsFromThePreviousRead(t *testing.T) {
	h := newHarness(t)
	sub := startApproval(t, h)
	first := appendFrom(t, h, sub, ccevent.OriginHuman, EventDecisionCreated, `{"blockId":"a1","verdict":"approved"}`)

	if got := deltaSeqs(readDelta(t, h, SinceNew)); len(got) != 1 || got[0] != first {
		t.Fatalf("first --new = %v, want everything", got)
	}
	if got := readDelta(t, h, SinceNew); len(got.Interactions) != 0 {
		t.Fatalf("--new with nothing new = %v, want none", deltaSeqs(got))
	}

	second := appendFrom(t, h, sub, ccevent.OriginHuman, EventFeedbackCreated, `{"id":"f1","blockId":"a1","text":"why"}`)
	if reply := handleOutcomes(h.hc(body{View: OutcomeLive}), h.cursors); !reply.OK {
		t.Fatalf("internal read: %s", reply.Error)
	}
	if got := deltaSeqs(readDelta(t, h, SinceNew)); len(got) != 1 || got[0] != second {
		t.Fatalf("--new after an internal read = %v, want the feedback the internal read left in place", got)
	}

	third := appendFrom(t, h, sub, ccevent.OriginHuman, EventInputSubmitted, `{"blockId":"i1","text":"x"}`)
	readOutcomes(t, h, SinceAll)
	appendFrom(t, h, sub, ccevent.OriginHuman, EventInputSubmitted, `{"blockId":"i2","text":"y"}`)
	if got := deltaSeqs(readDelta(t, h, SinceNew)); len(got) != 1 || got[0] == third {
		t.Fatalf("--new after an --all read = %v, want only the input after it", got)
	}
}
