import { useEffect, useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Approval as ApprovalBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { verdictToggle } from '../decide';
import { useDecidable } from '../keyboard';
import { Mark } from './Mark';
import { ReplyItem } from './ReplyThread';
import { DetailDisclosure } from './Detail';

export function Approval({ block, interactions }: { block: ApprovalBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const [composing, setComposing] = useState(false);
  const [draft, setDraft] = useState('');
  // Feedback is append-only and echoes back through the stream; a pending item
  // shows optimistically until its id lands in interactions.feedback.
  const [pending, setPending] = useState<{ id: string; text: string }[]>([]);
  const composerRef = useRef<HTMLTextAreaElement>(null);

  const verdict = interactions.decisions[block.id]?.verdict;
  const allowFeedback = block.allowFeedback ?? true;
  const feedback = interactions.feedback[block.id] ?? [];
  const replies = interactions.replies[block.id] ?? [];
  const stillPending = pending.filter((p) => !feedback.some((f) => f.id === p.id));

  function choose(next: 'approved' | 'rejected') {
    post({ type: 'decision.created', blockId: block.id, verdict: verdictToggle(verdict, next) });
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'approval',
    disabled: closed || readOnly,
    verdict: choose,
    clear: () => {
      if (verdict) post({ type: 'decision.created', blockId: block.id, verdict: 'cleared' });
    },
    engage: allowFeedback ? () => setComposing(true) : undefined,
  });
  useEffect(() => {
    if (composing) composerRef.current?.focus();
  }, [composing]);

  async function sendFeedback() {
    const text = draft.trim();
    if (!text) return;
    const id = crypto.randomUUID();
    setPending((prev) => [...prev, { id, text }]);
    setDraft('');
    setComposing(false);
    const ok = await post({ type: 'feedback.created', id, blockId: block.id, text });
    if (!ok) setPending((prev) => prev.filter((p) => p.id !== id));
  }

  return (
    <div className="approval" ref={ref} data-kbd-cursor={cursor || undefined} data-composing={composing || undefined}>
      {block.prompt && <p className="approval-prompt">{block.prompt}</p>}
      {block.detail && <DetailDisclosure detail={block.detail} />}
      <div className="verdict-pair" role="radiogroup" aria-label="verdict">
        <button
          type="button"
          role="radio"
          aria-checked={verdict === 'approved'}
          disabled={closed || readOnly}
          className={`verdict verdict-approve${verdict === 'approved' ? ' active' : ''}`}
          onClick={() => choose('approved')}
        >
          <span className="verdict-glyph" aria-hidden>
            {verdict === 'approved' ? <Mark kind="check" /> : '✓'}
          </span>
          Approve
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={verdict === 'rejected'}
          disabled={closed || readOnly}
          className={`verdict verdict-reject${verdict === 'rejected' ? ' active' : ''}`}
          onClick={() => choose('rejected')}
        >
          <span className="verdict-glyph" aria-hidden>
            {verdict === 'rejected' ? <Mark kind="cross" /> : '✕'}
          </span>
          Reject
        </button>
      </div>

      {allowFeedback && !(closed || readOnly) && (
        <div className="feedback-affordance">
          {composing ? (
            <div className="feedback-editor">
              <textarea
                ref={composerRef}
                value={draft}
                rows={2}
                placeholder="Add feedback for the agent…"
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (!e.repeat && (e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    e.preventDefault();
                    void sendFeedback();
                  }
                }}
              />
              <div className="feedback-actions">
                <button type="button" className="primary" onClick={sendFeedback}>
                  Send
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setComposing(false);
                    setDraft('');
                  }}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <button type="button" className="link-btn" onClick={() => setComposing(true)}>
              Add feedback
            </button>
          )}
        </div>
      )}

      {(feedback.length > 0 || stillPending.length > 0 || replies.length > 0) && (
        <div className="thread">
          {feedback.map((f) => (
            <div key={f.id} className="thread-item feedback-item">
              <span className="thread-who">you</span>
              <span className="thread-text">{f.text}</span>
            </div>
          ))}
          {stillPending.map((p) => (
            <div key={p.id} className="thread-item feedback-item pending">
              <span className="thread-who">you</span>
              <span className="thread-text">{p.text}</span>
              <span className="thread-status">sending…</span>
            </div>
          ))}
          {replies.map((r) => (
            <ReplyItem key={r.id} reply={r} />
          ))}
        </div>
      )}
    </div>
  );
}
