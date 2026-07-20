package daemon

import (
	"context"
	"encoding/json"
	"errors"

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
}

// PushResult is what a push reports back to the CLI.
type PushResult struct {
	Revision    int
	URL         string
	TailnetURLs []string
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

// Start creates or resumes the window's artifact, optionally seeding it with a
// document.
func (cl *Client) Start(ctx context.Context, session, scope string, pid int, fresh bool, title string, docJSON json.RawMessage) (StartResult, error) {
	reply, res, err := cl.do(ctx, OpStart, session, scope, pid, body{New: fresh, Title: title, Doc: docJSON})
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{SubjectID: reply.SubjectID, URL: res.URL, ChannelState: res.ChannelState, TailnetURLs: res.TailnetURLs}, nil
}

// Push replaces the document and returns the new revision plus the artifact's
// display URLs.
func (cl *Client) Push(ctx context.Context, session, scope string, pid int, docJSON json.RawMessage) (PushResult, error) {
	_, res, err := cl.do(ctx, OpPush, session, scope, pid, body{Doc: docJSON})
	if err != nil {
		return PushResult{}, err
	}
	return PushResult{Revision: res.Revision, URL: res.URL, TailnetURLs: res.TailnetURLs}, nil
}

// UpsertBlock inserts or replaces a single block, optionally after another.
func (cl *Client) UpsertBlock(ctx context.Context, session, scope string, pid int, blockJSON json.RawMessage, after string) error {
	_, _, err := cl.do(ctx, OpUpsertBlock, session, scope, pid, body{Block: blockJSON, After: after})
	return err
}

// RemoveBlock removes a block by id.
func (cl *Client) RemoveBlock(ctx context.Context, session, scope string, pid int, id string) error {
	_, _, err := cl.do(ctx, OpRemoveBlock, session, scope, pid, body{ID: id})
	return err
}

// Reply appends an agent reply to a block's thread.
func (cl *Client) Reply(ctx context.Context, session, scope string, pid int, blockID, md string) error {
	_, _, err := cl.do(ctx, OpReply, session, scope, pid, body{BlockID: blockID, Md: md})
	return err
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
func (cl *Client) Outcomes(ctx context.Context, session, scope string, pid int) (json.RawMessage, error) {
	_, res, err := cl.do(ctx, OpOutcomes, session, scope, pid, body{})
	return res.State, err
}
