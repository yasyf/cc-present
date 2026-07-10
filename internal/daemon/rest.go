package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
	"github.com/yasyf/cc-interact/sse"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
	"github.com/yasyf/cc-present/internal/web"
)

var validVerdict = map[string]bool{"approved": true, "rejected": true, "cleared": true}

// restServer holds the REST plane's shared state: the event-log connection, the
// Append chokepoint, the subject resolver, the asset store, and the SPA handler
// that /assets/{sha} falls through to for the app's own build files.
type restServer struct {
	db      *sql.DB
	append  ccd.AppendFunc
	resolve func(ctx context.Context, ref string) (string, bool, error)
	assets  *assets.Store
	static  http.Handler
}

// interactionReq is the POST /api/interactions body: which subject, the browser
// nonce keying retry idempotency, and one human interaction.
type interactionReq struct {
	Subject     string      `json:"subject"`
	Nonce       string      `json:"nonce"`
	Interaction interaction `json:"interaction"`
}

// interaction is the discriminated union over the human event payloads. Type is
// the event type; each handler reads only the fields its type uses.
type interaction struct {
	Type      string   `json:"type"`
	BlockID   string   `json:"blockId"`
	Verdict   string   `json:"verdict"`
	Note      string   `json:"note"`
	OptionIDs []string `json:"optionIds"`
	Text      string   `json:"text"`
	ID        string   `json:"id"`
	Revision  int      `json:"revision"`
}

// mountREST registers the human-interaction endpoint, the content-addressed
// asset store, and the SPA static handler on the daemon's mux. Go's pattern mux
// gives the more specific /api and /assets routes precedence over the catch-all.
func mountREST(s *ccd.Server, ast *assets.Store) {
	rs := &restServer{
		db:      s.DB(),
		append:  s.Append,
		resolve: s.ResolveSubject,
		assets:  ast,
		static:  sse.StaticHandler(web.Dist()),
	}
	mux := s.Mux()
	mux.HandleFunc("GET /api/sessions", rs.handleSessions)
	mux.HandleFunc("POST /api/interactions", rs.handleInteractions)
	mux.HandleFunc("POST /api/assets", rs.handlePutAsset)
	mux.HandleFunc("GET /assets/{sha}", rs.handleGetAsset)
	mux.Handle("/", rs.static)
}

