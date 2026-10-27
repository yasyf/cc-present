package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/prs"
	"github.com/yasyf/cc-present/internal/state"
)

const prGroupsDoc = `{"version":1,"title":"T","blocks":[
  {"id":"pg","type":"pr-groups","repo":"a/b","groups":[
    {"id":"g1","title":"G","summary":"S.","prs":[
      {"number":8,"head":"abc1234","title":"T8","change":"C.","plan":"P.","risk":"R.","review":"V.","blocker":"B.","files":["web/src"]},
      {"number":9,"head":"def5678","title":"T9","change":"C.","plan":"P.","risk":"R.","review":"V.","blocker":"B."}
    ]}
  ]},
  {"id":"tr1","type":"triage","items":[{"id":"i1","label":"First"}]}
]}`

func TestInteractionPRGroupsValidation(t *testing.T) {
	post := func(verdicts string) string {
		return `{"subject":"board--abcd0000","nonce":"n","interaction":{"type":"triage.decided","blockId":"pg","verdicts":` + verdicts + `}}`
	}
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"approve at the row head", post(`{"8":{"verdict":"approved","head":"abc1234"}}`), 200, ""},
		{"hold by full sha prefix", post(`{"9":{"verdict":"rejected","head":"def5678ffff","note":"wait"}}`), 200, ""},
		{"clear", post(`{"8":{"verdict":"cleared"}}`), 200, ""},
		{"stale head refused", post(`{"8":{"verdict":"approved","head":"0001111"}}`), 400, `is not the row's head "abc1234"`},
		{"missing head refused", post(`{"8":{"verdict":"approved"}}`), 400, "is not the row's head"},
		{"unknown row", post(`{"77":{"verdict":"approved","head":"abc1234"}}`), 400, `has no pull request "77"`},
		{"client time refused", post(`{"8":{"verdict":"approved","head":"abc1234","at":"2020-01-01T00:00:00Z"}}`), 400, "stamps a verdict's time"},
		{"cleared with note refused", post(`{"8":{"verdict":"cleared","note":"x"}}`), 400, "carries no note or head"},
		{"head on a triage refused", `{"subject":"board--abcd0000","nonce":"n","interaction":{"type":"triage.decided","blockId":"tr1","verdicts":{"i1":{"verdict":"approved","head":"abc1234"}}}}`, 400, "only a pr-groups verdict carries a head"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, prGroupsDoc, emptyPackLoader(t))
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

func TestPRGroupsVerdictsStampHeadAndTime(t *testing.T) {
	var d doc.Doc
	if err := json.Unmarshal([]byte(prGroupsDoc), &d); err != nil {
		t.Fatal(err)
	}
	g := d.Blocks[0].(*doc.PRGroups)
	now := time.Date(2026, 10, 26, 21, 40, 5, 0, time.FixedZone("PDT", -7*3600))
	out, err := prGroupsVerdicts(g, map[string]triageEntry{
		"8": {Verdict: "approved", Head: "abc1234"},
		"9": {Verdict: "rejected", Head: "def5678ffff", Note: "wait"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]triageEntry{
		"8": {Verdict: "approved", Head: "abc1234", At: "2026-10-27T04:40:05Z"},
		"9": {Verdict: "rejected", Note: "wait", Head: "def5678", At: "2026-10-27T04:40:05Z"},
	}
	if len(out) != len(want) || out["8"] != want["8"] || out["9"] != want["9"] {
		t.Fatalf("verdicts = %+v, want %+v", out, want)
	}
}

func TestHandlePRServesPRGroupsRow(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantCode  int
		wantFiles []string
	}{
		{"row with files", "subject=board--abcd0000&block=pg&repo=a/b&number=8", http.StatusOK, []string{"web/src"}},
		{"row without files", "subject=board--abcd0000&block=pg&repo=a/b&number=9", http.StatusOK, nil},
		{"number not on the board", "subject=board--abcd0000&block=pg&repo=a/b&number=10", http.StatusNotFound, nil},
		{"another repo", "subject=board--abcd0000&block=pg&repo=evil/repo&number=8", http.StatusNotFound, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, prGroupsDoc, emptyPackLoader(t))
			var got *doc.PR
			h.rs.fetchPR = func(_ context.Context, b *doc.PR) (*prs.PR, error) {
				got = b
				return &prs.PR{Repo: b.Repo, Number: b.Number, Files: []prs.File{}}, nil
			}
			w := httptest.NewRecorder()
			h.rs.handlePR(w, httptest.NewRequest(http.MethodGet, "/api/pr?"+tt.query, nil))
			if w.Code != tt.wantCode {
				t.Fatalf("code = %d body = %q, want %d", w.Code, w.Body.String(), tt.wantCode)
			}
			if tt.wantCode != http.StatusOK {
				if got != nil {
					t.Fatalf("fetched %+v, want no fetch", got)
				}
				return
			}
			if got.Repo != "a/b" || len(got.Files) != len(tt.wantFiles) || (len(tt.wantFiles) > 0 && got.Files[0] != tt.wantFiles[0]) {
				t.Fatalf("fetched %+v, want a/b with files %v", got, tt.wantFiles)
			}
		})
	}
}

