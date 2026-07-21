package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yasyf/cc-context/anchor"
	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
	"github.com/yasyf/cc-interact/sse"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/packs"
	"github.com/yasyf/cc-present/internal/state"
	"github.com/yasyf/cc-present/internal/web"
)

var validVerdict = map[string]bool{"approved": true, "rejected": true, "cleared": true}

// maxInteractionBytes caps the POST /api/interactions body so an oversized
// payload can't be decoded into memory or persisted to the event log.
const maxInteractionBytes = 256 << 10

const maxHumanTextBytes = 64 << 10

// maxQuoteBytes caps the server-stamped annotation quote so a draft with very long
// lines can't bloat the anchored excerpt echoed back into the log.
const maxQuoteBytes = 2 << 10

// restServer holds the REST plane's shared state: the event-log connection, the
// Append chokepoint, the subject resolver, the asset store, and the SPA handler
// that /assets/{sha} falls through to for the app's own build files.
type restServer struct {
	db      func() *sql.DB
	append  ccd.AppendFunc
	resolve func(ctx context.Context, ref string) (string, bool, error)
	assets  *assets.Store
	packs   *packs.Loader
	static  http.Handler
	version string
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
	Type      string                 `json:"type"`
	BlockID   string                 `json:"blockId"`
	Verdict   string                 `json:"verdict"`
	Note      string                 `json:"note"`
	OptionIDs []string               `json:"optionIds"`
	Other     string                 `json:"other"`
	Text      string                 `json:"text"`
	ID        string                 `json:"id"`
	Anchor    string                 `json:"anchor,omitempty"`
	Quote     string                 `json:"quote,omitempty"`
	Verdicts  map[string]triageEntry `json:"verdicts,omitempty"`
	Revision  int                    `json:"revision"`
	Payload   json.RawMessage        `json:"payload"`
}

// triageEntry is one item's verdict in a triage.decided interaction, mirrored
// verbatim into the appended payload with an empty note omitted.
type triageEntry struct {
	Verdict string `json:"verdict"`
	Note    string `json:"note,omitempty"`
}

// mountREST registers the human-interaction endpoint, the content-addressed
// asset store, the pack registry and bundle routes, and the SPA static handler
// on the daemon's mux. Go's pattern mux gives the more specific /api, /assets,
// and /packs routes precedence over the catch-all.
func mountREST(s *ccd.Server, ast *assets.Store, loader *packs.Loader, version string) {
	rs := &restServer{
		db:      s.DB,
		append:  s.Append,
		resolve: s.ResolveSubject,
		assets:  ast,
		packs:   loader,
		static:  sse.StaticHandler(web.Dist()),
		version: version,
	}
	rs.routes(s.Mux())
}

// routes registers every REST and static handler on mux. Go's pattern mux gives
// the specific /api and /assets routes precedence over the /api/ catch-all and
// the SPA catch-all, so an unknown /api path 404s instead of reaching the shell.
func (rs *restServer) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", rs.handleHealth)
	mux.HandleFunc("GET /api/sessions", rs.handleSessions)
	mux.HandleFunc("POST /api/interactions", rs.handleInteractions)
	mux.HandleFunc("POST /api/assets", rs.handlePutAsset)
	mux.HandleFunc("GET /api/packs", rs.handlePacks)
	mux.HandleFunc("GET /packs/{pack}/{file...}", rs.handlePackFile)
	mux.HandleFunc("GET /assets/{sha}", rs.handleGetAsset)
	mux.HandleFunc("/api/", rs.handleAPINotFound)
	mux.Handle("/", rs.static)
}

// handleHealth reports the daemon's build version — a liveness probe for a
// client that reached the HTTP plane.
func (rs *restServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": rs.version})
}

// handleAPINotFound 404s any /api path no specific route claimed, so an unknown
// API call never falls through to the SPA shell and reads back as HTML.
func (rs *restServer) handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
}

