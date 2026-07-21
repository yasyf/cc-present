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
	"testing/fstest"

	ccevent "github.com/yasyf/cc-interact/event"
	"github.com/yasyf/cc-interact/sse"
	ccstore "github.com/yasyf/cc-interact/store"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/packs"
)

const seedDoc = `{"version":1,"title":"T","blocks":[
  {"id":"a1","type":"approval","allowFeedback":false},
  {"id":"a2","type":"approval"},
  {"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"code","lang":"go","code":"x"}},{"id":"o2","label":"B"}]},
  {"id":"ch2","type":"choice","multi":true,"options":[{"id":"m1","label":"A"},{"id":"m2","label":"B"}]},
  {"id":"in1","type":"input","label":"Name"},
  {"id":"t1","type":"triage","items":[{"id":"i1","label":"A","visual":{"id":"tv1","type":"code","lang":"go","code":"x"}}]}
]}`

// draftTriageDoc seeds a draft and two triage blocks (one forbidding notes) for
// the annotation and triage interaction tests. The d1 line anchors are real
// anchor.Of hashes: "Intro line."=dy65, "Second line here."=anp5, "Third and
// final."=xr17.
const draftTriageDoc = `{"version":1,"title":"T","blocks":[
  {"id":"d1","type":"draft","lang":"markdown","text":"Intro line.\nSecond line here.\nThird and final."},
  {"id":"tr1","type":"triage","items":[{"id":"i1","label":"First"},{"id":"i2","label":"Second"}]},
  {"id":"trn","type":"triage","allowNotes":false,"items":[{"id":"j1","label":"NoNotes"}]},
  {"id":"a1","type":"approval"}
]}`

type restHarness struct {
	rs *restServer
	cc *ccstore.Store
	id string
}

func newRestHarness(t *testing.T) *restHarness {
	return newRestHarnessWith(t, seedDoc, emptyPackLoader(t))
}

// emptyPackLoader is a loader over no dev dirs with an isolated Claude config dir,
// so a test never scans the developer's real installed plugins.
func emptyPackLoader(t *testing.T) *packs.Loader {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	return packs.NewLoader(nil, nil)
}

func newRestHarnessWith(t *testing.T, docJSON string, loader *packs.Loader) *restHarness {
	t.Helper()
	cc, err := ccstore.Open(context.Background(), t.TempDir()+"/t.db", ccstore.Schema{})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	ast := assets.New(t.TempDir())
	if err := ast.Prepare(); err != nil {
		t.Fatalf("prepare assets: %v", err)
	}
	rs := &restServer{
		db:     cc.DB,
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
		packs:  loader,
		// The real scoped static handler over a stub shell, so fallthrough tests
		// distinguish an honest 404 from the SPA index answering everything.
		static: sse.StaticHandler(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>shell</html>")}}),
	}
	subs := ccstore.NewSubjectStore(cc.DB())
	sub, err := subs.Create(context.Background(), "sub1", "board--abcd0000", "s1", "/repo", 100, "open")
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	if _, err := cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
		Payload: docReplacedPayload(json.RawMessage(docJSON), 1),
	}); err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	return &restHarness{rs: rs, cc: cc, id: sub.ID}
}

func (h *restHarness) post(t *testing.T, bodyJSON string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/interactions", strings.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.rs.handleInteractions(w, req)
	return w
}

