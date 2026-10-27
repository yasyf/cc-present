import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react';
import { draftStore, useDraft } from '../drafts';
import { usePresent } from '../present';
import { uuid } from '../uuid';
import { Button } from './Button';
import { ReplyItem } from './ReplyThread';
import { useThreadHost } from './threadHost';
import type { Feedback, Reply } from '../events';

// FeedbackHandle lets a host block raise the composer imperatively (the `f` key).
export interface FeedbackHandle {
  open: () => void;
}

export interface FeedbackThreadProps {
  blockId: string;
  feedback: Feedback[];
  replies: Reply[];
  // Hides the composer (the round is closed or superseded); the thread still shows.
  locked: boolean;
  addLabel: string;
  placeholder: string;
  onComposingChange?: (composing: boolean) => void;
}

// FeedbackThread is the shared note channel for approvals, choices, and interactive
// pack blocks: an inline composer behind an "Add …" affordance, then the append-only
// thread of sent notes, each with the agent's replies to it beneath, then the
// replies that answer no note in this thread. It reports composing to the host only while the draft
// holds text — an open, empty composer must not latch the rail — while the
// data-composing attribute tracks the open composer for the deck's advance guard.
export const FeedbackThread = forwardRef<FeedbackHandle, FeedbackThreadProps>(function FeedbackThread(
  { blockId, feedback, replies, locked, addLabel, placeholder, onComposingChange },
  ref,
) {
  const { post } = usePresent();
  const rail = useThreadHost() === 'rail';
  const [open, setOpen] = useState(false);
  const draft = useDraft<string>(blockId, 'note');
  const text = draft ?? '';
  const composing = open || draft !== undefined;
  // Feedback is append-only and echoes back through the stream; a pending item
  // shows optimistically until its id lands in interactions.feedback.
  const [pending, setPending] = useState<{ id: string; text: string }[]>([]);
  const composerRef = useRef<HTMLTextAreaElement>(null);

  useImperativeHandle(
    ref,
    () => ({
      open: () => {
        if (open) {
          if (rail) composerRef.current?.focus();
          return;
        }
        setOpen(true);
      },
    }),
    [open, rail],
  );
  useEffect(() => {
    onComposingChange?.(composing && text.trim().length > 0);
  }, [composing, text, onComposingChange]);
  // Unmounting mid-draft (the sheet closing over an open composer) must release the
  // host latch, or the desktop rail inherits a composing=true it can never clear.
  useEffect(
    () => () => {
      onComposingChange?.(false);
    },
    [onComposingChange],
  );
  useEffect(() => {
    if (open) composerRef.current?.focus();
  }, [open]);

  const stillPending = pending.filter((p) => !feedback.some((f) => f.id === p.id));
  const loose = replies.filter((r) => !feedback.some((f) => f.id === r.re));

  async function send() {
    const body = text.trim();
    if (!body) return;
    const id = uuid();
    const epoch = draftStore.epoch;
    setPending((prev) => [...prev, { id, text: body }]);
    draftStore.clear(blockId, 'note');
    setOpen(false);
    const ok = await post({ type: 'feedback.created', id, blockId, text: body });
    if (ok) return;
    setPending((prev) => prev.filter((p) => p.id !== id));
    if (draftStore.epoch !== epoch) return;
    const typed = draftStore.get<string>(blockId, 'note');
    draftStore.set(blockId, 'note', typed ? `${body}\n${typed}` : body);
  }

  return (
    <>
      {!locked && (
        <div className="feedback-affordance" data-composing={composing || undefined}>
          {composing ? (
            <div className="feedback-editor">
              <textarea
                ref={composerRef}
                className="field"
                value={text}
                rows={2}
                placeholder={placeholder}
                onFocus={() => setOpen(true)}
                onChange={(e) => {
                  if (e.target.value) draftStore.set(blockId, 'note', e.target.value);
                  else draftStore.clear(blockId, 'note');
                }}
                onKeyDown={(e) => {
                  if (!e.repeat && (e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    e.preventDefault();
                    void send();
                  }
                }}
              />
              <div className="feedback-actions">
                <Button variant="primary" size="sm" onClick={send}>
                  Send
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    draftStore.clear(blockId, 'note');
                    setOpen(false);
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <button type="button" className="link-btn" onClick={() => setOpen(true)}>
              {addLabel}
            </button>
          )}
        </div>
      )}

      {(feedback.length > 0 || stillPending.length > 0 || replies.length > 0) && (
        <div className="thread">
          {feedback.map((f) => (
            <NoteThread key={f.id} note={f} replies={replies.filter((r) => r.re === f.id)} />
          ))}
          {stillPending.map((p) => (
            <div key={p.id} className="thread-item feedback-item pending">
              <span className="thread-who">you</span>
              <span className="thread-text">{p.text}</span>
              <span className="thread-status">sending…</span>
            </div>
          ))}
          {loose.map((r) => (
            <ReplyItem key={r.id} reply={r} />
          ))}
        </div>
      )}
    </>
  );
});

const SHOWN_REPLIES = 2;

// NoteThread is one sent note with the agent's replies to it indented beneath,
// newest last; past two replies the older ones fold behind a "Show N more".
function NoteThread({ note, replies }: { note: Feedback; replies: Reply[] }) {
  const [expanded, setExpanded] = useState(false);
  const hidden = expanded ? 0 : Math.max(0, replies.length - SHOWN_REPLIES);
  return (
    <div className="note-thread">
      <div className="thread-item feedback-item">
        <span className="thread-who">you</span>
        <span className="thread-text">{note.text}</span>
      </div>
      {replies.length > 0 && (
        <div className="note-replies">
          {hidden > 0 && (
            <button type="button" className="link-btn note-replies-more" onClick={() => setExpanded(true)}>
              Show {hidden} more
            </button>
          )}
          {replies.slice(hidden).map((r) => (
            <ReplyItem key={r.id} reply={r} />
          ))}
        </div>
      )}
    </div>
  );
}
