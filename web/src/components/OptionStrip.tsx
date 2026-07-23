import { useCallback, useEffect, useState } from 'react';
import type { ReactNode, RefObject } from 'react';
import { useScrollEdges } from '../useScrollEdges';

// activeCard maps a scroller's scrollLeft to the nearest card index given the
// card stride (card width plus gap). Pure so jsdom can test the paging math the
// layout-less DOM can't drive; the caller clamps to the card count.
export function activeCard(scrollLeft: number, cardWidth: number): number {
  if (cardWidth <= 0) return 0;
  return Math.max(0, Math.round(scrollLeft / cardWidth));
}

function cardStride(el: HTMLElement): number {
  const kids = el.children;
  if (kids.length > 1) {
    return (kids[1] as HTMLElement).offsetLeft - (kids[0] as HTMLElement).offsetLeft;
  }
  return (kids[0] as HTMLElement | undefined)?.offsetWidth ?? 0;
}

function scrollBehavior(): ScrollBehavior {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';
}

// OptionStrip is the option carousel shell: the .options scroller (a scroll-snap
// grid when `strip`, else the vertical stack) plus, when strip, a .strip-nav with
// position dots and edge-gated prev/next arrows. The option and Other cards are
// passed as children so their handlers stay in Choice. containerRef exposes the
// scroller so Choice's 1-9 keymap can reveal the chosen card.
export function OptionStrip({
  strip,
  role,
  ariaLabel,
  cardCount,
  containerRef,
  navActions,
  children,
}: {
  strip: boolean;
  role: 'radiogroup' | 'group';
  ariaLabel?: string;
  cardCount: number;
  containerRef: RefObject<HTMLDivElement | null>;
  navActions?: ReactNode;
  children: ReactNode;
}) {
  const { ref: edgesRef, edges } = useScrollEdges<HTMLDivElement>('x');
  const [active, setActive] = useState(0);

  const setContainer = useCallback(
    (el: HTMLDivElement | null) => {
      edgesRef.current = el;
      containerRef.current = el;
    },
    [edgesRef, containerRef],
  );

  const onScroll = useCallback(() => {
    const el = edgesRef.current;
    if (!el || el.children.length === 0) return;
    setActive(Math.min(activeCard(el.scrollLeft, cardStride(el)), el.children.length - 1));
  }, [edgesRef]);

  useEffect(() => {
    const el = edgesRef.current;
    if (!el) return;
    onScroll();
    el.addEventListener('scroll', onScroll, { passive: true });
    return () => el.removeEventListener('scroll', onScroll);
  }, [edgesRef, onScroll]);

  const scrollToCard = useCallback(
    (i: number) => {
      const card = edgesRef.current?.children[i] as HTMLElement | undefined;
      card?.scrollIntoView({ inline: 'nearest', block: 'nearest', behavior: scrollBehavior() });
    },
    [edgesRef],
  );

  const nudge = useCallback(
    (dir: 1 | -1) => {
      const el = edgesRef.current;
      if (!el) return;
      el.scrollBy({ left: dir * cardStride(el), behavior: scrollBehavior() });
    },
    [edgesRef],
  );

  return (
    <>
      <div
        ref={setContainer}
        className="options"
        data-strip={strip || undefined}
        data-edge-start={(strip && edges.atStart) || undefined}
        data-edge-end={(strip && edges.atEnd) || undefined}
        role={role}
        aria-label={ariaLabel}
      >
        {children}
      </div>
      {strip && (
        <div className="strip-nav">
          <div className="strip-dots">
            {Array.from({ length: cardCount }, (_, i) => (
              <button
                key={i}
                type="button"
                className={`strip-dot${i === active ? ' on' : ''}`}
                aria-label={`Go to option ${i + 1}`}
                aria-current={i === active || undefined}
                onClick={() => scrollToCard(i)}
              />
            ))}
          </div>
          {navActions}
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon"
            aria-label="Previous options"
            disabled={edges.atStart}
            onClick={() => nudge(-1)}
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" aria-hidden focusable="false">
              <path d="M15 5L8 12L15 19" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon"
            aria-label="Next options"
            disabled={edges.atEnd}
            onClick={() => nudge(1)}
          >
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" aria-hidden focusable="false">
              <path d="M9 5L16 12L9 19" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        </div>
      )}
    </>
  );
}
