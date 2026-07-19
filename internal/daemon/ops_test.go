package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
	ccstore "github.com/yasyf/cc-interact/store"
	"github.com/yasyf/cc-interact/subject"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
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
		Env:      ccd.Envelope{Session: "s1", ClaudePID: 100, Scope: scopeSentinel, Body: raw},
		Window:   subject.Window{Session: "s1", ClaudePID: 100},
		Scope:    scopeSentinel,
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

// cardDoc nests an input inside a card and keeps a non-interactive markdown at
// the top level, so a reply can target a card child and a leaf block alike.
const cardDoc = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","children":[{"id":"in1","type":"input","label":"Name"}]},{"id":"m1","type":"markdown","md":"note"}]}`

// nilDisplay is the no-trust display func the handler tests pass by default: a
// start or push then carries no tailnet URLs.
func nilDisplay(context.Context, string, int) []string { return nil }

// fakeDisplay stands in for the mesh-trust display closure, echoing its slug so
// a test can assert the URLs reached the reply.
func fakeDisplay(_ context.Context, slug string, _ int) []string {
	return []string{"https://host.ts.net:8080/p/" + slug}
}

func TestStart(t *testing.T) {
	t.Run("fresh start reports url and inactive channel", func(t *testing.T) {
		h := newHarness(t)
		reply := handleStart(h.hc(body{Title: "My Board"}), doc.NoPacks, nilDisplay)
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
		reply := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
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
		first := handleStart(h.hc(body{Title: "One"}), doc.NoPacks, nilDisplay)
		if reply := handleClose(h.hc(body{}), h.assets); !reply.OK {
			t.Fatalf("close not ok: %s", reply.Error)
		}
		second := handleStart(h.hc(body{Title: "One"}), doc.NoPacks, nilDisplay)
		if !second.OK {
			t.Fatalf("second start not ok: %s", second.Error)
		}
		if second.SubjectID == first.SubjectID {
			t.Fatal("start after close resumed the closed subject instead of creating fresh")
		}
	})

	t.Run("mesh trust surfaces tailnet urls in the reply", func(t *testing.T) {
		h := newHarness(t)
		reply := handleStart(h.hc(body{Title: "Mesh"}), doc.NoPacks, fakeDisplay)
		if !reply.OK {
			t.Fatalf("start not ok: %s", reply.Error)
		}
		var res result
		if err := json.Unmarshal(reply.Body, &res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		want := []string{"https://host.ts.net:8080/p/" + res.Slug}
		if !slices.Equal(res.TailnetURLs, want) {
			t.Fatalf("tailnetUrls = %v, want %v", res.TailnetURLs, want)
		}
	})
}

func TestPush(t *testing.T) {
	t.Run("push increments revision", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Title: "T"}), doc.NoPacks, nilDisplay)
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

	t.Run("push surfaces the loopback and tailnet urls", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Title: "T"}), doc.NoPacks, nilDisplay)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, fakeDisplay)
		if !reply.OK {
			t.Fatalf("push not ok: %s", reply.Error)
		}
		var res result
		if err := json.Unmarshal(reply.Body, &res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		const prefix = "http://127.0.0.1:8080/p/"
		if !strings.HasPrefix(res.URL, prefix) {
			t.Fatalf("url = %q, want prefix %q", res.URL, prefix)
		}
		want := []string{"https://host.ts.net:8080/p/" + strings.TrimPrefix(res.URL, prefix)}
		if !slices.Equal(res.TailnetURLs, want) {
			t.Fatalf("tailnetUrls = %v, want %v", res.TailnetURLs, want)
		}
	})

	t.Run("push without a subject is rejected", func(t *testing.T) {
		h := newHarness(t)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		if reply.OK || !strings.Contains(reply.Error, "no cc-present artifact") {
			t.Fatalf("push without subject: ok=%v err=%q", reply.OK, reply.Error)
		}
	})

	t.Run("push to a closed artifact is rejected", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Title: "T"}), doc.NoPacks, nilDisplay)
		handleClose(h.hc(body{}), h.assets)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		if reply.OK || !strings.Contains(reply.Error, "closed") {
			t.Fatalf("push when closed: ok=%v err=%q", reply.OK, reply.Error)
		}
	})

	t.Run("push rejects an invalid document", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Title: "T"}), doc.NoPacks, nilDisplay)
		reply := handlePush(h.hc(body{Doc: json.RawMessage(`{"version":1,"title":"","blocks":[]}`)}), doc.NoPacks, nilDisplay)
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
			start := handleStart(h.hc(body{Title: "T"}), doc.NoPacks, nilDisplay)
			reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(tt.block)}), doc.NoPacks)
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

