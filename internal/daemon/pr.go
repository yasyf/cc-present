package daemon

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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

// handlePR serves a pr block's live GitHub state. The requested repo and number
// must match a pr block the agent wrote into the subject's document, so a client
// can never point the user's gh credentials at a PR of its choosing.
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
	number, _ := strconv.Atoi(q.Get("number"))
	block := findPRBlock(&st, q.Get("block"), q.Get("repo"), number)
	if block == nil {
		http.Error(w, fmt.Sprintf("no pr block %s for %s#%s", q.Get("block"), q.Get("repo"), q.Get("number")), http.StatusNotFound)
		return
	}
	pr, err := rs.fetchPR(r.Context(), block)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, pr)
}

func findPRBlock(st *state.State, id, repo string, number int) *doc.PR {
	docs := []*doc.Doc{st.Doc}
	for i := len(st.Rounds.History) - 1; i >= 0; i-- {
		docs = append(docs, &doc.Doc{Blocks: st.Rounds.History[i].Blocks})
	}
	for _, d := range docs {
		loc, ok := doc.Locate(d, id)
		if !ok {
			continue
		}
		switch b := loc.Block.(type) {
		case *doc.PR:
			if b.Repo == repo && b.Number == number {
				return b
			}
		case *doc.PRGroups:
			if row, ok := b.Row(doc.RowID(number)); ok && b.Repo == repo {
				return &doc.PR{Repo: b.Repo, Number: row.Number, Files: row.Files}
			}
		}
	}
	return nil
}
