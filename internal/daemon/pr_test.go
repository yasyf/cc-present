package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/prs"
	"github.com/yasyf/cc-present/internal/state"
)

const prDoc = `{"version":1,"title":"T","blocks":[
  {"id":"c1","type":"card","children":[{"id":"pr1","type":"pr","repo":"yasyf/cc-present","number":8,"files":["go.mod"]}]},
  {"id":"m1","type":"markdown","md":"not a pr"}
]}`

func TestHandlePR(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		fetchErr error
		wantCode int
		wantBody string
	}{
		{"card child pr", "subject=board--abcd0000&block=pr1", nil, http.StatusOK, `"title":"Bump"`},
		{"unknown subject", "subject=nope&block=pr1", nil, http.StatusNotFound, "unknown subject"},
		{"unknown block", "subject=board--abcd0000&block=zz", nil, http.StatusNotFound, "no pr block zz"},
		{"non-pr block", "subject=board--abcd0000&block=m1", nil, http.StatusNotFound, "no pr block m1"},
		{"gh failure", "subject=board--abcd0000&block=pr1", errors.New("gh api graphql: exit status 1: HTTP 401"), http.StatusBadGateway, "HTTP 401"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRestHarnessWith(t, prDoc, emptyPackLoader(t))
			var got *doc.PR
			h.rs.fetchPR = func(_ context.Context, b *doc.PR) (*prs.PR, error) {
				got = b
				if tt.fetchErr != nil {
					return nil, tt.fetchErr
				}
				return &prs.PR{Repo: b.Repo, Number: b.Number, Title: "Bump", Files: []prs.File{}}, nil
			}
			w := httptest.NewRecorder()
			h.rs.handlePR(w, httptest.NewRequest(http.MethodGet, "/api/pr?"+tt.query, nil))
			if w.Code != tt.wantCode || !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Fatalf("code = %d body = %q, want %d containing %q", w.Code, w.Body.String(), tt.wantCode, tt.wantBody)
			}
			if tt.wantCode == http.StatusOK || tt.fetchErr != nil {
				if got == nil || got.Repo != "yasyf/cc-present" || got.Number != 8 || !slices.Equal(got.Files, []string{"go.mod"}) {
					t.Fatalf("fetched %+v, want the document's pr block", got)
				}
				return
			}
			if got != nil {
				t.Fatalf("fetched %+v for a request that names no pr block", got)
			}
		})
	}
}

func TestFindPRBlockReadsFrozenRounds(t *testing.T) {
	var d doc.Doc
	if err := json.Unmarshal([]byte(prDoc), &d); err != nil {
		t.Fatal(err)
	}
	st := state.State{Doc: &d}
	if b := findPRBlock(&st, "pr1"); b == nil || b.Number != 8 {
		t.Fatalf("live pr block = %+v", b)
	}
	st = state.State{
		Doc:    &doc.Doc{Version: 1, Blocks: []doc.Block{}},
		Rounds: state.Rounds{History: []state.RoundRecord{{Number: 1, Blocks: d.Blocks}}},
	}
	if b := findPRBlock(&st, "pr1"); b == nil || b.Number != 8 {
		t.Fatalf("frozen pr block = %+v", b)
	}
}

func TestPRRoute(t *testing.T) {
	h := newRestHarnessWith(t, prDoc, emptyPackLoader(t))
	h.rs.fetchPR = func(_ context.Context, b *doc.PR) (*prs.PR, error) {
		return &prs.PR{Repo: b.Repo, Number: b.Number, Files: []prs.File{}}, nil
	}
	mux := http.NewServeMux()
	h.rs.routes(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/pr?subject=board--abcd0000&block=pr1", nil))
	var body prs.PR
	if err := json.Unmarshal(w.Body.Bytes(), &body); w.Code != http.StatusOK || err != nil || body.Number != 8 {
		t.Fatalf("code = %d body = %q err = %v", w.Code, w.Body.String(), err)
	}
}
