import type { Reply } from '../events';
import { renderMarkdown } from '../markdown';
import { Clamped } from './Clamped';

// ReplyItem is one agent reply row: the accent-tinted, clamped markdown bubble
// keyed by the reply's id in its enclosing thread.
export function ReplyItem({ reply }: { reply: Reply }) {
  return (
    <div className="thread-item reply-item">
      <span className="thread-who">agent</span>
      <Clamped html={renderMarkdown(reply.md)} lines={4} className="thread-text prose" />
    </div>
  );
}

// ReplyThread renders the agent's replies to a block; it collapses to nothing
// when the block has none, so callers can mount it unconditionally.
export function ReplyThread({ replies }: { replies: Reply[] }) {
  if (replies.length === 0) return null;
  return (
    <div className="thread">
      {replies.map((r) => (
        <ReplyItem key={r.id} reply={r} />
      ))}
    </div>
  );
}