func TestUpsertBlockDocValidation(t *testing.T) {
	t.Run("option visual id colliding with an existing block id is rejected", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		// ch1's option visual reuses "a1", the existing top-level approval's id; the
		// upserted document would carry two "a1"s, which the whole-doc check rejects.
		dup := `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"a1","type":"code","lang":"go","code":"x"}}]}`
		reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(dup)}), doc.NoPacks)
		if reply.OK || !strings.Contains(reply.Error, "duplicate block id") {
			t.Fatalf("dup visual id: ok=%v err=%q, want a duplicate-id rejection", reply.OK, reply.Error)
		}
		if got := h.eventsOfType(t, start.SubjectID, EventBlockUpserted); len(got) != 0 {
			t.Fatalf("rejected upsert appended %d block.upserted, want 0", len(got))
		}
	})

	t.Run("upsert growing the doc past the size cap is rejected", func(t *testing.T) {
		h := newHarness(t)
		big := strings.Repeat("x", (doc.MaxDocBytes*3)/4)
		seed := `{"version":1,"title":"Board","blocks":[{"id":"m1","type":"markdown","md":"` + big + `"}]}`
		start := handleStart(h.hc(body{Doc: json.RawMessage(seed)}), doc.NoPacks, nilDisplay)
		if !start.OK {
			t.Fatalf("seed start not ok: %s", start.Error)
		}
		block := `{"id":"m2","type":"markdown","md":"` + big + `"}`
		reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(block)}), doc.NoPacks)
		if reply.OK || !strings.Contains(reply.Error, "exceeds") {
			t.Fatalf("oversized upsert: ok=%v err=%q, want a size rejection", reply.OK, reply.Error)
		}
		if got := h.eventsOfType(t, start.SubjectID, EventBlockUpserted); len(got) != 0 {
			t.Fatalf("rejected upsert appended %d block.upserted, want 0", len(got))
		}
	})

	t.Run("same-id replacement upsert still passes", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(`{"id":"a1","type":"markdown","md":"replaced"}`)}), doc.NoPacks)
		if !reply.OK {
			t.Fatalf("same-id replacement upsert not ok: %s", reply.Error)
		}
		if got := h.eventsOfType(t, start.SubjectID, EventBlockUpserted); len(got) != 1 {
			t.Fatalf("got %d block.upserted, want 1", len(got))
		}
	})
}

func TestUpsertBlockPack(t *testing.T) {
	reg := packLoader(t, writePackTree(t)).Current()
	tests := []struct {
		name    string
		block   string
		wantErr string
	}{
		{"declared pack block", `{"id":"c1","type":"example.callout","tone":"warn"}`, ""},
		{"undeclared dotted type", `{"id":"g1","type":"ghost.thing"}`, "not installed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			handleStart(h.hc(body{Title: "T"}), reg, nilDisplay)
			reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(tt.block)}), reg)
			if tt.wantErr == "" {
				if !reply.OK {
					t.Fatalf("upsert not ok: %s", reply.Error)
				}
				return
			}
			if reply.OK || !strings.Contains(reply.Error, tt.wantErr) {
				t.Fatalf("upsert %s: ok=%v err=%q, want %q", tt.name, reply.OK, reply.Error, tt.wantErr)
			}
		})
	}
}

func TestPushPack(t *testing.T) {
	reg := packLoader(t, writePackTree(t)).Current()
	h := newHarness(t)
	handleStart(h.hc(body{Title: "T"}), reg, nilDisplay)
	declared := `{"version":1,"title":"T","blocks":[{"id":"c1","type":"example.callout","tone":"warn"}]}`
	if reply := handlePush(h.hc(body{Doc: json.RawMessage(declared)}), reg, nilDisplay); !reply.OK {
		t.Fatalf("push declared pack block: %s", reply.Error)
	}
	undeclared := `{"version":1,"title":"T","blocks":[{"id":"g1","type":"ghost.thing"}]}`
	reply := handlePush(h.hc(body{Doc: json.RawMessage(undeclared)}), reg, nilDisplay)
	if reply.OK || !strings.Contains(reply.Error, "not installed") {
		t.Fatalf("push undeclared dotted type: ok=%v err=%q, want 'not installed'", reply.OK, reply.Error)
	}
}

