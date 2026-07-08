package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
	ccstore "github.com/yasyf/cc-interact/store"
	"github.com/yasyf/cc-interact/subject"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/state"
)

// harness assembles the pieces the cc-interact daemon would, so an artifact op
// can be driven directly and asserted through a Reply, on a real ephemeral store.
type harness struct {
	cc       *ccstore.Store
	resolver subject.Resolver
	activity *ccd.Activity
	assets   *assets.Store
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cc, err := ccstore.Open(filepath.Join(t.TempDir(), "t.db"), nil)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	ast, err := assets.New(filepath.Join(t.TempDir(), "assets"))
	if err != nil {
		t.Fatalf("new assets: %v", err)
	}
	return &harness{
		cc:       cc,
		activity: ccd.NewActivity(),
		assets:   ast,
		resolver: subject.Resolver{
			Store:  ccstore.NewSubjectStore(cc.DB()),
			Policy: subject.Policy{Active: func(s subject.Subject) bool { return s.Status == statusOpen }},
		},
	}
}

func (h *harness) hc(b body) ccd.HandlerCtx {
	raw, _ := json.Marshal(b)
	return ccd.HandlerCtx{
		Ctx:      context.Background(),
		Env:      ccd.Envelope{Session: "s1", ClaudePID: 100, Scope: "/repo", Body: raw},
		Window:   subject.Window{Session: "s1", ClaudePID: 100},
		Scope:    "/repo",
		Subjects: h.resolver,
		DB:       h.cc.DB(),
		Append:   h.cc.AppendEvent,
		HTTPPort: 8080,
		Activity: h.activity,
	}
}

func (h *harness) eventsOfType(t *testing.T, subjectID, typ string) []ccevent.Event {
	t.Helper()
	all, err := h.cc.EventsSince(context.Background(), subjectID, 0, "")
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	var out []ccevent.Event
	for _, e := range all {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

const approvalDoc = `{"version":1,"title":"Board","blocks":[{"id":"a1","type":"approval"}]}`

func TestStart(t *testing.T) {
	t.Run("fresh start reports url and inactive channel", func(t *testing.T) {
		h := newHarness(t)
		reply := handleStart(h.hc(body{Title: "My Board"}))
		if !reply.OK {
			t.Fatalf("start not ok: %s", reply.Error)
		}
		if reply.SubjectID == "" {
			t.Fatal("start returned no subject id")
		}
		var res result
		if err := json.Unmarshal(reply.Body, &res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		if !strings.Contains(res.URL, "/p/my-board--") {
			t.Fatalf("url = %q, want /p/my-board--<hash>", res.URL)
		}
		if res.ChannelState != "inactive" {
			t.Fatalf("channel = %q, want inactive", res.ChannelState)
		}
		if got := h.eventsOfType(t, reply.SubjectID, EventDocReplaced); len(got) != 0 {
			t.Fatalf("no-doc start appended %d doc.replaced, want 0", len(got))
		}
	})

	t.Run("start with a doc appends revision 1", func(t *testing.T) {
		h := newHarness(t)
		reply := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
		if !reply.OK {
			t.Fatalf("start not ok: %s", reply.Error)
		}
		docs := h.eventsOfType(t, reply.SubjectID, EventDocReplaced)
		if len(docs) != 1 {
			t.Fatalf("got %d doc.replaced, want 1", len(docs))
		}
		if rev := revisionOf(t, docs[0]); rev != 1 {
			t.Fatalf("initial revision = %d, want 1", rev)
		}
	})

	t.Run("start after close creates a fresh subject", func(t *testing.T) {
		h := newHarness(t)
		first := handleStart(h.hc(body{Title: "One"}))
		if reply := handleClose(h.hc(body{}), h.assets); !reply.OK {
			t.Fatalf("close not ok: %s", reply.Error)
		}
		second := handleStart(h.hc(body{Title: "One"}))
		if !second.OK {
			t.Fatalf("second start not ok: %s", second.Error)
		}
		if second.SubjectID == first.SubjectID {
			t.Fatal("start after close resumed the closed subject instead of creating fresh")
		}
	})
}

func TestPush(t *testing.T) {
	t.Run("push increments revision", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Title: "T"}))
		if rev := pushRev(t, h, approvalDoc); rev != 1 {
			t.Fatalf("first push revision = %d, want 1", rev)
		}
		if rev := pushRev(t, h, approvalDoc); rev != 2 {
			t.Fatalf("second push revision = %d, want 2", rev)
		}
		if got := h.eventsOfType(t, start.SubjectID, EventDocReplaced); len(got) != 2 {
			t.Fatalf("got %d doc.replaced, want 2", len(got))
		}
	})

	t.Run("push without a subject is rejected", func(t *testing.T) {
		h := newHarness(t)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
		if reply.OK || !strings.Contains(reply.Error, "no cc-present artifact") {
			t.Fatalf("push without subject: ok=%v err=%q", reply.OK, reply.Error)
		}
	})

	t.Run("push to a closed artifact is rejected", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Title: "T"}))
		handleClose(h.hc(body{}), h.assets)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
		if reply.OK || !strings.Contains(reply.Error, "closed") {
			t.Fatalf("push when closed: ok=%v err=%q", reply.OK, reply.Error)
		}
	})

	t.Run("push rejects an invalid document", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Title: "T"}))
		reply := handlePush(h.hc(body{Doc: json.RawMessage(`{"version":1,"title":"","blocks":[]}`)}))
		if reply.OK || !strings.Contains(reply.Error, "title") {
			t.Fatalf("push invalid doc: ok=%v err=%q", reply.OK, reply.Error)
		}
	})
}