// TestInteractionContentTypeGate proves the CSRF hardening: the preflight-free
// "simple" content types a hostile localhost page can POST are refused with 415
// before any body handling, while application/json (with or without a charset
// parameter) proceeds.
func TestInteractionContentTypeGate(t *testing.T) {
	const body = `{"subject":"board--abcd0000","nonce":"ct1","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`
	tests := []struct {
		name        string
		contentType string
		wantCode    int
	}{
		{"text/plain rejected", "text/plain", http.StatusUnsupportedMediaType},
		{"form-urlencoded rejected", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"missing content type rejected", "", http.StatusUnsupportedMediaType},
		{"json accepted", "application/json", http.StatusOK},
		{"json with charset accepted", "application/json; charset=utf-8", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarness(t)
			req := httptest.NewRequest(http.MethodPost, "/api/interactions", strings.NewReader(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()
			h.rs.handleInteractions(w, req)
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantCode == http.StatusUnsupportedMediaType && !strings.Contains(w.Body.String(), "application/json") {
				t.Fatalf("body = %q, want Content-Type complaint", w.Body.String())
			}
		})
	}
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
		{"valid submit", `{"subject":"board--abcd0000","nonce":"s1","interaction":{"type":"submit","revision":1}}`, 200, ""},
		{"submit before any doc.replaced seen", `{"subject":"board--abcd0000","nonce":"s2","interaction":{"type":"submit","revision":0}}`, 200, ""},
		{"submit future revision", `{"subject":"board--abcd0000","nonce":"s3","interaction":{"type":"submit","revision":999999}}`, 400, "does not exist"},
		{"submit negative revision", `{"subject":"board--abcd0000","nonce":"s4","interaction":{"type":"submit","revision":-1}}`, 400, "does not exist"},
		{"unknown block", `{"subject":"board--abcd0000","nonce":"n4","interaction":{"type":"decision.created","blockId":"zzz","verdict":"approved"}}`, 400, "unknown block"},
		{"decision on visual points to choice", `{"subject":"board--abcd0000","nonce":"visual-decision","interaction":{"type":"decision.created","blockId":"v1","verdict":"approved"}}`, 400, `block "v1" is the visual of option "o1" on choice "ch1"; address the choice`},
		{"selection on visual points to choice", `{"subject":"board--abcd0000","nonce":"visual-choice","interaction":{"type":"choice.selected","blockId":"v1","optionIds":["o1"]}}`, 400, `block "v1" is the visual of option "o1" on choice "ch1"; address the choice`},
		{"input on visual points to choice", `{"subject":"board--abcd0000","nonce":"visual-input","interaction":{"type":"input.submitted","blockId":"v1","text":"x"}}`, 400, `block "v1" is the visual of option "o1" on choice "ch1"; address the choice`},
		{"pack interaction on visual points to choice", `{"subject":"board--abcd0000","nonce":"visual-pack","interaction":{"type":"pack.interaction","blockId":"v1","payload":{}}}`, 400, `block "v1" is the visual of option "o1" on choice "ch1"; address the choice`},
		{"feedback on visual points to choice", `{"subject":"board--abcd0000","nonce":"visual-feedback","interaction":{"type":"feedback.created","blockId":"v1","text":"x"}}`, 400, `block "v1" is the visual of option "o1" on choice "ch1"; address the choice`},
		{"decision on triage visual points to triage", `{"subject":"board--abcd0000","nonce":"triage-visual-decision","interaction":{"type":"decision.created","blockId":"tv1","verdict":"approved"}}`, 400, `block "tv1" is the visual of item "i1" on triage "t1"; address the triage`},
		{"wrong kind", `{"subject":"board--abcd0000","nonce":"n5","interaction":{"type":"decision.created","blockId":"ch1","verdict":"approved"}}`, 400, "not an approval"},
		{"bad verdict", `{"subject":"board--abcd0000","nonce":"n6","interaction":{"type":"decision.created","blockId":"a2","verdict":"maybe"}}`, 400, "invalid verdict"},
		{"unknown choice option", `{"subject":"board--abcd0000","nonce":"n7","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o9"]}}`, 400, "no option"},
		{"single-select overload", `{"subject":"board--abcd0000","nonce":"n8","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1","o2"]}}`, 400, "single-select"},
		{"feedback forbidden", `{"subject":"board--abcd0000","nonce":"n9","interaction":{"type":"feedback.created","blockId":"a1","text":"hi"}}`, 400, "does not allow feedback"},
		{"note forbidden", `{"subject":"board--abcd0000","nonce":"n10","interaction":{"type":"decision.created","blockId":"a1","verdict":"approved","note":"x"}}`, 400, "does not allow feedback"},
		{"oversized decision note", `{"subject":"board--abcd0000","nonce":"n12","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved","note":"` + strings.Repeat("a", maxHumanTextBytes+1) + `"}}`, 400, "exceeds"},
		{"oversized input text", `{"subject":"board--abcd0000","nonce":"n14","interaction":{"type":"input.submitted","blockId":"in1","text":"` + strings.Repeat("a", maxHumanTextBytes+1) + `"}}`, 400, "exceeds"},
		{"duplicate optionIds", `{"subject":"board--abcd0000","nonce":"n13","interaction":{"type":"choice.selected","blockId":"ch2","optionIds":["m1","m1"]}}`, 400, "more than once"},
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

func TestInteractionChoiceOtherAndFeedback(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantCode         int
		wantErr          string
		checkChoice      bool
		wantOptionIDs    []string
		wantOther        string
		wantOtherPresent bool
	}{
		{
			name:     "single-select pick and other rejected",
			body:     `{"subject":"board--abcd0000","nonce":"other1","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1"],"other":"custom"}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "single-select",
		},
		{
			name:             "other-only accepted and trimmed",
			body:             `{"subject":"board--abcd0000","nonce":"other2","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":[],"other":"  custom  "}}`,
			wantCode:         http.StatusOK,
			checkChoice:      true,
			wantOptionIDs:    []string{},
			wantOther:        "custom",
			wantOtherPresent: true,
		},
		{
			name:             "multi-select pick and other accepted",
			body:             `{"subject":"board--abcd0000","nonce":"other-multi","interaction":{"type":"choice.selected","blockId":"ch2","optionIds":["m1"],"other":"custom"}}`,
			wantCode:         http.StatusOK,
			checkChoice:      true,
			wantOptionIDs:    []string{"m1"},
			wantOther:        "custom",
			wantOtherPresent: true,
		},
		{
			name:     "oversized other rejected",
			body:     `{"subject":"board--abcd0000","nonce":"other3","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":[],"other":"` + strings.Repeat("a", maxHumanTextBytes+1) + `"}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "exceeds",
		},
		{
			name:          "whitespace-only other treated as absent",
			body:          `{"subject":"board--abcd0000","nonce":"other4","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1"],"other":" \t\n "}}`,
			wantCode:      http.StatusOK,
			checkChoice:   true,
			wantOptionIDs: []string{"o1"},
		},
		{
			name:          "zero-width-only other treated as absent",
			body:          `{"subject":"board--abcd0000","nonce":"other5","interaction":{"type":"choice.selected","blockId":"ch1","optionIds":["o1"],"other":"\u200b\u200b"}}`,
			wantCode:      http.StatusOK,
			checkChoice:   true,
			wantOptionIDs: []string{"o1"},
		},
		{
			name:     "format-char-only feedback rejected",
			body:     `{"subject":"board--abcd0000","nonce":"feedback-cf","interaction":{"type":"feedback.created","blockId":"ch1","text":"\u200d\ufeff"}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "feedback requires text",
		},
		{
			name:     "feedback on choice accepted",
			body:     `{"subject":"board--abcd0000","nonce":"feedback1","interaction":{"type":"feedback.created","blockId":"ch1","text":"try another option"}}`,
			wantCode: http.StatusOK,
		},
		{
			name:     "oversized feedback rejected",
			body:     `{"subject":"board--abcd0000","nonce":"feedback-big","interaction":{"type":"feedback.created","blockId":"ch1","text":"` + strings.Repeat("a", maxHumanTextBytes+1) + `"}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "exceeds",
		},
		{
			name:     "feedback on unknown block rejected",
			body:     `{"subject":"board--abcd0000","nonce":"feedback2","interaction":{"type":"feedback.created","blockId":"zzz","text":"hello"}}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "unknown block",
		},
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
			if !tt.checkChoice {
				return
			}
			events, err := h.cc.EventsSince(context.Background(), h.id, 0, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 2 {
				t.Fatalf("events = %d, want 2", len(events))
			}
			var payload struct {
				OptionIDs []string `json:"optionIds"`
				Other     *string  `json:"other"`
			}
			if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if len(payload.OptionIDs) != len(tt.wantOptionIDs) {
				t.Fatalf("optionIds = %v, want %v", payload.OptionIDs, tt.wantOptionIDs)
			}
			for i := range payload.OptionIDs {
				if payload.OptionIDs[i] != tt.wantOptionIDs[i] {
					t.Fatalf("optionIds = %v, want %v", payload.OptionIDs, tt.wantOptionIDs)
				}
			}
			if (payload.Other != nil) != tt.wantOtherPresent {
				t.Fatalf("other present = %t, want %t", payload.Other != nil, tt.wantOtherPresent)
			}
			if payload.Other != nil && *payload.Other != tt.wantOther {
				t.Fatalf("other = %q, want %q", *payload.Other, tt.wantOther)
			}
		})
	}
}

func TestInteractionTooLarge(t *testing.T) {
	h := newRestHarness(t)
	body := `{"subject":"board--abcd0000","nonce":"big","interaction":{"type":"input.submitted","blockId":"in1","text":"` +
		strings.Repeat("a", maxInteractionBytes) + `"}}`
	w := h.post(t, body)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body %q)", w.Code, w.Body.String())
	}
	// The log is untouched: only the seed doc.replaced remains, no interaction.
	all, err := h.cc.EventsSince(context.Background(), h.id, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("events = %d, want 1 (oversized interaction not persisted)", len(all))
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

func TestInteractionClosedRound(t *testing.T) {
	h := newRestHarness(t)
	// Submit round 1 (its seed blocks carry the current round, so it is dirty):
	// this closes round 1 and advances to round 2.
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"sub","interaction":{"type":"submit","revision":1}}`); w.Code != http.StatusOK {
		t.Fatalf("submit status = %d (%s)", w.Code, w.Body.String())
	}
	// Upsert a fresh block for round 2; the seed blocks keep their round-1 stamp,
	// so the document now holds both a closed-round and a current-round block.
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: h.id, Origin: ccevent.OriginAgent, Type: EventBlockUpserted,
		Payload: blockUpsertedPayload(json.RawMessage(`{"id":"b2","type":"approval"}`), ""),
	}); err != nil {
		t.Fatalf("upsert round-2 block: %v", err)
	}
	// A decision on a round-1 block is rejected as belonging to a closed round.
	w := h.post(t, `{"subject":"board--abcd0000","nonce":"cr1","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("closed-round status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "closed round") {
		t.Fatalf("body = %q, want 'closed round'", w.Body.String())
	}
	// A decision on the current-round block still passes.
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"cr2","interaction":{"type":"decision.created","blockId":"b2","verdict":"approved"}}`); w.Code != http.StatusOK {
		t.Fatalf("current-round status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
}

func TestInteractionCarriedRound(t *testing.T) {
	h := newRestHarness(t)
	// A human approves a2, engaging round 1.
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"eng","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`); w.Code != http.StatusOK {
		t.Fatalf("engage status = %d (%s)", w.Code, w.Body.String())
	}
	// The agent advances the round, carrying the un-answered a1 forward; every
	// other seed block (a2 included) freezes into the closed round.
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: h.id, Origin: ccevent.OriginAgent, Type: EventRoundStarted,
		Payload: roundStartedPayload("", []string{"a1"}),
	}); err != nil {
		t.Fatalf("advance round: %v", err)
	}
	// The carried block rides into the current round, so it still accepts input.
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"car","interaction":{"type":"decision.created","blockId":"a1","verdict":"approved"}}`); w.Code != http.StatusOK {
		t.Fatalf("carried-round status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	// The frozen answered block is rejected as belonging to a closed round.
	w := h.post(t, `{"subject":"board--abcd0000","nonce":"frz","interaction":{"type":"decision.created","blockId":"a2","verdict":"approved"}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("closed-round status = %d, want 400 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "closed round") {
		t.Fatalf("body = %q, want 'closed round'", w.Body.String())
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

func TestInteractionDuplicateFeedbackID(t *testing.T) {
	h := newRestHarness(t)
	first := `{"subject":"board--abcd0000","nonce":"fb1","interaction":{"type":"feedback.created","blockId":"ch1","text":"first","id":"f1"}}`
	if w := h.post(t, first); w.Code != http.StatusOK {
		t.Fatalf("first feedback status = %d (%s)", w.Code, w.Body.String())
	}
	// A fresh request (new nonce) reusing the id already on the thread collides
	// with the key the browser lists feedback under, and is rejected.
	dup := `{"subject":"board--abcd0000","nonce":"fb2","interaction":{"type":"feedback.created","blockId":"ch1","text":"second","id":"f1"}}`
	w := h.post(t, dup)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate feedback id status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "already exists") {
		t.Fatalf("body = %q, want 'already exists'", w.Body.String())
	}
	all, err := h.cc.EventsSince(context.Background(), h.id, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range all {
		if e.Type == EventFeedbackCreated {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("got %d feedback.created, want 1 (duplicate rejected)", n)
	}
}

func TestAssetRoundTrip(t *testing.T) {
	h := newRestHarness(t)
	payload := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 16)...)

	req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/octet-stream")
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

func TestAssetRejectsNonImage(t *testing.T) {
	h := newRestHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/api/assets",
		strings.NewReader("<!doctype html><script>fetch('/api/interactions')</script>"))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.rs.handlePutAsset(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not an image") {
		t.Fatalf("body = %q, want 'not an image'", w.Body.String())
	}
}

// TestAssetContentTypeGate proves the CSRF hardening on the raw upload: the
// preflight-free "simple" content types are refused with 415 even when the body
// is a real image, while the CLI's application/octet-stream and an explicit
// image type proceed.
func TestAssetContentTypeGate(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 16)...)
	tests := []struct {
		name        string
		contentType string
		wantCode    int
	}{
		{"text/plain rejected", "text/plain", http.StatusUnsupportedMediaType},
		{"form-urlencoded rejected", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"multipart rejected", "multipart/form-data; boundary=x", http.StatusUnsupportedMediaType},
		{"missing content type rejected", "", http.StatusUnsupportedMediaType},
		{"octet-stream accepted", "application/octet-stream", http.StatusOK},
		{"image type accepted", "image/png", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarness(t)
			req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(png))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()
			h.rs.handlePutAsset(w, req)
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}

