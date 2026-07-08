package daemon

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yasyf/cc-interact/channel"
	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
	"github.com/yasyf/cc-interact/paths"
	"github.com/yasyf/cc-interact/subject"

	"github.com/yasyf/cc-present/internal/assets"
	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

const (
	appName      = "cc-present"
	statusOpen   = "open"
	statusClosed = "closed"

	// channelConsumer is the stream-consumer name the channel server registers
	// under; channelState keys presence to it.
	channelConsumer = "channel"
	// channelPollWindow is how recent a channel resolve poll must be to count as
	// presence; it only distinguishes pending from inactive.
	channelPollWindow = 3 * time.Second
)

// lifecycle names the subject statuses the resolver writes: a fresh artifact is
// born open; a fresh start closes the window's prior artifact.
var lifecycle = subject.Lifecycle{Initial: statusOpen, Closed: statusClosed}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

// BuildServer composes the cc-present daemon: presence via channel.Connectivity,
// no edit gate, raw-cwd scope. It registers the artifact ops and mounts the REST
// plane, returning a Server the caller Serves.
func BuildServer(p paths.Paths, version string) (*ccd.Server, error) {
	c := channel.Connectivity{}
	s, err := ccd.New(ccd.Config{
		AppName:        appName,
		Paths:          p,
		Version:        version,
		ActiveStatuses: []string{statusOpen},
		// c.Type() (not c.EventType) so the SSE plane filters the same presence type
		// the hooks emit — correct even for the Connectivity zero value.
		PresenceEventType: c.Type(),
		OnPresenceChange:  c.OnPresenceChange,
		BootReconcile:     c.BootReconcile,
		// Gate nil → no edit gate; ScopeResolve nil → raw cwd; Migrate nil → no
		// domain tables (document and interaction state are a pure reduction of the
		// event log).
	})
	if err != nil {
		return nil, err
	}
	ast, err := assets.New(filepath.Join(p.StateDir(), "assets"))
	if err != nil {
		return nil, err
	}
	s.Register(OpStart, handleStart)
	s.Register(OpPush, handlePush)
	s.Register(OpUpsertBlock, handleUpsertBlock)
	s.Register(OpRemoveBlock, handleRemoveBlock)
	s.Register(OpReply, handleReply)
	s.Register(OpRound, handleRound)
	s.Register(OpClose, func(hc ccd.HandlerCtx) ccd.Reply { return handleClose(hc, ast) })
	s.Register(OpOutcomes, handleOutcomes)
	mountREST(s, ast)
	return s, nil
}

// Serve builds the daemon and runs it until ctx is cancelled.
func Serve(ctx context.Context, p paths.Paths, version string) error {
	s, err := BuildServer(p, version)
	if err != nil {
		return err
	}
	return s.Serve(ctx)
}

// handleStart creates or resumes the window's artifact subject, optionally
// appending an initial doc.replaced, and reports the URL and channel state. A
// prior close is terminal, so a resume that would land on a closed subject is
// forced fresh instead.
func handleStart(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	var d *doc.Doc
	if len(b.Doc) > 0 {
		d = &doc.Doc{}
		if err := json.Unmarshal(b.Doc, d); err != nil {
			return errReply("decode doc: " + err.Error())
		}
		if err := d.Validate(); err != nil {
			return errReply(err.Error())
		}
	}
	titleBase := b.Title
	if d != nil {
		titleBase = d.Title
	}
	fresh := b.New
	if !fresh {
		if cur, ok, err := hc.Subjects.Find(hc.Ctx, hc.Window, hc.Scope); err != nil {
			return errReply(err.Error())
		} else if ok && cur.Status == statusClosed {
			fresh = true
		}
	}
	slug := slugify(titleBase) + "--" + randomHex(4)
	sub, _, err := hc.Subjects.Start(hc.Ctx, hc.Window, hc.Scope, slug, lifecycle, fresh)
	if err != nil {
		return errReply(err.Error())
	}
	if d != nil {
		rev, err := nextRevision(hc.Ctx, hc.DB, sub.ID)
		if err != nil {
			return errReply(err.Error())
		}
		if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
			SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
			Payload: docReplacedPayload(b.Doc, rev),
		}); err != nil {
			return errReply(err.Error())
		}
	}
	cs := channelState(hc.Activity, sub.ID, hc.Scope, hc.Window.ClaudePID)
	res := result{URL: artifactURL(hc.HTTPPort, sub.Slug), Slug: sub.Slug, ChannelState: cs}
	raw, _ := json.Marshal(res)
	return ccd.Reply{OK: true, SubjectID: sub.ID, HTTPPort: hc.HTTPPort, Body: raw}
}

