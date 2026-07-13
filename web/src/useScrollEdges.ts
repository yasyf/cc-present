import { useCallback, useEffect, useRef, useState } from 'react';

export interface ScrollEdges {
  atStart: boolean;
  atEnd: boolean;
}

/**
 * Tracks whether the ref'd scroller sits at the start/end of an axis, for
 * edge-fade overflow cues; updates on scroll and resize.
 */
export function useScrollEdges<T extends HTMLElement>(axis: 'x' | 'y' = 'x') {
  const ref = useRef<T | null>(null);
  const [edges, setEdges] = useState<ScrollEdges>({ atStart: true, atEnd: true });

  const measure = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    const pos = axis === 'x' ? el.scrollLeft : el.scrollTop;
    const max = axis === 'x' ? el.scrollWidth - el.clientWidth : el.scrollHeight - el.clientHeight;
    const next = { atStart: pos <= 1, atEnd: pos >= max - 1 };
    setEdges((prev) => (prev.atStart === next.atStart && prev.atEnd === next.atEnd ? prev : next));
  }, [axis]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    measure();
    el.addEventListener('scroll', measure, { passive: true });
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => {
      el.removeEventListener('scroll', measure);
      ro.disconnect();
    };
  }, [measure]);

  return { ref, edges };
}
