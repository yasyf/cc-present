import { useState } from 'react';
import type { Feedback, Reply } from '../events';
import { renderMarkdown } from '../markdown';

// ReplyItem is one agent reply row: the accent-tinted markdown bubble keyed by
// the reply's id in its enclosing thread.
export function ReplyItem({ reply }: { reply: Reply }) {
  return (
    <div className="thread-item reply-item">
      <span className="thread-who">agent</span>
      <div className="thread-text prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(reply.md) }} />
    </div>
  );
}

const SHOWN_REPLIES = 2;

// ReplyThread renders the agent's replies to a block, each under the note it
// answers, newest last; past two replies the older ones fold behind a "Show N
// earlier". It collapses to nothing when the block has none, so callers can
// mount it unconditionally.
export function ReplyThread({ replies, notes }: { replies: Reply[]; notes: Feedback[] }) {
  const [expanded, setExpanded] = useState(false);
  if (replies.length === 0) return null;
  const hidden = expanded ? 0 : Math.max(0, replies.length - SHOWN_REPLIES);
  const shown = replies.slice(hidden);
  return (
    <div className="thread">
      {hidden > 0 && (
        <button type="button" className="link-btn note-replies-more" onClick={() => setExpanded(true)}>
          Show {hidden} earlier
        </button>
      )}
      {shown.map((r, i) => {
        const note = shown.slice(0, i).some((prev) => prev.re === r.re) ? undefined : notes.find((n) => n.id === r.re);
        return (
          <div key={r.id} className="note-thread">
            {note && (
              <div className="thread-item feedback-item">
                <span className="thread-who">you</span>
                <span className="thread-text">{note.text}</span>
              </div>
            )}
            <ReplyItem reply={r} />
          </div>
        );
      })}
    </div>
  );
}