// handlePush validates the document, then replaces it with an incremented
// revision. A closed artifact rejects the push.
func handlePush(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	d := &doc.Doc{}
	if err := json.Unmarshal(b.Doc, d); err != nil {
		return errReply("decode doc: " + err.Error())
	}
	if err := d.Validate(); err != nil {
		return errReply(err.Error())
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	rev, err := nextRevision(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
		Payload: docReplacedPayload(b.Doc, rev),
	}); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{Revision: rev})
}

// handleUpsertBlock inserts or replaces a single top-level block. An unknown
// block type or a block that fails per-type validation is rejected; a closed
// artifact rejects the upsert.
func handleUpsertBlock(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	blk, err := doc.DecodeBlock(b.Block)
	if err != nil {
		return errReply(err.Error())
	}
	if err := validateBlock(blk); err != nil {
		return errReply(err.Error())
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventBlockUpserted,
		Payload: blockUpsertedPayload(b.Block, b.After),
	}); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{})
}

// handleRemoveBlock removes a top-level block by id. A closed artifact rejects
// the removal.
func handleRemoveBlock(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	if b.ID == "" {
		return errReply("remove-block requires an id")
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventBlockRemoved,
		Payload: mustJSON(map[string]string{"id": b.ID}),
	}); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{})
}

// handleReply appends an agent reply to a block's thread. A closed artifact
// rejects the reply.
func handleReply(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	if b.BlockID == "" {
		return errReply("reply requires a blockId")
	}
	if b.Md == "" {
		return errReply("reply requires md")
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	id := b.ID
	if id == "" {
		id = randomHex(4)
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventReplyCreated,
		Payload: mustJSON(map[string]string{"id": id, "blockId": b.BlockID, "md": b.Md}),
	}); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{})
}

// handleRound appends an agent round.started under the agent origin, then
// reduces the log to report the resulting current round. A dirty round (a live
// top-level block stamped with the current round) force-advances; a clean one is
// merely retitled. A closed artifact rejects the round. round.started carries no
// revision, so unlike a doc write it never bumps the revision counter.
func handleRound(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	payload := json.RawMessage("{}")
	if b.Title != "" {
		payload = mustJSON(map[string]string{"title": b.Title})
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventRoundStarted, Payload: payload,
	}); err != nil {
		return errReply(err.Error())
	}
	events, err := loadEvents(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	st, err := state.Reduce(events)
	if err != nil {
		return errReply(err.Error())
	}
	return okReply(result{Round: st.Rounds.Current})
}

// handleClose terminally closes the window's artifact: it appends present.closed
// under the system origin and flips the subject to closed. A re-close is
// rejected. The close is a system lifecycle event like channel.changed, not an
// agent write, so the agent-side watch and channel (which stream
// exclude_origin=agent) still receive it and terminate on it. Once the subject
// is closed, its assets become collectable, so the close drives a GC sweep of
// the asset store; a sweep failure surfaces in the reply even though the close
// itself has already taken effect.
func handleClose(hc ccd.HandlerCtx, ast *assets.Store) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	sub, ok, err := hc.Subjects.Find(hc.Ctx, hc.Window, hc.Scope)
	if err != nil {
		return errReply(err.Error())
	}
	if !ok {
		return errReply("no cc-present artifact for this scope; run `cc-present start` first")
	}
	if sub.Status == statusClosed {
		return errReply(fmt.Sprintf("artifact %s is already closed", sub.Slug))
	}
	payload := json.RawMessage("{}")
	if b.Summary != "" {
		payload = mustJSON(map[string]string{"summary": b.Summary})
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginSystem, Type: EventPresentClosed, Payload: payload,
	}); err != nil {
		return errReply(err.Error())
	}
	if err := hc.Subjects.Store.SetStatus(hc.Ctx, sub.ID, statusClosed); err != nil {
		return errReply(err.Error())
	}
	if err := gcAssets(hc.Ctx, hc.DB, ast); err != nil {
		return errReply(fmt.Sprintf("artifact %s closed, but asset GC failed: %v", sub.Slug, err))
	}
	return okReply(result{Slug: sub.Slug})
}

