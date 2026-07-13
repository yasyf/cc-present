import { useLayoutEffect, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { useExpandAll } from '../expand';

interface ClampedProps {
  html?: string;
  children?: ReactNode;
  lines: number;
  className?: string;
}

export function Clamped({ html, children, lines, className }: ClampedProps) {
  const { epoch, expanded: wantExpanded } = useExpandAll();
  const contentRef = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
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

  useLayoutEffect(() => {
    const el = contentRef.current;
    if (!el) return;
    const measure = () => {
      if (expandedRef.current) return;
      setOverflowing(el.scrollHeight > el.clientHeight);
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
  }, [html, children]);

  const collapsed = !expanded;
  const contentClass = `clamped-content${className ? ` ${className}` : ''}${
    collapsed ? ' is-clamped' : ''
  }${collapsed && overflowing ? ' is-faded' : ''}`;
  const style = collapsed ? ({ '--clamp-lines': lines } as CSSProperties) : undefined;

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
          onClick={(e) => {
            e.stopPropagation();
            setExpanded((v) => !v);
          }}
          onKeyDown={(e) => e.stopPropagation()}
        >
          {expanded ? 'Show less' : 'Show more'}
        </button>
      )}
    </div>
  );
}
