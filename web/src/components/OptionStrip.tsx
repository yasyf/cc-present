import { useCallback, useEffect, useState } from 'react';
import type { CSSProperties, ReactNode, RefObject } from 'react';
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

export function OptionStrip({
  strip,
  role,
  ariaLabel,
  cardCount,
  picked,
  containerRef,
  navActions,
  other,
  children,
}: {
  strip: boolean;
  role: 'radiogroup' | 'group';
  ariaLabel?: string;
  cardCount: number;
  picked: ReadonlySet<number>;
  containerRef: RefObject<HTMLDivElement | null>;
  navActions?: ReactNode;
  other: ReactNode;
  children: ReactNode;
}) {
  const { ref: edgesRef, edges } = useScrollEdges<HTMLDivElement>('x');
  const [shown, setShown] = useState<CardRange>({ first: 0, last: 0 });

  const setContainer = useCallback(
    (el: HTMLDivElement | null) => {
      edgesRef(el);
      containerRef.current = el;
    },
    [edgesRef, containerRef],
  );

  const measure = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const next = cardsInView(el);
    setShown((prev) => (prev.first === next.first && prev.last === next.last ? prev : next));
  }, [containerRef]);

  useEffect(() => {
    const el = containerRef.current;
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
  }, [containerRef, measure, cardCount]);

  const scrollToCard = useCallback(
    (i: number) => {
      const card = containerRef.current?.children[i] as HTMLElement | undefined;
      card?.scrollIntoView({ inline: 'nearest', block: 'nearest', behavior: scrollBehavior() });
    },
    [containerRef],
  );

  const nudge = useCallback(
    (dir: 1 | -1) => {
      const el = containerRef.current;
      if (!el) return;
      el.scrollBy({ left: dir * cardStride(el), behavior: scrollBehavior() });
    },
    [containerRef],
  );

  const scroller = (
    <div
      ref={setContainer}
      className="options"
      style={strip ? ({ '--strip-n': cardCount } as CSSProperties) : undefined}
      data-strip={strip || undefined}
      data-edge-start={(strip && edges.atStart) || undefined}
      data-edge-end={(strip && edges.atEnd) || undefined}
      role={strip ? undefined : role}
      aria-label={strip ? undefined : ariaLabel}
    >
      {children}
      {!strip && other}
    </div>
  );
  if (!strip) return scroller;

  return (
    <>
      <div className="strip-group" role={role} aria-label={ariaLabel}>
        <div className="strip-frame">
          {scroller}
          {!edges.atStart && (
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon strip-arrow strip-arrow-prev"
              aria-label="Previous options"
              onClick={() => nudge(-1)}
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" aria-hidden focusable="false">
                <path
                  d="M15 5L8 12L15 19"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </button>
          )}
          {!edges.atEnd && (
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-icon strip-arrow strip-arrow-next"
              aria-label="Next options"
              onClick={() => nudge(1)}
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" aria-hidden focusable="false">
                <path
                  d="M9 5L16 12L9 19"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </button>
          )}
        </div>
        {other}
      </div>
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
      </div>
    </>
  );
}