func TestUpsertBlock(t *testing.T) {
	tests := []struct {
		name    string
		block   string
		wantErr string
	}{
		{"valid markdown", `{"id":"m1","type":"markdown","md":"hi"}`, ""},
		{"unknown type", `{"id":"x","type":"bogus"}`, "unknown type"},
		{"missing required field", `{"id":"m2","type":"markdown","md":""}`, "md must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			start := handleStart(h.hc(body{Title: "T"}))
			reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(tt.block)}))
			if tt.wantErr == "" {
				if !reply.OK {
					t.Fatalf("upsert not ok: %s", reply.Error)
				}
				if got := h.eventsOfType(t, start.SubjectID, EventBlockUpserted); len(got) != 1 {
					t.Fatalf("got %d block.upserted, want 1", len(got))
				}
				return
			}
			if reply.OK || !strings.Contains(reply.Error, tt.wantErr) {
				t.Fatalf("upsert %s: ok=%v err=%q, want %q", tt.name, reply.OK, reply.Error, tt.wantErr)
			}
		})
	}
}

func TestClose(t *testing.T) {
	h := newHarness(t)
	start := handleStart(h.hc(body{Title: "T"}))
	if reply := handleClose(h.hc(body{Summary: "done"}), h.assets); !reply.OK {
		t.Fatalf("close not ok: %s", reply.Error)
	}
	if got := h.eventsOfType(t, start.SubjectID, EventPresentClosed); len(got) != 1 {
		t.Fatalf("got %d present.closed, want 1", len(got))
	}
	sub, err := h.resolver.Store.Get(context.Background(), start.SubjectID)
	if err != nil {
		t.Fatalf("get subject: %v", err)
	}
	if sub.Status != statusClosed {
		t.Fatalf("status = %q, want closed", sub.Status)
	}
	if reply := handleClose(h.hc(body{}), h.assets); reply.OK || !strings.Contains(reply.Error, "already closed") {
		t.Fatalf("re-close: ok=%v err=%q", reply.OK, reply.Error)
	}
}

