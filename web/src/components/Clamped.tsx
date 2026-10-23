import { useLayoutEffect, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { useExpandAll } from '../expand';

interface LineBox {
  top: number;
  bottom: number;
}

// snapHeight is the height from originTop to the bottom of the lines-th line
// box, grouping fragments that share a line; null when the text has no more
// than lines lines.
export function snapHeight(boxes: LineBox[], originTop: number, lines: number): number | null {
  const bottoms: number[] = [];
  for (const box of boxes.filter((b) => b.bottom > b.top).sort((a, b) => a.top - b.top)) {
    const last = bottoms.length - 1;
    if (last >= 0 && box.top < bottoms[last]! - 1) bottoms[last] = Math.max(bottoms[last]!, box.bottom);
    else bottoms.push(box.bottom);
  }
  return bottoms.length > lines ? Math.ceil(bottoms[lines - 1]! - originTop) : null;
}

function textLineBoxes(el: HTMLElement): LineBox[] {
  const boxes: LineBox[] = [];
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  const range = document.createRange();
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    range.selectNodeContents(node);
    for (const r of range.getClientRects()) boxes.push({ top: r.top, bottom: r.bottom });
  }
  return boxes;
}

interface ClampedProps {
  html?: string;
  children?: ReactNode;
  contentKey?: string;
  lines: number;
  className?: string;
}

export function Clamped({ html, children, contentKey, lines, className }: ClampedProps) {
  const { epoch, expanded: wantExpanded } = useExpandAll();
  const contentRef = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const [clampHeight, setClampHeight] = useState<number | null>(null);
  // A global expand-all bumps the epoch; re-sync this block to the shared target,
  // which a later Show more/less press overrides until the next epoch.
  const [syncedEpoch, setSyncedEpoch] = useState(epoch);
  if (syncedEpoch !== epoch) {
    setSyncedEpoch(epoch);
    setExpanded(wantExpanded);
  }
  const expandedRef = useRef(expanded);
  expandedRef.current = expanded;
  const wantExpandedRef = useRef(wantExpanded);
  wantExpandedRef.current = wantExpanded;
  const mounted = useRef(false);
  const linesRef = useRef(lines);
  linesRef.current = lines;

  useLayoutEffect(() => {
    const el = contentRef.current;
    if (!el) return;
    const measure = () => {
      if (expandedRef.current) return;
      setOverflowing(el.scrollHeight > el.clientHeight);
      setClampHeight(snapHeight(textLineBoxes(el), el.getBoundingClientRect().top - el.scrollTop, linesRef.current));
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  // A content swap re-syncs to the shared expand-all target (false without a provider)
  // rather than force-collapsing, so a global expand survives the swap.
  useLayoutEffect(() => {
    if (!mounted.current) {
      mounted.current = true;
      return;
    }
    setExpanded(wantExpandedRef.current);
  }, [html, contentKey]);

  const collapsed = !expanded;
  const contentClass = `clamped-content${className ? ` ${className}` : ''}${collapsed ? ' is-clamped' : ''}`;
  const style = collapsed
    ? ({ '--clamp-lines': lines, ...(clampHeight !== null && { '--clamp-h': `${clampHeight}px` }) } as CSSProperties)
    : undefined;

  return (
    <div className="clamped">
      {html !== undefined ? (
        <div
          ref={contentRef}
          className={contentClass}
          style={style}
          dangerouslySetInnerHTML={{ __html: html }}
        />
      ) : (
        <div ref={contentRef} className={contentClass} style={style}>
          {children}
        </div>
      )}
      {overflowing && (
        <button
          type="button"
          className="clamp-toggle link-btn"
          aria-expanded={expanded}
          title={expanded ? 'Show less (e collapses all)' : 'Show more (e expands all)'}
          onClick={(e) => {
            e.stopPropagation();
            setExpanded((v) => !v);
          }}
          onKeyDown={(e) => {
            if ((e.key === 'Enter' || e.key === ' ') && !e.metaKey && !e.ctrlKey) e.stopPropagation();
          }}
        >
          {expanded ? 'Show less' : 'Show more'}
        </button>
      )}
    </div>
  );
}
