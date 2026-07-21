import { useEffect, useRef } from 'react';
import { useActiveBlock } from '../activeBlock';
import { useKeyboardApi } from '../keyboard';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import { ReplyItem } from './ReplyThread';
import type { ThreadEntry, ThreadKind, ThreadProjection } from '../threadFeed';

const LABELS: Record<ThreadKind, { add: string; placeholder: string }> = {
  approval: { add: 'Add feedback', placeholder: 'Add feedback for the agent…' },
  choice: { add: 'Add note', placeholder: 'Add a note for the agent…' },
};

function scrollBehavior(): ScrollBehavior {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';
}

// SidebarPanel is the rail's body, shared by the desktop margin rail and the
// comments sheet: the pinned block's live thread and composer on top, then a
// read-only feed of every other conversation with a jump-to-block control. A live
// jump routes through the keyboard (StepNav-aware in focus mode); a history jump
// scrolls the frozen round into view by its data-block-id.
export function SidebarPanel({
  projection,
  composeEpoch,
  openComposerOnMount,
  onJumped,
}: {
  projection: ThreadProjection;
  composeEpoch: number;
  openComposerOnMount: boolean;
  onJumped?: () => void;
}) {
  const kbd = useKeyboardApi();
  const { setComposing } = useActiveBlock();
  const feedbackRef = useRef<FeedbackHandle>(null);

  // Mount-only: the sheet re-mounts this panel when it opens, so a compose that
  // opened it raises the composer here rather than through the epoch below.
  const mountCompose = useRef(openComposerOnMount);
  useEffect(() => {
    if (mountCompose.current) feedbackRef.current?.open();
  }, []);
  const firstEpoch = useRef(composeEpoch);
  useEffect(() => {
    if (composeEpoch !== firstEpoch.current) {
      firstEpoch.current = composeEpoch;
      feedbackRef.current?.open();
    }
  }, [composeEpoch]);

  function jump(entry: ThreadEntry) {
    if (entry.locked) {
      const target = [...document.querySelectorAll<HTMLElement>('[data-block-id]')].find(
        (el) => el.dataset.blockId === entry.blockId,
      );
      target?.scrollIntoView({ block: 'center', behavior: scrollBehavior() });
    } else {
      kbd.jumpTo(entry.blockId);
    }
    onJumped?.();
  }

  const { pinned, feed } = projection;

  return (
    <div className="rail-panel">
      <div className="rail-head">Margin</div>
      {pinned ? (
        <div className="rail-pinned">
          <button type="button" className="rail-pin-label" onClick={() => jump(pinned)}>
            {pinned.label}
          </button>
          <FeedbackThread
            key={pinned.blockId}
            ref={feedbackRef}
            blockId={pinned.blockId}
            feedback={pinned.feedback}
            replies={pinned.replies}
            locked={pinned.locked}
            addLabel={LABELS[pinned.kind].add}
            placeholder={LABELS[pinned.kind].placeholder}
            onComposingChange={setComposing}
          />
        </div>
      ) : (
        <p className="rail-empty">Select a block to leave a note in the margin.</p>
      )}

      {feed.length > 0 && (
        <div className="rail-feed">
          <div className="rail-feed-head">Other threads</div>
          {feed.map((entry) => (
            <div key={entry.blockId} className="rail-feed-row" data-locked={entry.locked || undefined}>
              <button type="button" className="rail-feed-jump" onClick={() => jump(entry)}>
                {entry.label}
              </button>
              <div className="rail-feed-thread">
                {entry.feedback.map((f) => (
                  <div key={f.id} className="thread-item feedback-item">
                    <span className="thread-who">you</span>
                    <span className="thread-text">{f.text}</span>
                  </div>
                ))}
                {entry.replies.map((r) => (
                  <ReplyItem key={r.id} reply={r} />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
