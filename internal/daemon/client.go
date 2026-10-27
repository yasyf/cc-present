package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	ccd "github.com/yasyf/cc-interact/daemon"
)

// resolveConsumer names the CLI's resolve poll, distinct from the stream
// consumers (watch, channel) so their presence cursors never contend.
const resolveConsumer = "cli"

// Client is the typed control client the CLI speaks through. It wraps the
// substrate control client and marshals each op's body.
type Client struct{ c *ccd.Client }

// NewClient wraps a substrate control client.
func NewClient(c *ccd.Client) *Client { return &Client{c: c} }

// CloseSession closes the persistent control session.
func (cl *Client) CloseSession() error { return cl.c.Close() }

// StartResult is what a start reports back to the CLI.
type StartResult struct {
	SubjectID    string
	URL          string
	ChannelState string
	TailnetURLs  []string
	Warnings     []string
}

// PushResult is what a push reports back to the CLI. Round is the round the push
// advanced into (0 when it did not open a new round).
type PushResult struct {
	Revision    int
	URL         string
	TailnetURLs []string
	Round       int
	Warnings    []string
}

func (cl *Client) do(ctx context.Context, op ccd.Op, session, scope string, pid int, b body) (ccd.Reply, result, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return ccd.Reply{}, result{}, err
	}
	reply, err := cl.c.Do(ctx, ccd.Envelope{Op: op, Session: session, ClaudePID: pid, Scope: scope, Body: raw})
	if err != nil {
		return ccd.Reply{}, result{}, err
	}
	if !reply.OK {
		return reply, result{}, errors.New(reply.Error)
	}
	var res result
	if len(reply.Body) > 0 {
		if err := json.Unmarshal(reply.Body, &res); err != nil {
			return reply, result{}, err
		}
	}
	return reply, res, nil
}

// Resolve returns the window's subject id (empty when none) and the daemon's
// HTTP port — the port the CLI needs to upload inlined image assets.
func (cl *Client) Resolve(ctx context.Context, session, scope string, pid int) (subjectID string, port int, err error) {
	reply, err := cl.c.Do(ctx, ccd.Envelope{
		Op: ccd.OpResolve, Session: session, ClaudePID: pid, Scope: scope, Consumer: resolveConsumer,
	})
	if err != nil {
		return "", 0, err
	}
	if !reply.OK {
		return "", 0, errors.New(reply.Error)
	}
	return reply.SubjectID, reply.HTTPPort, nil
}

// StartMode is how a start treats the session's existing artifact.
type StartMode int

const (
	// StartResume resumes the session's artifact, creating one when none is open.
	StartResume StartMode = iota
	// StartNew creates a fresh artifact and refuses while the session has an open one.
	StartNew
	// StartReplace closes the session's open artifact and creates a fresh one.
	StartReplace
)

// Start creates or resumes the window's artifact, optionally seeding it with a
// document.
func (cl *Client) Start(ctx context.Context, session, scope string, pid int, mode StartMode, named bool, reopen []string, title string, docJSON json.RawMessage) (StartResult, error) {
	reply, res, err := cl.do(ctx, OpStart, session, scope, pid, body{New: mode == StartNew, Replace: mode == StartReplace, Named: named, Reopen: reopen, Title: title, Doc: docJSON})
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{SubjectID: reply.SubjectID, URL: res.URL, ChannelState: res.ChannelState, TailnetURLs: res.TailnetURLs, Warnings: res.Warnings}, nil
}

// Push replaces the document and returns the new revision, the artifact's
// display URLs, and the round it advanced into. round is the caller's declared
// round intent ("", "current", or "new"); roundTitle titles a newly opened round.
func (cl *Client) Push(ctx context.Context, session, scope string, pid int, docJSON json.RawMessage, reopen []string, round, roundTitle string) (PushResult, error) {
	_, res, err := cl.do(ctx, OpPush, session, scope, pid, body{Doc: docJSON, Reopen: reopen, Round: round, Title: roundTitle})
	if err != nil {
		return PushResult{}, err
	}
	return PushResult{Revision: res.Revision, URL: res.URL, TailnetURLs: res.TailnetURLs, Round: res.Round, Warnings: res.Warnings}, nil
}

// UpsertBlock inserts or replaces a single block, optionally after another. round
// is the caller's declared round intent ("", "current", or "new") and roundTitle
// titles a newly opened round; it returns the round an advance opened (0 if none).
func (cl *Client) UpsertBlock(ctx context.Context, session, scope string, pid int, blockJSON json.RawMessage, reopen []string, after, round, roundTitle string) (int, []string, error) {
	_, res, err := cl.do(ctx, OpUpsertBlock, session, scope, pid, body{Block: blockJSON, Reopen: reopen, After: after, Round: round, Title: roundTitle})
	return res.Round, res.Warnings, err
}

