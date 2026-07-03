import { useState } from 'react';
import type { Approval as ApprovalBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { renderMarkdown } from '../markdown';

export function Approval({ block, interactions }: { block: ApprovalBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const [composing, setComposing] = useState(false);
  const [draft, setDraft] = useState('');

  const verdict = interactions.decisions[block.id]?.verdict;
  const allowFeedback = block.allowFeedback ?? true;
  const feedback = interactions.feedback[block.id] ?? [];
  const replies = interactions.replies[block.id] ?? [];

  function choose(next: 'approved' | 'rejected') {
    post({ type: 'decision.created', blockId: block.id, verdict: verdict === next ? 'cleared' : next });
  }

  function sendFeedback() {
    const text = draft.trim();
    if (!text) return;
    post({ type: 'feedback.created', id: crypto.randomUUID(), blockId: block.id, text });
    setDraft('');
    setComposing(false);
  }

  return (
    <div className="approval">
      {block.prompt && <p className="approval-prompt">{block.prompt}</p>}
      <div className="verdict-pair" role="radiogroup" aria-label="verdict">
        <button
          type="button"
          role="radio"
          aria-checked={verdict === 'approved'}
          disabled={closed}
          className={`verdict verdict-approve${verdict === 'approved' ? ' active' : ''}`}
          onClick={() => choose('approved')}
        >
          Approve
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={verdict === 'rejected'}
          disabled={closed}
          className={`verdict verdict-reject${verdict === 'rejected' ? ' active' : ''}`}
          onClick={() => choose('rejected')}
        >
          Reject
        </button>
      </div>

      {allowFeedback && !closed && (
        <div className="feedback-affordance">
          {composing ? (
            <div className="feedback-editor">
              <textarea
                value={draft}
                rows={2}
                placeholder="Add feedback for the agent…"
                onChange={(e) => setDraft(e.target.value)}
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

      {(feedback.length > 0 || replies.length > 0) && (
        <div className="thread">
          {feedback.map((f) => (
            <div key={f.id} className="thread-item feedback-item">
              <span className="thread-who">you</span>
              <span className="thread-text">{f.text}</span>
            </div>
          ))}
          {replies.map((r) => (
            <div key={r.id} className="thread-item reply-item">
              <span className="thread-who">agent</span>
              <span
                className="thread-text prose"
                dangerouslySetInnerHTML={{ __html: renderMarkdown(r.md) }}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