// handleInteractions resolves the subject, rejects a closed artifact, validates
// the interaction against the reduced document, then appends it under the human
// origin with a retry-idempotent dedup key.
func (rs *restServer) handleInteractions(w http.ResponseWriter, r *http.Request) {
	var req interactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Nonce == "" {
		http.Error(w, "nonce required", http.StatusBadRequest)
		return
	}
	id, ok, err := rs.resolve(r.Context(), req.Subject)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "unknown subject: "+req.Subject, http.StatusNotFound)
		return
	}
	events, err := loadEvents(r.Context(), rs.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	st, err := state.Reduce(events)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if st.Interactions.Closed.Value {
		http.Error(w, "presentation is closed", http.StatusConflict)
		return
	}
	revision := 0
	for _, ev := range events {
		if ev.Type == EventDocReplaced {
			revision++
		}
	}
	payload, err := validateInteraction(&st, revision, &req.Interaction)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	seq, err := appendEvent(r.Context(), rs.append, &ccevent.Event{
		SubjectID: id, Origin: ccevent.OriginHuman, Type: req.Interaction.Type,
		Payload:  payload,
		DedupKey: req.Interaction.Type + ":" + req.Interaction.BlockID + ":" + req.Nonce,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

// validateInteraction checks one interaction against the reduced document and
// returns the reducer payload to append. It rejects an unknown block, a block
// whose type does not match the interaction, an out-of-set choice option, a
// verdict outside the enum, feedback on an approval that forbids it, a
// block-scoped interaction on a block from a round already closed, and a
// submit naming a revision the log never produced (revision is the count of
// doc.replaced events; 0 is a document never replaced). Submit is exempt from
// the round guard: it is what closes a round.
func validateInteraction(st *state.State, revision int, it *interaction) (json.RawMessage, error) {
	switch it.Type {
	case EventSubmit:
		if it.Revision < 0 || it.Revision > revision {
			return nil, fmt.Errorf("submit revision %d does not exist (current revision is %d)", it.Revision, revision)
		}
		return mustJSON(map[string]int{"revision": it.Revision}), nil
	case EventDecisionCreated:
		ap, topID, err := requireApproval(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		if !validVerdict[it.Verdict] {
			return nil, fmt.Errorf("invalid verdict %q", it.Verdict)
		}
		if it.Note != "" && !allowsFeedback(ap) {
			return nil, fmt.Errorf("block %q does not allow feedback", it.BlockID)
		}
		p := map[string]string{"blockId": it.BlockID, "verdict": it.Verdict}
		if it.Note != "" {
			p["note"] = it.Note
		}
		return mustJSON(p), nil
	case EventChoiceSelected:
		ch, topID, err := requireChoice(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		valid := map[string]bool{}
		for _, o := range ch.Options {
			valid[o.ID] = true
		}
		for _, oid := range it.OptionIDs {
			if !valid[oid] {
				return nil, fmt.Errorf("choice %q has no option %q", it.BlockID, oid)
			}
		}
		if !ch.Multi && len(it.OptionIDs) > 1 {
			return nil, fmt.Errorf("choice %q is single-select but %d options were chosen", it.BlockID, len(it.OptionIDs))
		}
		ids := it.OptionIDs
		if ids == nil {
			ids = []string{}
		}
		return mustJSON(map[string]any{"blockId": it.BlockID, "optionIds": ids}), nil
	case EventFeedbackCreated:
		ap, topID, err := requireApproval(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		if !allowsFeedback(ap) {
			return nil, fmt.Errorf("block %q does not allow feedback", it.BlockID)
		}
		if it.Text == "" {
			return nil, fmt.Errorf("feedback requires text")
		}
		id := it.ID
		if id == "" {
			id = randomHex(4)
		}
		return mustJSON(map[string]string{"id": id, "blockId": it.BlockID, "text": it.Text}), nil
	case EventInputSubmitted:
		_, topID, err := requireInput(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		return mustJSON(map[string]string{"blockId": it.BlockID, "text": it.Text}), nil
	default:
		return nil, fmt.Errorf("unknown interaction type %q", it.Type)
	}
}

func requireApproval(st *state.State, id string) (*doc.Approval, string, error) {
	b, topID, ok := findBlock(st.Doc, id)
	if !ok {
		return nil, "", fmt.Errorf("unknown block %q", id)
	}
	ap, ok := b.(*doc.Approval)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not an approval", id, b.BlockType())
	}
	return ap, topID, nil
}

func requireChoice(st *state.State, id string) (*doc.Choice, string, error) {
	b, topID, ok := findBlock(st.Doc, id)
	if !ok {
		return nil, "", fmt.Errorf("unknown block %q", id)
	}
	ch, ok := b.(*doc.Choice)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not a choice", id, b.BlockType())
	}
	return ch, topID, nil
}

func requireInput(st *state.State, id string) (*doc.Input, string, error) {
	b, topID, ok := findBlock(st.Doc, id)
	if !ok {
		return nil, "", fmt.Errorf("unknown block %q", id)
	}
	in, ok := b.(*doc.Input)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not an input", id, b.BlockType())
	}
	return in, topID, nil
}

// requireCurrentRound rejects an interaction on a block whose enclosing
// top-level block belongs to a round the reducer has already closed, so a human
// can never act on a superseded round's blocks.
func requireCurrentRound(st *state.State, id, topID string) error {
	if r := st.Rounds.BlockRounds[topID]; r != st.Rounds.Current {
		return fmt.Errorf("block %q belongs to closed round %d", id, r)
	}
	return nil
}

// findBlock locates a block by id at the top level or one level deep inside a
// card, mirroring where interactive blocks may appear. It also returns the id of
// the enclosing top-level block — the block itself when top-level, or the card
// when the block is a card child — which is the key the reducer stamps a round
// against.
func findBlock(d *doc.Doc, id string) (doc.Block, string, bool) {
	for _, b := range d.Blocks {
		if b.BlockID() == id {
			return b, b.BlockID(), true
		}
		if card, ok := b.(*doc.Card); ok {
			for _, child := range card.Children {
				if child.BlockID() == id {
					return child, card.BlockID(), true
				}
			}
		}
	}
	return nil, "", false
}

// allowsFeedback reports whether an approval permits free-text feedback;
// allowFeedback defaults to true when unset.
func allowsFeedback(ap *doc.Approval) bool {
	return ap.AllowFeedback == nil || *ap.AllowFeedback
}

// handlePutAsset content-addresses the raw request body into the asset store and
// returns its asset:<sha256> reference, rejecting a non-image body or one past
// the cap.
func (rs *restServer) handlePutAsset(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(io.LimitReader(r.Body, assets.MaxBytes+1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(b) > assets.MaxBytes {
		http.Error(w, fmt.Sprintf("asset exceeds %d bytes", assets.MaxBytes), http.StatusRequestEntityTooLarge)
		return
	}
	sha, err := rs.assets.Put(b)
	if errors.Is(err, assets.ErrNotImage) {
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"asset": "asset:" + sha, "sha": sha})
}

// handleGetAsset serves a stored asset immutably. A path that is not a stored
// asset falls through to the SPA static handler, so the app's own /assets/*
// build files still resolve.
func (rs *restServer) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	b, ct, err := rs.assets.Get(r.PathValue("sha"))
	if err != nil {
		rs.static.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	// Content-addressed bytes served under a sniffed type: forbid the browser from
	// re-sniffing a stored blob into active content.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // G705: b is content-addressed bytes served with a detected Content-Type and nosniff, not templated HTML.
	_, _ = w.Write(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
