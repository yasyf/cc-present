import { useEffect } from 'react';
import type { ReactNode } from 'react';
import type { ThreadProjection } from '../threadFeed';

// MarginRail is the desktop margin overlay. At rest it is a slim strip fixed to the
// right edge — a speech-bubble count over one tick per thread (the active block's in
// pencil, the rest graphite) — and its panel slides in when `open`. The strip is one
// button that toggles the pin latch; hover, focus, and a compose request open the
// panel without a click (see useRailOpen). The panel keeps the `.margin-rail` aside
// class so the FocusDeck compose guard and the rail styles still address it.
export function MarginRail({
  open,
  projection,
  activeId,
  total,
  onToggle,
  railRef,
  children,
}: {
  open: boolean;
  projection: ThreadProjection;
  activeId: string | null;
  total: number;
  onToggle: () => void;
  railRef: (el: HTMLElement | null) => void;
  children: ReactNode;
}) {
  // The rail is fixed and hangs below the masthead while it is visible in the
  // viewport, then reaches the viewport top as the masthead scrolls away.
  useEffect(() => {
    const header = document.querySelector('.doc-header');
    if (!header) return;
    const root = document.documentElement;
    const apply = () => root.style.setProperty('--rail-top', `${Math.max(0, header.getBoundingClientRect().bottom)}px`);
    apply();
    const observer = new ResizeObserver(apply);
    observer.observe(header);
    window.addEventListener('scroll', apply, { passive: true });
    return () => {
      observer.disconnect();
      window.removeEventListener('scroll', apply);
      root.style.removeProperty('--rail-top');
    };
  }, []);

  const entries = projection.pinned ? [projection.pinned, ...projection.feed] : projection.feed;

  return (
    <aside className="margin-rail" ref={railRef} data-open={open || undefined}>
      <button
        type="button"
        className="rail-strip"
        aria-label={total > 0 ? `Open notes — ${total} ${total === 1 ? 'comment' : 'comments'}` : 'Open notes'}
        aria-expanded={open}
        onClick={onToggle}
      >
        <span className="rail-strip-count" data-count={total > 0 || undefined}>
          <svg viewBox="0 0 24 24" width="15" height="15" fill="none" aria-hidden focusable="false">
            <path d="M4 5h16v11H8l-4 4V5z" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
          </svg>
          {total > 0 && <span className="rail-strip-num">{total}</span>}
        </span>
        <span className="rail-strip-ticks" aria-hidden>
          {entries.map((entry) => (
            <span key={entry.blockId} className="rail-tick" data-active={entry.blockId === activeId || undefined} />
          ))}
        </span>
      </button>
      <div className="rail-overlay" inert={!open || undefined}>
        {children}
      </div>
    </aside>
  );
}
