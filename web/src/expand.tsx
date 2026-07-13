// The expand-all coordinator: a shared epoch and expanded target. Toggling bumps
// the epoch; each Clamped re-syncs, a per-block toggle wins until the next epoch.

import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';

export interface ExpandAllApi {
  epoch: number;
  expanded: boolean;
  toggle: () => boolean;
}

const ExpandAllContext = createContext<ExpandAllApi>({ epoch: 0, expanded: false, toggle: () => false });

// useExpandAll reads the shared state; the default stands in when no provider wraps the tree.
export function useExpandAll(): ExpandAllApi {
  return useContext(ExpandAllContext);
}

// ExpandAllProvider owns the epoch/expanded pair; toggle returns the new state to announce.
export function ExpandAllProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState({ epoch: 0, expanded: false });
  const expandedRef = useRef(state.expanded);
  expandedRef.current = state.expanded;
  const toggle = useCallback(() => {
    const next = !expandedRef.current;
    setState((s) => ({ epoch: s.epoch + 1, expanded: next }));
    return next;
  }, []);
  const value = useMemo<ExpandAllApi>(
    () => ({ epoch: state.epoch, expanded: state.expanded, toggle }),
    [state.epoch, state.expanded, toggle],
  );
  return <ExpandAllContext.Provider value={value}>{children}</ExpandAllContext.Provider>;
}