// requireJSON rejects a request whose Content-Type is not application/json
// with 415, reporting whether the caller may proceed. Every state-changing
// JSON route sits behind it as CSRF hardening: the daemon's auth layer admits
// tokenless loopback and trusted-peer requests under a localhost Origin, and a
// hostile page on such a machine can fire preflight-free "simple" POSTs
// (text/plain, form encodings) — but it cannot send application/json without a
// CORS preflight the daemon never answers.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	return true
}

// handleInteractions resolves the subject, rejects a closed artifact, validates
// the interaction against the reduced document, then appends it under the human
// origin with a retry-idempotent dedup key.
func (rs *restServer) handleInteractions(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxInteractionBytes)
	var req interactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, fmt.Sprintf("interaction exceeds %d bytes", maxInteractionBytes), http.StatusRequestEntityTooLarge)
			return
		}
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
	payload, err := validateInteraction(&st, revision, &req.Interaction, rs.packs.Current())
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
// the round guard: it is what closes a round. A pack.interaction validates its
// payload against the block's declared interaction schema in reg.
func validateInteraction(st *state.State, revision int, it *interaction, reg *packs.Registry) (json.RawMessage, error) {
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
		if len(it.Note) > maxHumanTextBytes {
			return nil, fmt.Errorf("decision %q note exceeds %d bytes", it.BlockID, maxHumanTextBytes)
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
		seen := map[string]bool{}
		for _, oid := range it.OptionIDs {
			if !valid[oid] {
				return nil, fmt.Errorf("choice %q has no option %q", it.BlockID, oid)
			}
			if seen[oid] {
				return nil, fmt.Errorf("choice %q selects option %q more than once", it.BlockID, oid)
			}
			seen[oid] = true
		}
		other := strings.TrimSpace(it.Other)
		if visuallyEmpty(other) {
			other = ""
		}
		if len(other) > maxHumanTextBytes {
			return nil, fmt.Errorf("choice %q other text exceeds %d bytes", it.BlockID, maxHumanTextBytes)
		}
		arity := len(it.OptionIDs)
		if other != "" {
			arity++
		}
		if !ch.Multi && arity > 1 {
			return nil, fmt.Errorf("choice %q is single-select but %d options were chosen", it.BlockID, arity)
		}
		ids := it.OptionIDs
		if ids == nil {
			ids = []string{}
		}
		p := map[string]any{"blockId": it.BlockID, "optionIds": ids}
		if other != "" {
			p["other"] = other
		}
		return mustJSON(p), nil
	case EventFeedbackCreated:
		b, topID, err := findBlock(st.Doc, it.BlockID)
		if err != nil {
			return nil, err
		}
		var ap *doc.Approval
		switch target := b.(type) {
		case *doc.Approval:
			ap = target
		case *doc.Choice:
		default:
			return nil, fmt.Errorf("block %q is a %s, not an approval or choice", it.BlockID, b.BlockType())
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		if ap != nil && !allowsFeedback(ap) {
			return nil, fmt.Errorf("block %q does not allow feedback", it.BlockID)
		}
		if visuallyEmpty(it.Text) {
			return nil, fmt.Errorf("feedback requires text")
		}
		if len(it.Text) > maxHumanTextBytes {
			return nil, fmt.Errorf("feedback exceeds %d bytes", maxHumanTextBytes)
		}
		if it.ID != "" {
			for _, f := range st.Interactions.Feedback[it.BlockID] {
				if f.ID == it.ID {
					return nil, fmt.Errorf("feedback %q already exists on block %q", it.ID, it.BlockID)
				}
			}
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
		if len(it.Text) > maxHumanTextBytes {
			return nil, fmt.Errorf("input %q text exceeds %d bytes", it.BlockID, maxHumanTextBytes)
		}
		return mustJSON(map[string]string{"blockId": it.BlockID, "text": it.Text}), nil
	case EventPackInteraction:
		pb, topID, err := requirePackBlock(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		if err := reg.ValidateInteraction(pb.Type, it.Payload); err != nil {
			return nil, err
		}
		return mustJSON(struct {
			BlockID string          `json:"blockId"`
			Payload json.RawMessage `json:"payload"`
		}{it.BlockID, it.Payload}), nil
	case EventAnnotationCreated:
		dr, topID, err := requireDraft(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		if visuallyEmpty(it.Text) {
			return nil, fmt.Errorf("annotation requires text")
		}
		if len(it.Text) > maxHumanTextBytes {
			return nil, fmt.Errorf("annotation %q text exceeds %d bytes", it.BlockID, maxHumanTextBytes)
		}
		ref, ok, err := anchor.Parse(it.Anchor)
		if err != nil {
			return nil, fmt.Errorf("annotation %q anchor: %w", it.BlockID, err)
		}
		if !ok {
			return nil, fmt.Errorf("annotation %q anchor %q: invalid anchor reference", it.BlockID, it.Anchor)
		}
		rng, _, err := anchor.FromBytes("draft", []byte(dr.Text)).Resolve(ref)
		if err != nil {
			return nil, fmt.Errorf("annotation %q anchor: %w", it.BlockID, err)
		}
		id := it.ID
		if id == "" {
			id = randomHex(4)
		}
		lines := strings.Split(dr.Text, "\n")
		quote := truncateRunes(strings.Join(lines[rng.Start-1:rng.End], "\n"), maxQuoteBytes)
		return mustJSON(map[string]string{
			"id":      id,
			"blockId": it.BlockID,
			"anchor":  anchor.FormatRange(rng.Start, rng.End, ref.Hash),
			"text":    it.Text,
			"quote":   quote,
		}), nil
	case EventAnnotationRemoved:
		_, topID, err := requireDraft(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		found := false
		for _, a := range st.Interactions.Annotations[it.BlockID] {
			if a.ID == it.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("annotation %q does not exist on block %q", it.ID, it.BlockID)
		}
		return mustJSON(map[string]string{"id": it.ID, "blockId": it.BlockID}), nil
	case EventTriageDecided:
		tr, topID, err := requireTriage(st, it.BlockID)
		if err != nil {
			return nil, err
		}
		if err := requireCurrentRound(st, it.BlockID, topID); err != nil {
			return nil, err
		}
		if len(it.Verdicts) == 0 {
			return nil, fmt.Errorf("triage %q requires at least one verdict", it.BlockID)
		}
		items := map[string]bool{}
		for _, item := range tr.Items {
			items[item.ID] = true
		}
		out := map[string]triageEntry{}
		for itemID, entry := range it.Verdicts {
			if !items[itemID] {
				return nil, fmt.Errorf("triage %q has no item %q", it.BlockID, itemID)
			}
			if !validVerdict[entry.Verdict] {
				return nil, fmt.Errorf("invalid verdict %q", entry.Verdict)
			}
			if entry.Note != "" {
				if entry.Verdict == "cleared" {
					return nil, fmt.Errorf("triage %q item %q: a cleared verdict cannot carry a note", it.BlockID, itemID)
				}
				if !allowsNotes(tr) {
					return nil, fmt.Errorf("triage %q does not allow notes", it.BlockID)
				}
				if len(entry.Note) > maxHumanTextBytes {
					return nil, fmt.Errorf("triage %q item %q note exceeds %d bytes", it.BlockID, itemID, maxHumanTextBytes)
				}
			}
			out[itemID] = triageEntry{Verdict: entry.Verdict, Note: entry.Note}
		}
		return mustJSON(map[string]any{"blockId": it.BlockID, "verdicts": out}), nil
	default:
		return nil, fmt.Errorf("unknown interaction type %q", it.Type)
	}
}

func requireApproval(st *state.State, id string) (*doc.Approval, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	ap, ok := b.(*doc.Approval)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not an approval", id, b.BlockType())
	}
	return ap, topID, nil
}

func requireChoice(st *state.State, id string) (*doc.Choice, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	ch, ok := b.(*doc.Choice)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not a choice", id, b.BlockType())
	}
	return ch, topID, nil
}

func requireInput(st *state.State, id string) (*doc.Input, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	in, ok := b.(*doc.Input)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not an input", id, b.BlockType())
	}
	return in, topID, nil
}

func requirePackBlock(st *state.State, id string) (*doc.PackBlock, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	pb, ok := b.(*doc.PackBlock)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not a pack block", id, b.BlockType())
	}
	return pb, topID, nil
}

func requireDraft(st *state.State, id string) (*doc.Draft, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	dr, ok := b.(*doc.Draft)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not a draft", id, b.BlockType())
	}
	return dr, topID, nil
}

