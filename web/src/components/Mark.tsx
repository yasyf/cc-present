import type { ReactNode } from 'react';

export type MarkKind = 'check' | 'cross' | 'ring' | 'strike' | 'rule';

// Each path is pathLength-1 normalized so the stroke draw is length-independent.
const PATHS: Record<MarkKind, ReactNode> = {
  check: <path className="mark-stroke" pathLength={1} d="M5 12.5L10 17.5L19.5 6.5" />,
  cross: (
    <>
      <path className="mark-stroke" pathLength={1} d="M6.5 6.5L17.5 17.5" />
      <path className="mark-stroke" pathLength={1} d="M17.5 6.5L6.5 17.5" />
    </>
  ),
  ring: (
    <path
      className="mark-stroke"
      pathLength={1}
      d="M12 3.4C16.7 3.4 20.6 7.2 20.6 12C20.6 16.8 16.7 20.6 12 20.6C7.3 20.6 3.4 16.8 3.4 12C3.4 7.2 7.3 3.4 12 3.4Z"
    />
  ),
  strike: <path className="mark-stroke" pathLength={1} d="M4 12H20" />,
  rule: <path className="mark-stroke" pathLength={1} d="M2.5 12H21.5" />,
};

// Mark is the Blue Pencil signature: one drawn SVG family stamped at every human
// commitment. Decorative (aria-hidden) — the surrounding control owns semantics.
export function Mark({ kind, className }: { kind: MarkKind; className?: string }) {
  return (
    <span className={`mark mark-${kind}${className ? ` ${className}` : ''}`} aria-hidden>
      <svg viewBox="0 0 24 24" fill="none" focusable="false">
        {PATHS[kind]}
      </svg>
    </span>
  );
}