func TestAssetCap(t *testing.T) {
	h := newRestHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(bytes.Repeat([]byte{1}, assets.MaxBytes+1)))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.rs.handlePutAsset(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

// mux builds the full REST mux so a test can exercise route precedence: an
// unknown /api path and an unknown /assets path both 404 rather than reaching
// the SPA shell.
func (h *restHarness) mux() http.Handler {
	mux := http.NewServeMux()
	h.rs.routes(mux)
	return mux
}

func TestAssetGetUnknownFallsThrough(t *testing.T) {
	h := newRestHarness(t)
	sha := strings.Repeat("a", 64)
	req := httptest.NewRequest(http.MethodGet, "/assets/"+sha, nil)
	w := httptest.NewRecorder()
	h.mux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (unknown asset 404s through the mux)", w.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	h := newRestHarness(t)
	h.rs.version = "9.9.9"
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	h.mux().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var got struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if got.Version != "9.9.9" {
		t.Errorf("version = %q, want 9.9.9", got.Version)
	}
}

func TestAPICatchAll404(t *testing.T) {
	h := newRestHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/api/nope", nil)
	w := httptest.NewRecorder()
	h.mux().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (unknown /api path)", w.Code)
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

func TestInteractionDraftTriageValidation(t *testing.T) {
	bigText := strings.Repeat("a", maxHumanTextBytes+1)
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"valid annotation", `{"subject":"board--abcd0000","nonce":"an1","interaction":{"type":"annotation.created","blockId":"d1","anchor":"2#anp5","text":"needs a source"}}`, 200, ""},
		{"annotation on non-draft", `{"subject":"board--abcd0000","nonce":"an2","interaction":{"type":"annotation.created","blockId":"tr1","anchor":"2#anp5","text":"x"}}`, 400, "not a draft"},
		{"annotation unknown block", `{"subject":"board--abcd0000","nonce":"an3","interaction":{"type":"annotation.created","blockId":"zzz","anchor":"2#anp5","text":"x"}}`, 400, "unknown block"},
		{"annotation malformed anchor", `{"subject":"board--abcd0000","nonce":"an4","interaction":{"type":"annotation.created","blockId":"d1","anchor":"not-an-anchor","text":"x"}}`, 400, "invalid anchor reference"},
		{"annotation unresolvable anchor", `{"subject":"board--abcd0000","nonce":"an5","interaction":{"type":"annotation.created","blockId":"d1","anchor":"1#fa7h","text":"x"}}`, 400, "not found"},
		{"annotation empty text", `{"subject":"board--abcd0000","nonce":"an6","interaction":{"type":"annotation.created","blockId":"d1","anchor":"2#anp5","text":"   "}}`, 400, "annotation requires text"},
		{"annotation oversize text", `{"subject":"board--abcd0000","nonce":"an7","interaction":{"type":"annotation.created","blockId":"d1","anchor":"2#anp5","text":"` + bigText + `"}}`, 400, "exceeds"},
		{"annotation remove unknown id", `{"subject":"board--abcd0000","nonce":"an8","interaction":{"type":"annotation.removed","blockId":"d1","id":"nope"}}`, 400, "does not exist"},
		{"annotation remove on non-draft", `{"subject":"board--abcd0000","nonce":"an9","interaction":{"type":"annotation.removed","blockId":"tr1","id":"nope"}}`, 400, "not a draft"},

		{"valid triage decision", `{"subject":"board--abcd0000","nonce":"tr1","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"approved"},"i2":{"verdict":"rejected","note":"weak"}}}}`, 200, ""},
		{"triage on non-triage", `{"subject":"board--abcd0000","nonce":"tr2","interaction":{"type":"triage.decided","blockId":"d1","verdicts":{"i1":{"verdict":"approved"}}}}`, 400, "not a triage"},
		{"triage empty verdicts", `{"subject":"board--abcd0000","nonce":"tr3","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{}}}`, 400, "at least one verdict"},
		{"triage unknown item", `{"subject":"board--abcd0000","nonce":"tr4","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i9":{"verdict":"approved"}}}}`, 400, `no item "i9"`},
		{"triage invalid verdict", `{"subject":"board--abcd0000","nonce":"tr5","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"maybe"}}}}`, 400, "invalid verdict"},
		{"triage note on cleared", `{"subject":"board--abcd0000","nonce":"tr6","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"cleared","note":"why"}}}}`, 400, "cleared verdict cannot carry a note"},
		{"triage note when notes forbidden", `{"subject":"board--abcd0000","nonce":"tr7","interaction":{"type":"triage.decided","blockId":"trn","verdicts":{"j1":{"verdict":"approved","note":"hi"}}}}`, 400, "does not allow notes"},
		{"triage note without notes flag accepted", `{"subject":"board--abcd0000","nonce":"tr8","interaction":{"type":"triage.decided","blockId":"trn","verdicts":{"j1":{"verdict":"approved"}}}}`, 200, ""},
		{"triage oversize note", `{"subject":"board--abcd0000","nonce":"tr9","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"approved","note":"` + bigText + `"}}}}`, 400, "exceeds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, draftTriageDoc, emptyPackLoader(t))
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

// TestInteractionAnnotationNormalization proves the server rewrites a moved
// anchor to its resolved range and stamps the quote server-side, replacing a
// client-supplied lie: the hint points at line 1 but the hash matches line 2, so
// the echoed anchor moves to 2-2 and the quote becomes line 2's real text.
func TestInteractionAnnotationNormalization(t *testing.T) {
	h := newRestHarnessWith(t, draftTriageDoc, emptyPackLoader(t))
	body := `{"subject":"board--abcd0000","nonce":"norm","interaction":{"type":"annotation.created","blockId":"d1","anchor":"1#anp5","text":"needs a source","quote":"LIE"}}`
	if w := h.post(t, body); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	events, err := h.cc.EventsSince(context.Background(), h.id, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	var p struct {
		ID      string `json:"id"`
		BlockID string `json:"blockId"`
		Anchor  string `json:"anchor"`
		Text    string `json:"text"`
		Quote   string `json:"quote"`
	}
	if err := json.Unmarshal(events[1].Payload, &p); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if p.Anchor != "2-2#anp5" {
		t.Fatalf("anchor = %q, want %q (rewritten to resolved range)", p.Anchor, "2-2#anp5")
	}
	if p.Quote != "Second line here." {
		t.Fatalf("quote = %q, want %q (server-stamped over the client lie)", p.Quote, "Second line here.")
	}
	if p.ID == "" {
		t.Fatal("id = empty, want a server-defaulted id")
	}
	if p.Text != "needs a source" {
		t.Fatalf("text = %q, want %q", p.Text, "needs a source")
	}
}

// TestInteractionTriageEcho proves the appended triage payload mirrors the
// validated verdicts, omitting an empty note.
func TestInteractionTriageEcho(t *testing.T) {
	h := newRestHarnessWith(t, draftTriageDoc, emptyPackLoader(t))
	body := `{"subject":"board--abcd0000","nonce":"echo","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"approved"},"i2":{"verdict":"rejected","note":"weak"}}}}`
	if w := h.post(t, body); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	events, err := h.cc.EventsSince(context.Background(), h.id, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		BlockID  string `json:"blockId"`
		Verdicts map[string]struct {
			Verdict string  `json:"verdict"`
			Note    *string `json:"note"`
		} `json:"verdicts"`
	}
	if err := json.Unmarshal(events[1].Payload, &p); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if p.BlockID != "tr1" || len(p.Verdicts) != 2 {
		t.Fatalf("payload = %+v, want blockId tr1 with 2 verdicts", p)
	}
	if p.Verdicts["i1"].Verdict != "approved" || p.Verdicts["i1"].Note != nil {
		t.Fatalf("i1 = %+v, want approved with no note", p.Verdicts["i1"])
	}
	if p.Verdicts["i2"].Verdict != "rejected" || p.Verdicts["i2"].Note == nil || *p.Verdicts["i2"].Note != "weak" {
		t.Fatalf("i2 = %+v, want rejected with note weak", p.Verdicts["i2"])
	}
}

// TestInteractionDraftTriageClosedRound proves an annotation or triage decision
// on a block from a round the reducer has already closed is rejected.
func TestInteractionDraftTriageClosedRound(t *testing.T) {
	h := newRestHarnessWith(t, draftTriageDoc, emptyPackLoader(t))
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"sub","interaction":{"type":"submit","revision":1}}`); w.Code != http.StatusOK {
		t.Fatalf("submit status = %d (%s)", w.Code, w.Body.String())
	}
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: h.id, Origin: ccevent.OriginAgent, Type: EventBlockUpserted,
		Payload: blockUpsertedPayload(json.RawMessage(`{"id":"b2","type":"approval"}`), ""),
	}); err != nil {
		t.Fatalf("upsert round-2 block: %v", err)
	}
	ann := h.post(t, `{"subject":"board--abcd0000","nonce":"cr-an","interaction":{"type":"annotation.created","blockId":"d1","anchor":"2#anp5","text":"x"}}`)
	if ann.Code != http.StatusBadRequest || !strings.Contains(ann.Body.String(), "closed round") {
		t.Fatalf("annotation status = %d body %q, want 400 closed round", ann.Code, ann.Body.String())
	}
	tri := h.post(t, `{"subject":"board--abcd0000","nonce":"cr-tr","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"approved"}}}}`)
	if tri.Code != http.StatusBadRequest || !strings.Contains(tri.Body.String(), "closed round") {
		t.Fatalf("triage status = %d body %q, want 400 closed round", tri.Code, tri.Body.String())
	}
}
