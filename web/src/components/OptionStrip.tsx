import { useCallback, useEffect, useState } from 'react';
import type { ReactNode, RefObject } from 'react';
import { useScrollEdges } from '../useScrollEdges';

interface CardRange {
  first: number;
  last: number;
}

export function inView(left: number, width: number, scrollLeft: number, clientWidth: number): boolean {
  return (Math.min(left + width, scrollLeft + clientWidth) - Math.max(left, scrollLeft)) * 2 >= width;
}

function cardsInView(el: HTMLElement): CardRange {
  const shown = Array.from(el.children as HTMLCollectionOf<HTMLElement>, (kid) =>
    inView(kid.offsetLeft - el.offsetLeft, kid.offsetWidth, el.scrollLeft, el.clientWidth),
  );
  return { first: shown.indexOf(true), last: shown.lastIndexOf(true) };
}

function cardStride(el: HTMLElement): number {
  const kids = el.children;
  if (kids.length > 1) {
    return (kids[1] as HTMLElement).offsetLeft - (kids[0] as HTMLElement).offsetLeft;
  }
  return (kids[0] as HTMLElement | undefined)?.offsetWidth ?? 0;
}

export function scrollBehavior(): ScrollBehavior {
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
  picked,
  containerRef,
  navActions,
  children,
}: {
  strip: boolean;
  role: 'radiogroup' | 'group';
  ariaLabel?: string;
  cardCount: number;
  picked: ReadonlySet<number>;
  containerRef: RefObject<HTMLDivElement | null>;
  navActions?: ReactNode;
  children: ReactNode;
}) {
  const { ref: edgesRef, edges } = useScrollEdges<HTMLDivElement>('x');
  const [shown, setShown] = useState<CardRange>({ first: 0, last: 0 });

  const setContainer = useCallback(
    (el: HTMLDivElement | null) => {
      edgesRef.current = el;
      containerRef.current = el;
    },
    [edgesRef, containerRef],
  );

  const measure = useCallback(() => {
    const el = edgesRef.current;
    if (!el) return;
    const next = cardsInView(el);
    setShown((prev) => (prev.first === next.first && prev.last === next.last ? prev : next));
  }, [edgesRef]);

  useEffect(() => {
    const el = edgesRef.current;
    if (!el) return;
    measure();
    el.addEventListener('scroll', measure, { passive: true });
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    for (const card of el.children) ro.observe(card);
    return () => {
      el.removeEventListener('scroll', measure);
      ro.disconnect();
    };
  }, [edgesRef, measure, cardCount]);

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
            {Array.from({ length: cardCount }, (_, i) => {
              const on = i >= shown.first && i <= shown.last;
              const isPicked = picked.has(i);
              return (
                <button
                  key={i}
                  type="button"
                  className={`strip-dot${on ? ' on' : ''}${isPicked ? ' picked' : ''}`}
                  aria-label={`Go to option ${i + 1}${isPicked ? ', selected' : ''}`}
                  onClick={() => scrollToCard(i)}
                />
              );
            })}
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