// RemoveBlock removes a block by id.
func (cl *Client) RemoveBlock(ctx context.Context, session, scope string, pid int, id string) error {
	_, _, err := cl.do(ctx, OpRemoveBlock, session, scope, pid, body{ID: id})
	return err
}

// Reply appends an agent reply to a block's thread, under the feedback entry re
// names when re is set, returning the waits id of the Slack ask card the block's
// card is synced with, if any.
func (cl *Client) Reply(ctx context.Context, session, scope string, pid int, blockID, re, md string) (string, error) {
	_, res, err := cl.do(ctx, OpReply, session, scope, pid, body{BlockID: blockID, Re: re, Md: md})
	return res.Waits, err
}

// Round force-advances the round when the current one is dirty, or titles the
// current round when clean, returning the resulting current round.
func (cl *Client) Round(ctx context.Context, session, scope string, pid int, title string) (int, error) {
	_, res, err := cl.do(ctx, OpRound, session, scope, pid, body{Title: title})
	return res.Round, err
}

// Revising declares the agent's revising working set plus an optional shared
// note. Child ids resolve to their enclosing cards. No ids and no note abandons
// the announcement; no ids with a note is the doc-level drafting state.
func (cl *Client) Revising(ctx context.Context, session, scope string, pid int, blockIDs []string, note string) error {
	_, _, err := cl.do(ctx, OpRevising, session, scope, pid, body{BlockIDs: blockIDs, Note: note})
	return err
}

// Close terminally closes the window's artifact and returns its slug.
func (cl *Client) Close(ctx context.Context, session, scope string, pid int, summary string) (string, error) {
	_, res, err := cl.do(ctx, OpClose, session, scope, pid, body{Summary: summary})
	return res.Slug, err
}

// Outcomes returns the reduced state of the window's artifact as JSON.
func (cl *Client) Outcomes(ctx context.Context, session, scope string, pid int) (json.RawMessage, []string, error) {
	_, res, err := cl.do(ctx, OpOutcomes, session, scope, pid, body{})
	return res.State, res.Retired, err
}

// SlackCard puts a card synced with a Slack ask card on the window's board,
// starting the board when the window has none open.
func (cl *Client) SlackCard(ctx context.Context, session, scope string, pid int, blockJSON json.RawMessage) (StartResult, error) {
	reply, res, err := cl.do(ctx, OpSlackCard, session, scope, pid, body{Block: blockJSON})
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{SubjectID: reply.SubjectID, URL: res.URL, TailnetURLs: res.TailnetURLs}, nil
}

// SlackCardCheck refuses a card the window's board would refuse, before its
// Slack card is posted; it writes nothing.
func (cl *Client) SlackCardCheck(ctx context.Context, session, scope string, pid int, blockJSON json.RawMessage) error {
	_, _, err := cl.do(ctx, OpSlackCardCheck, session, scope, pid, body{Block: blockJSON})
	return err
}

// SlackDecided records a verdict tapped on a pr-groups row's Slack card.
func (cl *Client) SlackDecided(ctx context.Context, session, scope string, pid int, decidedJSON json.RawMessage) error {
	_, _, err := cl.do(ctx, OpSlackDecided, session, scope, pid, body{Answer: decidedJSON})
	return err
}

// SlackAnswer records an answer given on a synced card's Slack ask card.
func (cl *Client) SlackAnswer(ctx context.Context, session, scope string, pid int, answerJSON json.RawMessage) error {
	_, _, err := cl.do(ctx, OpSlackAnswer, session, scope, pid, body{Answer: answerJSON})
	return err
}

// ImportVerdicts carries verdicts given on an earlier board onto the window's
// pr-groups rows, leaving any row that already has a verdict untouched.
func (cl *Client) ImportVerdicts(ctx context.Context, session, scope string, pid int, verdictsJSON json.RawMessage) (ImportResult, error) {
	_, res, err := cl.do(ctx, OpImportVerdicts, session, scope, pid, body{Verdicts: verdictsJSON})
	if err != nil {
		return ImportResult{}, err
	}
	var out ImportResult
	if err := json.Unmarshal(res.State, &out); err != nil {
		return ImportResult{}, fmt.Errorf("decode import result: %w", err)
	}
	return out, nil
}

// MoveVerdicts re-keys recorded verdicts onto other triage items or pr-groups
// rows, clearing each source unless its move keeps it.
func (cl *Client) MoveVerdicts(ctx context.Context, session, scope string, pid int, movesJSON json.RawMessage) error {
	_, _, err := cl.do(ctx, OpMoveVerdicts, session, scope, pid, body{Verdicts: movesJSON})
	return err
}
