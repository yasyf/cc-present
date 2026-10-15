package daemon

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/prs"
	"github.com/yasyf/cc-present/internal/state"
	"github.com/yasyf/cc-present/internal/toolpath"
)

const prFetchTimeout = 30 * time.Second

type prFetcher func(ctx context.Context, b *doc.PR) (*prs.PR, error)

func ghPRFetcher(ctx context.Context, b *doc.PR) (*prs.PR, error) {
	gh, err := toolpath.Find("gh")
	if err != nil {
		return nil, fmt.Errorf("pr block needs gh: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, prFetchTimeout)
	defer cancel()
	return prs.Fetch(ctx, prs.GH(gh), b.Repo, b.Number, b.Files, time.Now())
}

// handlePR serves a pr block's live GitHub state. It reads only the repo and
// number the agent wrote into the subject's document, so a client can never
// point the user's gh credentials at a PR of its choosing.
func (rs *restServer) handlePR(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id, ok, err := rs.resolve(r.Context(), q.Get("subject"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "unknown subject: "+q.Get("subject"), http.StatusNotFound)
		return
	}
	events, err := loadEvents(r.Context(), rs.db(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	st, err := state.Reduce(events)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	block := findPRBlock(&st, q.Get("block"))
	if block == nil {
		http.Error(w, "no pr block "+q.Get("block"), http.StatusNotFound)
		return
	}
	pr, err := rs.fetchPR(r.Context(), block)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

func findPRBlock(st *state.State, id string) *doc.PR {
	docs := []*doc.Doc{st.Doc}
	for i := len(st.Rounds.History) - 1; i >= 0; i-- {
		docs = append(docs, &doc.Doc{Blocks: st.Rounds.History[i].Blocks})
	}
	for _, d := range docs {
		if loc, ok := doc.Locate(d, id); ok {
			pr, _ := loc.Block.(*doc.PR)
			return pr
		}
	}
	return nil
}
