package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/liveread"
)

type liveReader func(ctx context.Context, bindings []liveread.Binding, cards []liveread.Card) ([]liveread.Result, error)

func hostLiveReader(stateDir string) liveReader {
	return func(ctx context.Context, bindings []liveread.Binding, cards []liveread.Card) ([]liveread.Result, error) {
		command, err := liveread.HostCommand(stateDir)
		if err != nil {
			return nil, err
		}
		return liveread.Read(ctx, command, bindings, cards)
	}
}

// handleLive answers a live block's rows from the host's reader command, read
// when it is drawn and never stored on the board.
func (rs *restServer) handleLive(w http.ResponseWriter, r *http.Request) {
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
	board, err := caughtUp(r.Context(), rs.db(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	loc, found := doc.Locate(board.st.Doc, q.Get("block"))
	var block *doc.Live
	if found {
		block, found = loc.Block.(*doc.Live)
	}
	if !found {
		board.mu.Unlock()
		http.Error(w, fmt.Sprintf("no live block %s", q.Get("block")), http.StatusNotFound)
		return
	}
	bindings, cards := liveread.Bindings([]*doc.Live{block}), liveread.Cards(&board.st)
	board.mu.Unlock()
	results, err := rs.readLive(r.Context(), bindings, cards)
	if errors.Is(err, liveread.ErrNoCommand) {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, results[0])
}
