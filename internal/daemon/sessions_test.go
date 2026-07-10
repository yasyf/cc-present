package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	ccevent "github.com/yasyf/cc-interact/event"
	ccstore "github.com/yasyf/cc-interact/store"
)

// sessionsHarness holds a real ephemeral store and a restServer wired to it, so
// GET /api/sessions can be driven and its JSON asserted.
type sessionsHarness struct {
	cc *ccstore.Store
	rs *restServer
}

func newSessionsHarness(t *testing.T) *sessionsHarness {
	t.Helper()
	cc, err := ccstore.Open(filepath.Join(t.TempDir(), "t.db"), nil)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return &sessionsHarness{cc: cc, rs: &restServer{db: cc.DB(), append: cc.AppendEvent}}
}

// seed creates a subject with the given status and updated_at, then appends one
// doc.replaced per title so the latest title and the revision count are testable.
func (h *sessionsHarness) seed(t *testing.T, id, slug, status string, updated time.Time, titles ...string) {
	t.Helper()
	ctx := context.Background()
	subs := ccstore.NewSubjectStore(h.cc.DB())
	if _, err := subs.Create(ctx, id, slug, "s-"+id, "/repo/"+id, 100, status); err != nil {
		t.Fatalf("create subject %s: %v", id, err)
	}
	for i, title := range titles {
		raw := json.RawMessage(fmt.Sprintf(`{"version":1,"title":%q,"blocks":[]}`, title))
		if _, err := h.cc.AppendEvent(ctx, &ccevent.Event{
			SubjectID: id, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
			Payload: docReplacedPayload(raw, i+1),
		}); err != nil {
			t.Fatalf("seed doc %s: %v", id, err)
		}
	}
	if _, err := h.cc.DB().ExecContext(ctx,
		`UPDATE subjects SET updated_at=? WHERE id=?`, updated.Unix(), id); err != nil {
		t.Fatalf("set updated_at %s: %v", id, err)
	}
}

func (h *sessionsHarness) get(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	h.rs.handleSessions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	return w
}

func TestSessionsEmpty(t *testing.T) {
	h := newSessionsHarness(t)
	w := h.get(t)
	if body := w.Body.String(); body != "[]\n" {
		t.Fatalf("empty listing = %q, want %q", body, "[]\n")
	}
}

func TestSessionsExcludesClosed(t *testing.T) {
	h := newSessionsHarness(t)
	base := time.Unix(1_700_000_000, 0)
	h.seed(t, "open1", "a--0001", statusOpen, base.Add(2*time.Minute), "Alpha")
	h.seed(t, "open2", "b--0002", statusOpen, base.Add(1*time.Minute), "Beta")
	h.seed(t, "gone", "c--0003", statusClosed, base.Add(3*time.Minute), "Gamma")

	var got []sessionSummary
	if err := json.Unmarshal(h.get(t).Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2 (closed excluded): %+v", len(got), got)
	}
	// ORDER BY updated_at DESC: open1 (t+2m) before open2 (t+1m).
	if got[0].Subject != "open1" || got[1].Subject != "open2" {
		t.Fatalf("order = [%s %s], want [open1 open2]", got[0].Subject, got[1].Subject)
	}
	for _, s := range got {
		if s.Status != statusOpen {
			t.Fatalf("session %s status = %q, want open", s.Subject, s.Status)
		}
	}
}

func TestSessionsTitleAndRevisionFromLatestDoc(t *testing.T) {
	h := newSessionsHarness(t)
	h.seed(t, "s1", "board--abcd", statusOpen, time.Unix(1_700_000_000, 0), "First", "Second", "Third")

	var got []sessionSummary
	if err := json.Unmarshal(h.get(t).Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sessions, want 1", len(got))
	}
	if got[0].Title != "Third" {
		t.Fatalf("title = %q, want %q (latest doc.replaced)", got[0].Title, "Third")
	}
	if got[0].Slug != "board--abcd" {
		t.Fatalf("slug = %q, want board--abcd", got[0].Slug)
	}
	if got[0].Revision != 3 {
		t.Fatalf("revision = %d, want 3 (count of doc.replaced)", got[0].Revision)
	}
}

// TestSessionsUpdatedAtIsRFC3339String is the hard contract: the already-built
// Swift client decodes updatedAt as a string, so it must serialize as a quoted
// RFC3339 timestamp — Go time.Time's default JSON encoding.
func TestSessionsUpdatedAtIsRFC3339String(t *testing.T) {
	h := newSessionsHarness(t)
	updated := time.Unix(1_700_000_000, 0)
	h.seed(t, "s1", "board--abcd", statusOpen, updated, "Only")

	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(h.get(t).Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("got %d rows, want 1", len(raw))
	}
	field, ok := raw[0]["updatedAt"]
	if !ok {
		t.Fatal("response has no updatedAt field")
	}
	// A JSON string is quoted; unmarshal into a Go string and parse as RFC3339.
	var s string
	if err := json.Unmarshal(field, &s); err != nil {
		t.Fatalf("updatedAt is not a JSON string: %v (raw %s)", err, field)
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("updatedAt %q is not RFC3339: %v", s, err)
	}
	if !parsed.Equal(updated) {
		t.Fatalf("updatedAt = %s, want %s", parsed, updated)
	}
}