// handleOutcomes reduces the artifact's full event log into the document plus
// the keyed human interactions and returns it as JSON — the post-submit drain.
func handleOutcomes(hc ccd.HandlerCtx) ccd.Reply {
	sub, ok, err := hc.Subjects.Find(hc.Ctx, hc.Window, hc.Scope)
	if err != nil {
		return errReply(err.Error())
	}
	if !ok {
		return errReply("no cc-present artifact for this scope; run `cc-present start` first")
	}
	events, err := loadEvents(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	st, err := state.Reduce(events)
	if err != nil {
		return errReply(err.Error())
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return errReply(err.Error())
	}
	return okReply(result{State: raw})
}

// resolveOpen finds the window's artifact and rejects a closed one, so a
// mutating op never appends past present.closed.
func resolveOpen(hc ccd.HandlerCtx) (subject.Subject, error) {
	sub, ok, err := hc.Subjects.Find(hc.Ctx, hc.Window, hc.Scope)
	if err != nil {
		return subject.Subject{}, err
	}
	if !ok {
		return subject.Subject{}, errors.New("no cc-present artifact for this scope; run `cc-present start` first")
	}
	if sub.Status == statusClosed {
		return subject.Subject{}, fmt.Errorf("artifact %s is closed", sub.Slug)
	}
	return sub, nil
}

// channelState classifies this window's channel route: active once the model has
// proven a delivered channel tag while a channel consumer is attached, pending
// while attached or recently polling but unproven, inactive otherwise. The pid
// key keeps one window's presence from lighting up another's start.
func channelState(act *ccd.Activity, subjectID, scope string, pid int) string {
	attached := act.Attached(subjectID, channelConsumer, pid)
	if attached && act.Proven(pid) {
		return "active"
	}
	if attached || act.PolledSince(scope, channelConsumer, pid, channelPollWindow) {
		return "pending"
	}
	return "inactive"
}

// validateBlock validates a single block by folding it into a minimal document,
// reusing the per-type field, choice-option, and card-nesting rules.
func validateBlock(b doc.Block) error {
	d := &doc.Doc{Version: 1, Title: "block", Blocks: []doc.Block{b}}
	return d.Validate()
}

func artifactURL(port int, slug string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/p/%s", port, slug)
}

func docReplacedPayload(rawDoc json.RawMessage, revision int) json.RawMessage {
	return mustJSON(struct {
		Doc      json.RawMessage `json:"doc"`
		Revision int             `json:"revision"`
	}{rawDoc, revision})
}

func blockUpsertedPayload(rawBlock json.RawMessage, after string) json.RawMessage {
	p := struct {
		Block json.RawMessage `json:"block"`
		After *string         `json:"after,omitempty"`
	}{Block: rawBlock}
	if after != "" {
		p.After = &after
	}
	return mustJSON(p)
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal payload: %v", err))
	}
	return raw
}

// appendEvent injects the event's Type into its payload as a self-describing
// "type" field, then appends it through the daemon's Append chokepoint. Every
// wire frame is thus self-describing: the browser SPA, the agent-side watch, and
// the channel all read the event discriminant out of the payload JSON, matching
// the framework's Connectivity convention (channel.changed carries its own type
// too). It is the single append path for every domain event.
func appendEvent(ctx context.Context, appendFn ccd.AppendFunc, ev *ccevent.Event) (int64, error) {
	ev.Payload = injectType(ev.Type, ev.Payload)
	return appendFn(ctx, ev)
}

// injectType returns payload with a top-level "type" field set to eventType. The
// payload must be a JSON object; every domain payload is.
func injectType(eventType string, payload json.RawMessage) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		panic(fmt.Sprintf("event payload is not a JSON object: %v", err))
	}
	obj["type"] = mustJSON(eventType)
	return mustJSON(obj)
}

// nextRevision is the revision the next doc.replaced takes: one past the count
// of doc.replaced events already in the log.
func nextRevision(ctx context.Context, db *sql.DB, subjectID string) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events WHERE subject_id=? AND type=?`, subjectID, EventDocReplaced).Scan(&n); err != nil {
		return 0, fmt.Errorf("count doc.replaced: %w", err)
	}
	return n + 1, nil
}

// loadEvents reads a subject's full log, oldest first, as reducer events.
func loadEvents(ctx context.Context, db *sql.DB, subjectID string) ([]state.Event, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT origin, type, seq, payload FROM events WHERE subject_id=? ORDER BY seq ASC`, subjectID)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []state.Event
	for rows.Next() {
		var (
			e       state.Event
			payload string
		)
		if err := rows.Scan(&e.Origin, &e.Type, &e.Seq, &payload); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Payload = json.RawMessage(payload)
		out = append(out, e)
	}
	return out, rows.Err()
}

// slugify lowercases s and collapses runs of non-alphanumerics to a single
// hyphen, falling back to "present" when nothing survives.
func slugify(s string) string {
	out := strings.Trim(slugStrip.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(out) > 48 {
		out = strings.Trim(out[:48], "-")
	}
	if out == "" {
		return "present"
	}
	return out
}

// randomHex returns n random bytes as 2n lowercase hex chars.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("read random: %v", err))
	}
	return hex.EncodeToString(b)
}