func requireTriage(st *state.State, id string) (*doc.Triage, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	tr, ok := b.(*doc.Triage)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not a triage", id, b.BlockType())
	}
	return tr, topID, nil
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

// findBlock locates an addressable block and returns its enclosing top-level id.
func findBlock(d *doc.Doc, id string) (doc.Block, string, error) {
	loc, ok := doc.Locate(d, id)
	if !ok {
		return nil, "", fmt.Errorf("unknown block %q", id)
	}
	if loc.Visual() {
		return nil, "", pointingError(id, loc)
	}
	return loc.Block, loc.TopID, nil
}

func pointingError(id string, loc doc.Location) error {
	if loc.Kind == doc.ItemVisual {
		return fmt.Errorf("block %q is the visual of item %q on triage %q; address the triage", id, loc.ItemID, loc.TriageID)
	}
	return fmt.Errorf("block %q is the visual of option %q on choice %q; address the choice", id, loc.OptionID, loc.ChoiceID)
}

// allowsFeedback reports whether an approval permits free-text feedback;
// allowFeedback defaults to true when unset.
func allowsFeedback(ap *doc.Approval) bool {
	return ap.AllowFeedback == nil || *ap.AllowFeedback
}

// allowsNotes reports whether a triage permits per-item notes; allowNotes
// defaults to true when unset.
func allowsNotes(tr *doc.Triage) bool {
	return tr.AllowNotes == nil || *tr.AllowNotes
}