func TestPriorVerdictPayloads(t *testing.T) {
	at := time.Date(2026, 10, 26, 20, 3, 0, 0, time.UTC)
	tests := []struct {
		name     string
		priors   []PriorVerdict
		decided  map[string]state.Decision
		replace  bool
		want     string
		skipped  []string
		replaced []string
		wantErr  string
	}{
		{
			name:   "defaults the head to the row's",
			priors: []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved", At: at}},
			want:   `{"blockId":"pg","verdicts":{"8":{"verdict":"approved","head":"abc1234","at":"2026-10-26T20:03:00Z"}}}`,
		},
		{
			name:   "keeps an older head so the board asks again",
			priors: []PriorVerdict{{Block: "pg", Row: "9", Verdict: "rejected", At: at, Head: "0001111", Note: "flaky"}},
			want:   `{"blockId":"pg","verdicts":{"9":{"verdict":"rejected","note":"flaky","head":"0001111","at":"2026-10-26T20:03:00Z"}}}`,
		},
		{
			name:    "never overwrites a tap",
			priors:  []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved", At: at}, {Block: "pg", Row: "9", Verdict: "approved", At: at}},
			decided: map[string]state.Decision{"8": {Verdict: "rejected", Head: "abc1234"}},
			want:    `{"blockId":"pg","verdicts":{"9":{"verdict":"approved","head":"def5678","at":"2026-10-26T20:03:00Z"}}}`,
			skipped: []string{"pg/8"},
		},
		{
			name:     "replace overwrites a tap and records the carry",
			priors:   []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved", At: at, CarriedFrom: "0001111", CarryNote: "patch-identical replay"}},
			decided:  map[string]state.Decision{"8": {Verdict: "approved", Head: "0001111"}},
			replace:  true,
			want:     `{"blockId":"pg","verdicts":{"8":{"verdict":"approved","head":"abc1234","at":"2026-10-26T20:03:00Z","carriedFrom":"0001111","carryNote":"patch-identical replay"}}}`,
			replaced: []string{"pg/8"},
		},
		{name: "carry note without its head", priors: []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved", At: at, CarryNote: "same"}}, wantErr: "come together"},
		{name: "bad carried head", priors: []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved", At: at, CarriedFrom: "XYZ", CarryNote: "same"}}, wantErr: "carriedFrom: head must be"},
		{name: "unknown row", priors: []PriorVerdict{{Block: "pg", Row: "77", Verdict: "approved", At: at}}, wantErr: `no pull request "77"`},
		{name: "not a pr-groups", priors: []PriorVerdict{{Block: "tr1", Row: "8", Verdict: "approved", At: at}}, wantErr: "not a pr-groups"},
		{name: "cleared is not a prior verdict", priors: []PriorVerdict{{Block: "pg", Row: "8", Verdict: "cleared", At: at}}, wantErr: "approved or rejected"},
		{name: "needs a time", priors: []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved"}}, wantErr: "needs the time"},
		{name: "bad head", priors: []PriorVerdict{{Block: "pg", Row: "8", Verdict: "approved", At: at, Head: "XYZ"}}, wantErr: "head must be"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, prGroupsDoc, emptyPackLoader(t))
			events, err := loadEvents(t.Context(), h.cc.DB(), h.id)
			if err != nil {
				t.Fatal(err)
			}
			st, err := state.Reduce(events)
			if err != nil {
				t.Fatal(err)
			}
			if tt.decided != nil {
				st.Interactions.Triage["pg"] = tt.decided
			}
			payloads, res, err := priorVerdictPayloads(&st, tt.priors, tt.replace)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(payloads) != 1 || string(payloads[0]) != tt.want {
				t.Fatalf("payloads = %s, want [%s]", payloads, tt.want)
			}
			if strings.Join(res.Skipped, ",") != strings.Join(tt.skipped, ",") {
				t.Fatalf("skipped = %v, want %v", res.Skipped, tt.skipped)
			}
			if strings.Join(res.Replaced, ",") != strings.Join(tt.replaced, ",") {
				t.Fatalf("replaced = %v, want %v", res.Replaced, tt.replaced)
			}
			var p struct {
				Verdicts map[string]json.RawMessage `json:"verdicts"`
			}
			_ = json.Unmarshal(payloads[0], &p)
			if res.Imported != len(p.Verdicts) {
				t.Fatalf("imported = %d, want %d", res.Imported, len(p.Verdicts))
			}
		})
	}
}

func TestMoveVerdictPayloads(t *testing.T) {
	now := time.Date(2026, 10, 26, 21, 50, 0, 0, time.UTC)
	seeded := map[string]map[string]state.Decision{
		"old":  {"row-42732-ab4cced": {Verdict: "approved", Note: "ok"}},
		"tr1":  {"i1": {Verdict: "rejected"}},
		"gone": {"x": {Verdict: "approved", Head: "0001111", At: "2026-10-26T20:03:00Z"}},
	}
	tests := []struct {
		name    string
		moves   []VerdictMove
		taken   map[string]map[string]state.Decision
		want    []string
		wantErr string
	}{
		{
			name:  "triage verdict moves onto a pr-groups row at its head and clears the source",
			moves: []VerdictMove{{From: VerdictRef{"old", "row-42732-ab4cced"}, To: VerdictRef{"pg", "8"}}},
			want: []string{
				`{"blockId":"old","verdicts":{"row-42732-ab4cced":{"verdict":"cleared"}}}`,
				`{"blockId":"pg","verdicts":{"8":{"verdict":"approved","note":"ok","head":"abc1234","at":"2026-10-26T21:50:00Z"}}}`,
			},
		},
		{
			name:  "copy keeps the source and the recorded head and time",
			moves: []VerdictMove{{From: VerdictRef{"gone", "x"}, To: VerdictRef{"pg", "9"}, Keep: true}},
			want:  []string{`{"blockId":"pg","verdicts":{"9":{"verdict":"approved","head":"0001111","at":"2026-10-26T20:03:00Z"}}}`},
		},
		{
			name:  "pr-groups verdict moves onto a triage item without head or time",
			moves: []VerdictMove{{From: VerdictRef{"gone", "x"}, To: VerdictRef{"tr2", "j1"}, Keep: true}},
			want:  []string{`{"blockId":"tr2","verdicts":{"j1":{"verdict":"approved"}}}`},
		},
		{name: "no source verdict", moves: []VerdictMove{{From: VerdictRef{"old", "nope"}, To: VerdictRef{"pg", "8"}}}, wantErr: "no verdict recorded at old/nope"},
		{name: "target taken", moves: []VerdictMove{{From: VerdictRef{"old", "row-42732-ab4cced"}, To: VerdictRef{"tr1", "i1"}}}, wantErr: "already carries a verdict"},
		{name: "unknown target row", moves: []VerdictMove{{From: VerdictRef{"old", "row-42732-ab4cced"}, To: VerdictRef{"pg", "77"}}}, wantErr: `no pull request "77"`},
		{name: "unknown target block", moves: []VerdictMove{{From: VerdictRef{"old", "row-42732-ab4cced"}, To: VerdictRef{"nowhere", "1"}}}, wantErr: `unknown block "nowhere"`},
		{
			name:    "same target twice",
			moves:   []VerdictMove{{From: VerdictRef{"old", "row-42732-ab4cced"}, To: VerdictRef{"pg", "8"}}, {From: VerdictRef{"gone", "x"}, To: VerdictRef{"pg", "8"}}},
			wantErr: "named twice",
		},
		{name: "note onto a notes-free triage", moves: []VerdictMove{{From: VerdictRef{"old", "row-42732-ab4cced"}, To: VerdictRef{"trn", "k1"}}}, wantErr: "does not allow notes"},
	}
	const moveDoc = `{"version":1,"title":"T","blocks":[
  {"id":"pg","type":"pr-groups","repo":"a/b","groups":[{"id":"g1","title":"G","summary":"S.","prs":[
    {"number":8,"head":"abc1234","title":"T8","change":"C.","plan":"P.","risk":"R.","review":"V.","blocker":"B."},
    {"number":9,"head":"def5678","title":"T9","change":"C.","plan":"P.","risk":"R.","review":"V.","blocker":"B."}]}]},
  {"id":"tr1","type":"triage","items":[{"id":"i1","label":"First"}]},
  {"id":"tr2","type":"triage","items":[{"id":"j1","label":"Second"}]},
  {"id":"trn","type":"triage","allowNotes":false,"items":[{"id":"k1","label":"Third"}]}
]}`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, moveDoc, emptyPackLoader(t))
			events, err := loadEvents(t.Context(), h.cc.DB(), h.id)
			if err != nil {
				t.Fatal(err)
			}
			st, err := state.Reduce(events)
			if err != nil {
				t.Fatal(err)
			}
			for block, rows := range seeded {
				st.Interactions.Triage[block] = rows
			}
			payloads, err := moveVerdictPayloads(&st, tt.moves, now)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(payloads))
			for i, p := range payloads {
				got[i] = string(p)
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("payloads =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestCarryVerdictPayload(t *testing.T) {
	const tapped = "2026-10-26T20:03:00Z"
	tests := []struct {
		name    string
		decided map[string]state.Decision
		history map[string]state.Decision
		carry   VerdictCarry
		want    string
		res     CarryResult
		wantErr string
	}{
		{
			name:    "a tap at an old head stands at the row's head",
			decided: map[string]state.Decision{"8": {Verdict: "approved", Note: "ship it", Head: "0001111", At: tapped}},
			carry:   VerdictCarry{Block: "pg", Row: "8", Head: "abc1234ffff", Note: "patch-identical replay"},
			want:    `{"blockId":"pg","verdicts":{"8":{"verdict":"approved","note":"ship it","head":"abc1234","at":"2026-10-26T20:03:00Z","carriedFrom":"0001111","carryNote":"patch-identical replay"}}}`,
			res:     CarryResult{Verdict: "approved", From: "0001111", At: tapped, To: "abc1234"},
		},
		{
			name:    "a second carry names the head the human tapped",
			decided: map[string]state.Decision{"8": {Verdict: "rejected", Head: "2223333", At: tapped, CarriedFrom: "0001111", CarryNote: "rebase"}},
			carry:   VerdictCarry{Block: "pg", Row: "8", Head: "abc1234", Note: "restack"},
			want:    `{"blockId":"pg","verdicts":{"8":{"verdict":"rejected","head":"abc1234","at":"2026-10-26T20:03:00Z","carriedFrom":"0001111","carryNote":"restack"}}}`,
			res:     CarryResult{Verdict: "rejected", From: "0001111", At: tapped, To: "abc1234"},
		},
		{
			name:    "a tap from a closed round carries",
			history: map[string]state.Decision{"9": {Verdict: "approved", Head: "0001111", At: tapped}},
			carry:   VerdictCarry{Block: "pg", Row: "9", Head: "def5678", Note: "same patch"},
			want:    `{"blockId":"pg","verdicts":{"9":{"verdict":"approved","head":"def5678","at":"2026-10-26T20:03:00Z","carriedFrom":"0001111","carryNote":"same patch"}}}`,
			res:     CarryResult{Verdict: "approved", From: "0001111", At: tapped, To: "def5678"},
		},
		{
			name:    "a cleared row's closed-round tap at the head is restored",
			history: map[string]state.Decision{"9": {Verdict: "approved", Head: "def5678", At: tapped}},
			carry:   VerdictCarry{Block: "pg", Row: "9", Head: "def5678", Note: "same patch"},
			want:    `{"blockId":"pg","verdicts":{"9":{"verdict":"approved","head":"def5678","at":"2026-10-26T20:03:00Z"}}}`,
			res:     CarryResult{Verdict: "approved", From: "def5678", At: tapped, To: "def5678"},
		},
		{
			name:    "a verdict already at the head writes nothing",
			decided: map[string]state.Decision{"8": {Verdict: "approved", Head: "abc1234", At: tapped}},
			carry:   VerdictCarry{Block: "pg", Row: "8", Head: "abc1234", Note: "same"},
			res:     CarryResult{Verdict: "approved", From: "abc1234", At: tapped, To: "abc1234", Already: true},
		},
		{name: "no verdict to carry", carry: VerdictCarry{Block: "pg", Row: "8", Head: "abc1234", Note: "same"}, wantErr: "no recorded verdict"},
		{name: "a note is required", decided: map[string]state.Decision{"8": {Verdict: "approved", Head: "0001111", At: tapped}}, carry: VerdictCarry{Block: "pg", Row: "8", Head: "abc1234", Note: " "}, wantErr: "needs a note"},
		{name: "bad head", carry: VerdictCarry{Block: "pg", Row: "8", Head: "XYZ", Note: "same"}, wantErr: "head must be"},
		{
			name:    "a row a push took off the board carries for when it returns",
			decided: map[string]state.Decision{"77": {Verdict: "approved", Head: "0001111", At: tapped}},
			carry:   VerdictCarry{Block: "pg", Row: "77", Head: "4445555", Note: "same patch"},
			want:    `{"blockId":"pg","verdicts":{"77":{"verdict":"approved","head":"4445555","at":"2026-10-26T20:03:00Z","carriedFrom":"0001111","carryNote":"same patch"}}}`,
			res:     CarryResult{Verdict: "approved", From: "0001111", At: tapped, To: "4445555"},
		},
		{name: "unknown row without a verdict", carry: VerdictCarry{Block: "pg", Row: "77", Head: "abc1234", Note: "same"}, wantErr: "no recorded verdict"},
		{name: "not a pr-groups", carry: VerdictCarry{Block: "tr1", Row: "i1", Head: "abc1234", Note: "same"}, wantErr: "not a pr-groups"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, prGroupsDoc, emptyPackLoader(t))
			events, err := loadEvents(t.Context(), h.cc.DB(), h.id)
			if err != nil {
				t.Fatal(err)
			}
			st, err := state.Reduce(events)
			if err != nil {
				t.Fatal(err)
			}
			if tt.decided != nil {
				st.Interactions.Triage["pg"] = tt.decided
			}
			if tt.history != nil {
				st.Rounds.History = append(st.Rounds.History, state.RoundRecord{Triage: map[string]map[string]state.Decision{"pg": tt.history}})
			}
			payload, res, err := carryVerdictPayload(&st, tt.carry)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(payload) != tt.want {
				t.Fatalf("payload = %s, want %s", payload, tt.want)
			}
			if res != tt.res {
				t.Fatalf("result = %+v, want %+v", res, tt.res)
			}
		})
	}
}
