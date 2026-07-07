import { useLayoutEffect, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';

interface ClampedProps {
  html?: string;
  children?: ReactNode;
  lines: number;
  className?: string;
}

export function Clamped({ html, children, lines, className }: ClampedProps) {
  const contentRef = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);
  const expandedRef = useRef(expanded);
  expandedRef.current = expanded;
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

  useLayoutEffect(() => {
    if (!mounted.current) {
      mounted.current = true;
      return;
    }
    setExpanded(false);
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
