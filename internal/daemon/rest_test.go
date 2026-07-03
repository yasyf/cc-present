package daemon

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ccevent "github.com/yasyf/cc-interact/event"
	ccstore "github.com/yasyf/cc-interact/store"

	"github.com/yasyf/cc-present/internal/assets"
)

const seedDoc = `{"version":1,"title":"T","blocks":[
  {"id":"a1","type":"approval","allowFeedback":false},
  {"id":"a2","type":"approval"},
  {"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"},{"id":"o2","label":"B"}]},
  {"id":"in1","type":"input","label":"Name"}
]}`

type restHarness struct {
	rs *restServer
	cc *ccstore.Store
	id string
}

func newRestHarness(t *testing.T) *restHarness {
	t.Helper()
	cc, err := ccstore.Open(t.TempDir()+"/t.db", nil)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	ast, err := assets.New(t.TempDir())
	if err != nil {
		t.Fatalf("assets: %v", err)
	}
	rs := &restServer{
		db:     cc.DB(),
		append: cc.AppendEvent,
		resolve: func(ctx context.Context, ref string) (string, bool, error) {
			var id string
			err := cc.DB().QueryRowContext(ctx, `SELECT id FROM subjects WHERE id=? OR slug=?`, ref, ref).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				return "", false, nil
			}
			return id, err == nil, err
		},
		assets: ast,
		static: http.NotFoundHandler(),
	}
	subs := ccstore.NewSubjectStore(cc.DB())
	sub, err := subs.Create(context.Background(), "sub1", "board--abcd0000", "s1", "/repo", 100, "open")
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	if _, err := cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
		Payload: docReplacedPayload(json.RawMessage(seedDoc), 1),
	}); err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	return &restHarness{rs: rs, cc: cc, id: sub.ID}
}

func (h *restHarness) post(t *testing.T, bodyJSON string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/interactions", strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()
	h.rs.handleInteractions(w, req)
	return w
}

func TestInteractionValidation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"valid decision", `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`, 200, ""},
		{"valid choice", `{"subject":"board--abcd0000","nonce":"n2","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1"]}}`, 200, ""},
		{"valid input", `{"subject":"board--abcd0000","nonce":"n3","interaction":{"type":"input.submitted","blockId":"in1","text":"Ada"}}`, 200, ""},
		{"unknown block", `{"subject":"board--abcd0000","nonce":"n4","interaction":{"type":"decision.created","blockId":"zzz","verdict":"approved"}}`, 400, "unknown block"},
		{"wrong kind", `{"subject":"board--abcd0000","nonce":"n5","interaction":{"type":"decision.created","blockId":"ch1","verdict":"approved"}}`, 400, "not an approval"},
		{"bad verdict", `{"subject":"board--abcd0000","nonce":"n6","interaction":{"type":"decision.created","blockId":"a2","verdict":"maybe"}}`, 400, "invalid verdict"},
		{"unknown choice option", `{"subject":"board--abcd0000","nonce":"n7","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o9"]}}`, 400, "no option"},
		{"single-select overload", `{"subject":"board--abcd0000","nonce":"n8","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1","o2"]}}`, 400, "single-select"},
		{"feedback forbidden", `{"subject":"board--abcd0000","nonce":"n9","interaction":{"type":"feedback.created","blockId":"a1","text":"hi"}}`, 400, "does not allow feedback"},
		{"note forbidden", `{"subject":"board--abcd0000","nonce":"n10","interaction":{"type":"decision.created","blockId":"a1","verdict":"approved","note":"x"}}`, 400, "does not allow feedback"},
		{"missing nonce", `{"subject":"board--abcd0000","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`, 400, "nonce"},
		{"unknown subject", `{"subject":"nope","nonce":"n11","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`, 404, "unknown subject"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarness(t)
			w := h.post(t, tt.body)
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantErr != "" && !strings.Contains(w.Body.String(), tt.wantErr) {
				t.Fatalf("body = %q, want %q", w.Body.String(), tt.wantErr)
			}
		})
	}
}

func TestInteractionClosed(t *testing.T) {
	h := newRestHarness(t)
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: h.id, Origin: ccevent.OriginAgent, Type: EventPresentClosed, Payload: json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("close: %v", err)
	}
	w := h.post(t, `{"subject":"board--abcd0000","nonce":"nz","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %q)", w.Code, w.Body.String())
	}
}

func TestInteractionDedup(t *testing.T) {
	h := newRestHarness(t)
	const b = `{"subject":"board--abcd0000","nonce":"same","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`
	first := seqOf(t, h.post(t, b))
	second := seqOf(t, h.post(t, b))
	if first != second {
		t.Fatalf("dedup seq mismatch: %d vs %d", first, second)
	}
	all, err := h.cc.EventsSince(context.Background(), h.id, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range all {
		if e.Type == EventDecisionCreated {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("got %d decision.created, want 1 (deduped)", n)
	}
}

func TestAssetRoundTrip(t *testing.T) {
	h := newRestHarness(t)
	payload := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 16)...)

	req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.rs.handlePutAsset(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put status = %d (%s)", w.Code, w.Body.String())
	}
	var put struct{ Asset, Sha string }
	if err := json.Unmarshal(w.Body.Bytes(), &put); err != nil {
		t.Fatalf("decode put: %v", err)
	}
	if put.Asset != "asset:"+assets.SHA(payload) {
		t.Fatalf("asset ref = %q", put.Asset)
	}

	greq := httptest.NewRequest(http.MethodGet, "/assets/"+put.Sha, nil)
	greq.SetPathValue("sha", put.Sha)
	gw := httptest.NewRecorder()
	h.rs.handleGetAsset(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("get status = %d", gw.Code)
	}
	if !bytes.Equal(gw.Body.Bytes(), payload) {
		t.Fatal("get bytes mismatch")
	}
	if cc := gw.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("cache-control = %q, want immutable", cc)
	}
	if ct := gw.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type = %q, want image/png", ct)
	}
}

func TestAssetCap(t *testing.T) {
	h := newRestHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(bytes.Repeat([]byte{1}, assets.MaxBytes+1)))
	w := httptest.NewRecorder()
	h.rs.handlePutAsset(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

func TestAssetGetUnknownFallsThrough(t *testing.T) {
	h := newRestHarness(t)
	sha := strings.Repeat("a", 64)
	greq := httptest.NewRequest(http.MethodGet, "/assets/"+sha, nil)
	greq.SetPathValue("sha", sha)
	gw := httptest.NewRecorder()
	h.rs.handleGetAsset(gw, greq)
	if gw.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (static fallthrough)", gw.Code)
	}
}

func seqOf(t *testing.T, w *httptest.ResponseRecorder) int64 {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", w.Code, w.Body.String())
	}
	var out struct {
		Seq int64 `json:"seq"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode seq: %v", err)
	}
	return out.Seq
}
