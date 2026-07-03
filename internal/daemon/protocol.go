// Package daemon wires cc-present's domain onto cc-interact's daemon substrate:
// it builds the daemon.Config (presence via channel.Connectivity, no edit gate,
// raw-cwd scope), registers the artifact ops as handlers, and mounts the REST
// plane (human interactions + the content-addressed asset store). Client is the
// typed control client the CLI speaks through.
package daemon

import (
	"encoding/json"

	ccd "github.com/yasyf/cc-interact/daemon"
)

// Artifact control-plane operations, registered on the daemon by name.
const (
	OpStart       ccd.Op = "start"
	OpPush        ccd.Op = "push"
	OpUpsertBlock ccd.Op = "upsert-block"
	OpRemoveBlock ccd.Op = "remove-block"
	OpReply       ccd.Op = "reply"
	OpClose       ccd.Op = "close"
	OpOutcomes    ccd.Op = "outcomes"
)

// Event types appended to a subject's log. The agent-side ops append the agent
// types; the REST interaction handler appends the human types. The reducer in
// package state is the single authority on how each folds into document and
// interaction state.
const (
	EventDocReplaced     = "doc.replaced"
	EventBlockUpserted   = "block.upserted"
	EventBlockRemoved    = "block.removed"
	EventReplyCreated    = "reply.created"
	EventPresentClosed   = "present.closed"
	EventDecisionCreated = "decision.created"
	EventChoiceSelected  = "choice.selected"
	EventFeedbackCreated = "feedback.created"
	EventInputSubmitted  = "input.submitted"
	EventSubmit          = "submit"
)

// body is the domain payload carried in an Envelope.Body; each handler reads
// only the fields its op uses. The window (session, pid) and scope ride on the
// envelope itself.
type body struct {
	New     bool            `json:"new,omitempty"`     // start
	Title   string          `json:"title,omitempty"`   // start
	Doc     json.RawMessage `json:"doc,omitempty"`     // start | push
	Block   json.RawMessage `json:"block,omitempty"`   // upsert-block
	After   string          `json:"after,omitempty"`   // upsert-block
	ID      string          `json:"id,omitempty"`      // remove-block | reply
	BlockID string          `json:"blockId,omitempty"` // reply
	Md      string          `json:"md,omitempty"`      // reply
	Summary string          `json:"summary,omitempty"` // close
}

// result is the domain payload a handler returns in Reply.Body. Envelope-level
// outputs (subject id, http port) ride on the Reply itself.
type result struct {
	URL          string          `json:"url,omitempty"`          // start
	Slug         string          `json:"slug,omitempty"`         // start | close
	ChannelState string          `json:"channelState,omitempty"` // start
	Revision     int             `json:"revision,omitempty"`     // push
	State        json.RawMessage `json:"state,omitempty"`        // outcomes
}

func decodeBody(raw json.RawMessage) body {
	var b body
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &b)
	}
	return b
}

func okReply(r result) ccd.Reply {
	raw, _ := json.Marshal(r)
	return ccd.Reply{OK: true, Body: raw}
}

func errReply(msg string) ccd.Reply { return ccd.Reply{OK: false, Error: msg} }