// visuallyEmpty reports whether s carries no visible content: once outer
// whitespace is trimmed, every remaining rune is a Unicode space or format (Cf)
// character — a zero-width space or joiner, say — so the string reads as blank
// to a human even though it is not "".
func visuallyEmpty(s string) bool {
	for _, r := range strings.TrimSpace(s) {
		if !unicode.IsSpace(r) && !unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// truncateRunes returns the longest prefix of s no larger than limit bytes that
// ends on a rune boundary, so an oversized quote never splits a multibyte rune.
func truncateRunes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	b := limit
	for b > 0 && !utf8.RuneStart(s[b]) {
		b--
	}
	return s[:b]
}

// requireAssetType is requireJSON's CSRF gate for the raw-bytes asset upload:
// the CLI uploader sends application/octet-stream, and neither it nor image/*
// is a CORS-"simple" type a hostile localhost page could POST preflight-free.
// The tokenless loopback CLI rules out a pair-bearer check here.
func requireAssetType(w http.ResponseWriter, r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || (mt != "application/octet-stream" && !strings.HasPrefix(mt, "image/")) {
		http.Error(w, "Content-Type must be application/octet-stream or image/*", http.StatusUnsupportedMediaType)
		return false
	}
	return true
}

// handlePutAsset content-addresses the raw request body into the asset store and
// returns its asset:<sha256> reference, rejecting a non-image body or one past
// the cap.
func (rs *restServer) handlePutAsset(w http.ResponseWriter, r *http.Request) {
	if !requireAssetType(w, r) {
		return
	}
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
