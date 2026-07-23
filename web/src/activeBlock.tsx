// The rail's control layer: records the last-touched block, resolves the
// addressed block (pin > cursor > last touch), and freezes it while composing.

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { PresentContext, usePresent } from './present';
import type { PresentApi } from './present';
import { useCursor } from './keyboard';
import type { Interaction } from './events';

export interface ActiveBlockApi {
  // The block the rail pins its thread to; null when nothing is active yet.
  activeId: string | null;
  panelOpen: boolean;
  // The panel was opened to compose, so the sheet raises its composer on mount.
  panelCompose: boolean;
  // Bumped when the compose affordance fires, so the always-mounted desktop panel
  // opens its composer on the change.
  composeEpoch: number;
  // The desktop rail's pin latch: held open by a chip click or a compose request,
  // released by Esc / a pointerdown outside the rail (see useRailOpen).
  pinnedOpen: boolean;
  // True while a rail composer holds a draft — the rail stays open regardless of
  // hover or focus while this is set.
  composing: boolean;
  pin: (id: string) => void;
  openPanel: () => void;
  closePanel: () => void;
  requestCompose: () => void;
  setPinnedOpen: (open: boolean) => void;
  setComposing: (composing: boolean) => void;
}

const NOOP: ActiveBlockApi = {
  activeId: null,
  panelOpen: false,
  panelCompose: false,
  composeEpoch: 0,
  pinnedOpen: false,
  composing: false,
  pin: () => {},
  openPanel: () => {},
  closePanel: () => {},
  requestCompose: () => {},
  setPinnedOpen: () => {},
  setComposing: () => {},
};

// A default no-op value lets an inline block (SingleBlockView's embed) call the
// hook without a provider — it simply never drives a rail.
const ActiveBlockContext = createContext<ActiveBlockApi>(NOOP);

export function useActiveBlock(): ActiveBlockApi {
  return useContext(ActiveBlockContext);
}

export function ActiveBlockProvider({ children }: { children: ReactNode }) {
  const parent = usePresent();
  const cursor = useCursor();

  const [lastInteracted, setLastInteracted] = useState<string | null>(null);
  const [pinnedId, setPinnedId] = useState<string | null>(null);
  const [panel, setPanel] = useState<{ open: boolean; compose: boolean }>({ open: false, compose: false });
  const [pinnedOpen, setPinnedOpen] = useState(false);
  const [composeEpoch, setComposeEpoch] = useState(0);
  const [composing, setComposing] = useState(false);

  // A real cursor move supersedes an explicit pin, so the rail follows j/k
  // navigation once the human leaves the block they clicked a chip on.
  const prevCursor = useRef(cursor);
  useEffect(() => {
    if (cursor !== null && cursor !== prevCursor.current) setPinnedId(null);
    prevCursor.current = cursor;
  }, [cursor]);

  const raw = pinnedId ?? cursor ?? lastInteracted;
  const frozenRef = useRef(raw);
  useEffect(() => {
    if (!composing) frozenRef.current = raw;
  }, [composing, raw]);
  const activeId = composing ? frozenRef.current : raw;

  const post = useCallback(
    (interaction: Interaction) => {
      if ('blockId' in interaction) setLastInteracted(interaction.blockId);
      return parent.post(interaction);
    },
    [parent],
  );
  const api = useMemo<PresentApi>(() => ({ ...parent, post }), [parent, post]);

  const pin = useCallback((id: string) => {
    setPinnedId(id);
    setPanel({ open: true, compose: false });
  }, []);
  const openPanel = useCallback(() => setPanel({ open: true, compose: false }), []);
  const closePanel = useCallback(() => {
    setPanel({ open: false, compose: false });
    setPinnedOpen(false);
  }, []);
  // Compose is an explicit open: it raises the composer (below the rail, on the sheet
  // remount; at the rail, off composeEpoch) and pins the desktop rail open so the
  // `f` key and the chip both deliver a focused composer, not a silent no-op.
  const requestCompose = useCallback(() => {
    setComposeEpoch((e) => e + 1);
    setPanel({ open: true, compose: true });
    setPinnedOpen(true);
  }, []);

  const value = useMemo<ActiveBlockApi>(
    () => ({
      activeId,
      panelOpen: panel.open,
      panelCompose: panel.compose,
      composeEpoch,
      pinnedOpen,
      composing,
      pin,
      openPanel,
      closePanel,
      requestCompose,
      setPinnedOpen,
      setComposing,
    }),
    [activeId, panel.open, panel.compose, composeEpoch, pinnedOpen, composing, pin, openPanel, closePanel, requestCompose],
  );

  return (
    <PresentContext.Provider value={api}>
      <ActiveBlockContext.Provider value={value}>{children}</ActiveBlockContext.Provider>
    </PresentContext.Provider>
  );
}
