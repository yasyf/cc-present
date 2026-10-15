import { useEffect, useState } from 'react';

export interface ScrollEdges {
  atStart: boolean;
  atEnd: boolean;
}

/**
 * Tracks whether the ref'd scroller sits at the start/end of an axis, for
 * edge-fade overflow cues; updates on scroll and resize.
 */
export function useScrollEdges<T extends HTMLElement>(axis: 'x' | 'y' = 'x') {
  const [node, setNode] = useState<T | null>(null);
  const [edges, setEdges] = useState<ScrollEdges>({ atStart: true, atEnd: true });

  useEffect(() => {
    if (!node) return;
    const measure = () => {
      const pos = axis === 'x' ? node.scrollLeft : node.scrollTop;
      const max = axis === 'x' ? node.scrollWidth - node.clientWidth : node.scrollHeight - node.clientHeight;
      const next = { atStart: pos <= 1, atEnd: pos >= max - 1 };
      setEdges((prev) => (prev.atStart === next.atStart && prev.atEnd === next.atEnd ? prev : next));
    };
    measure();
    node.addEventListener('scroll', measure, { passive: true });
    const ro = new ResizeObserver(measure);
    ro.observe(node);
    // A same-id content swap can change scrollWidth without resizing the box.
    if (node.firstElementChild) ro.observe(node.firstElementChild);
    return () => {
      node.removeEventListener('scroll', measure);
      ro.disconnect();
    };
  }, [node, axis]);

  return { ref: setNode, edges };
}
