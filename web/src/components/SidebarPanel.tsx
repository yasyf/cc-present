import { useEffect, useRef } from 'react';
import { useActiveBlock } from '../activeBlock';
import { useKeyboardApi } from '../keyboard';
import { useMediaQuery } from '../useMediaQuery';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import type { ThreadEntry, ThreadKind, ThreadProjection } from '../threadFeed';

const LABELS: Record<ThreadKind, { add: string; placeholder: string }> = {
  approval: { add: 'Add feedback', placeholder: 'Add feedback for the agent…' },
  choice: { add: 'Add note', placeholder: 'Add a note for the agent…' },
};

function scrollBehavior(): ScrollBehavior {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';
}

function commentCount(entry: ThreadEntry): number {
  return entry.feedback.length + entry.replies.length;
}

function totalComments(projection: ThreadProjection): number {
  const pinned = projection.pinned ? commentCount(projection.pinned) : 0;
  return pinned + projection.feed.reduce((n, e) => n + commentCount(e), 0);
}

// SidebarPanel is the rail's body, shared by the desktop margin rail and the comments
// sheet: a MARGIN masthead over the pinned block's live thread (a lifted margin slip
// holding the thread and composer) and a document-ordered feed of every other
// conversation, each row a one-line excerpt that jumps to its block. With nothing
// noted anywhere it draws a designed vacancy instead of an empty drawer. The panel
// reads the rail breakpoint itself — the sheet (below it) carries a close control and
// drops the keyboard hint; the desktop overlay closes on Esc / blur, so it keeps
// neither. A live jump routes through the keyboard (StepNav-aware in focus mode); a
// history jump scrolls the frozen round into view by its data-block-id.
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
  const { setComposing, closePanel } = useActiveBlock();
  const isSheet = !useMediaQuery('(min-width: 1100px)');
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
  const total = totalComments(projection);
  const isEmpty = !pinned && feed.length === 0;

  return (
    <div className="rail-panel">
      <div className="rail-masthead">
        <span className="rail-mast-label">Margin</span>
        <span className="rail-mast-count" data-count={total > 0 || undefined}>
          {total}
        </span>
        {isSheet && (
          <button type="button" className="btn btn-ghost btn-sm btn-icon rail-close" aria-label="Close comments" onClick={closePanel}>
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" aria-hidden focusable="false">
              <path d="M6 6L18 18M18 6L6 18" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
            </svg>
          </button>
        )}
      </div>

      {isEmpty ? (
        <div className="rail-empty">
          <span className="mark rail-empty-mark" aria-hidden>
            <svg viewBox="0 0 24 24" fill="none" focusable="false">
              <path className="mark-stroke" pathLength={1} d="M16.2 4.2L19.8 7.8L9.8 17.8L5 19L6.2 14.2Z" />
              <path className="mark-stroke" pathLength={1} d="M13.2 7.2L16.8 10.8" />
            </svg>
          </span>
          <p className="rail-empty-lead">No margin notes yet.</p>
          <p className="rail-empty-how">
            {isSheet ? (
              'Tap the Add note chip on any block to start one here.'
            ) : (
              <>
                Use the Add note chip on any block, or press <kbd>f</kbd> on the focused one.
              </>
            )}
          </p>
        </div>
      ) : (
        <>
          {pinned && (
            <div className="rail-pinned">
              <button type="button" className="rail-pin-label" onClick={() => jump(pinned)}>
                {pinned.label}
              </button>
              <div className="rail-slip">
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
            </div>
          )}

          {feed.length > 0 && (
            <div className="rail-feed">
              <div className="rail-feed-head">{pinned ? 'Other threads' : 'Threads'}</div>
              {feed.map((entry) => (
                <button
                  key={entry.blockId}
                  type="button"
                  className="rail-feed-row"
                  data-locked={entry.locked || undefined}
                  aria-label={`Jump to ${entry.label}`}
                  onClick={() => jump(entry)}
                >
                  <span className="rail-feed-row-top">
                    <span className="rail-feed-title">{entry.label}</span>
                    <span className="rail-feed-count">{commentCount(entry)}</span>
                  </span>
                  <span className="rail-feed-excerpt">{entry.lastComment}</span>
                </button>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