func TestClose(t *testing.T) {
	h := newHarness(t)
	start := handleStart(h.hc(body{Title: "T"}), doc.NoPacks, nilDisplay)
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
	tests := []struct {
		name    string
		doc     string // seeds start; empty starts a subject with no document
		blockID string
		md      string
		wantErr string
	}{
		{name: "top-level approval", doc: approvalDoc, blockID: "a1", md: "answered"},
		{name: "card child input", doc: cardDoc, blockID: "in1", md: "noted"},
		{name: "non-interactive markdown", doc: cardDoc, blockID: "m1", md: "fyi"},
		{name: "unknown block", doc: approvalDoc, blockID: "nope", md: "x", wantErr: `unknown block "nope"`},
		{name: "reply before any doc", doc: "", blockID: "a1", md: "x", wantErr: `unknown block "a1"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			seed := body{Title: "T"}
			if tt.doc != "" {
				seed = body{Doc: json.RawMessage(tt.doc)}
			}
			start := handleStart(h.hc(seed), doc.NoPacks, nilDisplay)
			if !start.OK {
				t.Fatalf("start not ok: %s", start.Error)
			}
			reply := handleReply(h.hc(body{BlockID: tt.blockID, Md: tt.md}))
			got := h.eventsOfType(t, start.SubjectID, EventReplyCreated)
			if tt.wantErr != "" {
				if reply.OK || !strings.Contains(reply.Error, tt.wantErr) {
					t.Fatalf("reply %s: ok=%v err=%q, want %q", tt.name, reply.OK, reply.Error, tt.wantErr)
				}
				if len(got) != 0 {
					t.Fatalf("rejected reply appended %d reply.created, want 0", len(got))
				}
				return
			}
			if !reply.OK {
				t.Fatalf("reply not ok: %s", reply.Error)
			}
			if len(got) != 1 {
				t.Fatalf("got %d reply.created, want 1", len(got))
			}
			var p struct{ BlockID, Md string }
			if err := json.Unmarshal(got[0].Payload, &p); err != nil {
				t.Fatalf("decode reply payload: %v", err)
			}
			if p.BlockID != tt.blockID || p.Md != tt.md {
				t.Fatalf("reply payload = %+v, want blockId=%q md=%q", p, tt.blockID, tt.md)
			}
		})
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
				if r := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay); !r.OK {
					t.Fatalf("start not ok: %s", r.Error)
				}
			},
			wantRound: 2,
		},
		{
			name: "title-only on a clean round after submit",
			setup: func(t *testing.T, h *harness) {
				start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
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
				handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
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

func TestRevising(t *testing.T) {
	t.Run("announce top-level id records the working set", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		reply := handleRevising(h.hc(body{BlockIDs: []string{"a1"}, Note: "reworking"}))
		if !reply.OK {
			t.Fatalf("revising not ok: %s", reply.Error)
		}
		evs := h.eventsOfType(t, start.SubjectID, EventRevisingChanged)
		if len(evs) != 1 {
			t.Fatalf("revising.changed events = %d, want 1", len(evs))
		}
		st := reduceSubject(t, h, start.SubjectID)
		if got := st.Revising.BlockIDs; len(got) != 1 || got[0] != "a1" {
			t.Fatalf("revising.blockIds = %v, want [a1]", got)
		}
		if st.Revising.Note != "reworking" {
			t.Fatalf("revising.note = %q, want reworking", st.Revising.Note)
		}
	})

	t.Run("unknown id is rejected", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		reply := handleRevising(h.hc(body{BlockIDs: []string{"nope"}}))
		if reply.OK || !strings.Contains(reply.Error, "not a current top-level block") {
			t.Fatalf("revising: ok=%v err=%q, want a top-level rejection", reply.OK, reply.Error)
		}
	})

	t.Run("child id is rejected (only top-level blocks are addressable)", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Doc: json.RawMessage(cardDoc)}), doc.NoPacks, nilDisplay)
		reply := handleRevising(h.hc(body{BlockIDs: []string{"in1"}}))
		if reply.OK || !strings.Contains(reply.Error, "not a current top-level block") {
			t.Fatalf("revising: ok=%v err=%q, want a top-level rejection", reply.OK, reply.Error)
		}
	})

	t.Run("doc-level note with no ids skips id validation", func(t *testing.T) {
		h := newHarness(t)
		start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		reply := handleRevising(h.hc(body{Note: "drafting a new step"}))
		if !reply.OK {
			t.Fatalf("revising not ok: %s", reply.Error)
		}
		st := reduceSubject(t, h, start.SubjectID)
		if len(st.Revising.BlockIDs) != 0 || st.Revising.Note != "drafting a new step" {
			t.Fatalf("revising = %+v, want empty ids + doc-level note", st.Revising)
		}
	})

	t.Run("rejected on a closed artifact", func(t *testing.T) {
		h := newHarness(t)
		handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
		handleClose(h.hc(body{}), h.assets)
		reply := handleRevising(h.hc(body{BlockIDs: []string{"a1"}}))
		if reply.OK || !strings.Contains(reply.Error, "closed") {
			t.Fatalf("revising: ok=%v err=%q, want closed", reply.OK, reply.Error)
		}
	})
}

func reduceSubject(t *testing.T, h *harness, subjectID string) state.State {
	t.Helper()
	events, err := loadEvents(context.Background(), h.cc.DB(), subjectID)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	st, err := state.Reduce(events)
	if err != nil {
		t.Fatalf("reduce: %v", err)
	}
	return st
}

func TestOutcomes(t *testing.T) {
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(approvalDoc)}), doc.NoPacks, nilDisplay)
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
	reply := handlePush(h.hc(body{Doc: json.RawMessage(docJSON)}), doc.NoPacks, nilDisplay)
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