func TestReply(t *testing.T) {
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
	if reply := handleReply(h.hc(body{BlockID: "a1", Md: "answered"})); !reply.OK {
		t.Fatalf("reply not ok: %s", reply.Error)
	}
	got := h.eventsOfType(t, start.SubjectID, EventReplyCreated)
	if len(got) != 1 {
		t.Fatalf("got %d reply.created, want 1", len(got))
	}
	var p struct{ BlockID, Md string }
	if err := json.Unmarshal(got[0].Payload, &p); err != nil {
		t.Fatalf("decode reply payload: %v", err)
	}
	if p.BlockID != "a1" || p.Md != "answered" {
		t.Fatalf("reply payload = %+v", p)
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, h *harness)
		title     string
		wantErr   string
		wantRound int
	}{
		{
			name: "advance on a dirty round",
			setup: func(t *testing.T, h *harness) {
				if r := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)})); !r.OK {
					t.Fatalf("start not ok: %s", r.Error)
				}
			},
			wantRound: 2,
		},
		{
			name: "title-only on a clean round after submit",
			setup: func(t *testing.T, h *harness) {
				start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
				// A submit on the dirty round closes it and advances to round 2; the
				// live block keeps its round-1 stamp, so round 2 is clean.
				if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
					SubjectID: start.SubjectID, Origin: ccevent.OriginHuman, Type: EventSubmit,
					Payload: json.RawMessage(`{"revision":1}`),
				}); err != nil {
					t.Fatalf("append submit: %v", err)
				}
			},
			title:     "Round Two",
			wantRound: 2,
		},
		{
			name: "rejected on a closed artifact",
			setup: func(_ *testing.T, h *harness) {
				handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
				handleClose(h.hc(body{}), h.assets)
			},
			wantErr: "closed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			tt.setup(t, h)
			reply := handleRound(h.hc(body{Title: tt.title}))
			if tt.wantErr != "" {
				if reply.OK || !strings.Contains(reply.Error, tt.wantErr) {
					t.Fatalf("round: ok=%v err=%q, want %q", reply.OK, reply.Error, tt.wantErr)
				}
				return
			}
			if !reply.OK {
				t.Fatalf("round not ok: %s", reply.Error)
			}
			var res result
			if err := json.Unmarshal(reply.Body, &res); err != nil {
				t.Fatalf("decode result: %v", err)
			}
			if res.Round != tt.wantRound {
				t.Fatalf("round = %d, want %d", res.Round, tt.wantRound)
			}
		})
	}
}

func TestOutcomes(t *testing.T) {
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}))
	// A human decision lands directly on the log, as the REST plane would append it.
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: start.SubjectID, Origin: ccevent.OriginHuman, Type: EventDecisionCreated,
		Payload: json.RawMessage(`{"blockId":"a1","verdict":"approved"}`),
	}); err != nil {
		t.Fatalf("append decision: %v", err)
	}
	reply := handleOutcomes(h.hc(body{}))
	if !reply.OK {
		t.Fatalf("outcomes not ok: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	var st state.State
	if err := json.Unmarshal(res.State, &st); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if d, ok := st.Interactions.Decisions["a1"]; !ok || d.Verdict != "approved" {
		t.Fatalf("decisions = %+v, want a1 approved", st.Interactions.Decisions)
	}
	if st.Doc.Title != "Board" {
		t.Fatalf("reduced doc title = %q, want Board", st.Doc.Title)
	}
}

func pushRev(t *testing.T, h *harness, docJSON string) int {
	t.Helper()
	reply := handlePush(h.hc(body{Doc: json.RawMessage(docJSON)}))
	if !reply.OK {
		t.Fatalf("push not ok: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode push result: %v", err)
	}
	return res.Revision
}

func revisionOf(t *testing.T, e ccevent.Event) int {
	t.Helper()
	var p struct {
		Revision int `json:"revision"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		t.Fatalf("decode doc.replaced payload: %v", err)
	}
	return p.Revision
}
