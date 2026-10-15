// The review layer's provider: a single cursor over the live round's decidable
// ring, a registry the interactive blocks join, and one document-level keydown
// listener that folds each key through interpretKey and dispatches. The cursor is
// derived at render — raw state clamped to the ring — so a block leaving the ring
// drops the cursor with no reset effect. The listener bubbles (never captures) and
// respects e.defaultPrevented, so Choice's per-option keys and Clamped's guarded
// buttons win before it ever sees the event.

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode, RefObject } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import { decidableIds, nextUndecided, step, submitItems } from './decide';
import { interpretKey } from './keymap';
import type { KeyDescriptor } from './keymap';
import { HelpOverlay } from './components/HelpOverlay';
import { useInteractivePackTypes } from './packs/registry';
import type { Block } from './schema';
import type { Interactions } from './events';

export type DecidableKind = 'approval' | 'choice' | 'input' | 'pack' | 'draft' | 'triage';

export interface DecidableSpec {
  kind: DecidableKind;
  disabled: boolean;
  engage?: () => void;
  verdict?: (target: 'approved' | 'rejected') => void;
  clear?: () => void;
  choose?: (option: number) => void;
}

interface DecidableHandle {
  id: string;
  kind: DecidableKind;
  elRef: RefObject<HTMLElement | null>;
  specRef: RefObject<DecidableSpec>;
}

// StepNav is the focus deck's navigation, registered while the deck is mounted so
// the keyboard's move / next / dot-jump route to steps instead of the ring.
export interface StepNav {
  move: (delta: 1 | -1) => void;
  next: () => void;
  jump: (id: string) => void;
  end: () => void;
}

export interface KeyboardApi {
  register: (handle: DecidableHandle) => void;
  unregister: (handle: DecidableHandle) => void;
  registerSubmit: (fn: (() => void) | null) => void;
  registerNote: (fn: (() => void) | null) => void;
  registerEscape: (fn: (() => boolean) | null) => void;
  registerStepNav: (nav: StepNav | null) => void;
  setCursor: (id: string | null) => void;
  reveal: (id: string) => void;
  jumpTo: (id: string) => void;
  jumpNextUndecided: () => void;
  announce: (msg: string) => void;
}

const ApiContext = createContext<KeyboardApi | null>(null);
const CursorContext = createContext<string | null>(null);
const ClosedContext = createContext(false);

export function useKeyboardApi(): KeyboardApi {
  const api = useContext(ApiContext);
  if (!api) throw new Error('useKeyboardApi must be used within a KeyboardProvider');
  return api;
}

// useCursor reports the id the ring cursor currently rests on (null off-ring), so
// the margin rail can pin the active block's thread as the cursor moves.
export function useCursor(): string | null {
  return useContext(CursorContext);
}

// useDecidable joins the ring for `id` while it is live or the board is closed,
// and reports whether the cursor rests on it. A frozen history round re-renders the
// same block ids read-only, so it never registers and never draws the ring.
export function useDecidable(id: string, spec: DecidableSpec): {
  ref: (el: HTMLElement | null) => void;
  cursor: boolean;
} {
  const api = useKeyboardApi();
  const cursorId = useContext(CursorContext);
  const closed = useContext(ClosedContext);
  const readOnly = useGroupReadOnly();
  const elRef = useRef<HTMLElement | null>(null);
  const specRef = useRef(spec);
  specRef.current = spec;
  const navigable = !readOnly && (closed || !spec.disabled);

  useEffect(() => {
    if (!navigable) return;
    const handle: DecidableHandle = { id, kind: spec.kind, elRef, specRef };
    api.register(handle);
    return () => api.unregister(handle);
  }, [api, id, spec.kind, navigable]);

  const ref = useCallback((el: HTMLElement | null) => {
    elRef.current = el;
  }, []);

  return { ref, cursor: navigable && cursorId === id };
}

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
}

function descriptorOf(e: KeyboardEvent): KeyDescriptor {
  return {
    key: e.key,
    meta: e.metaKey,
    ctrl: e.ctrlKey,
    alt: e.altKey,
    shift: e.shiftKey,
    repeat: e.repeat,
  };
}

function scrollBehavior(): ScrollBehavior {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';
}

function viewportDistance(top: number, bottom: number, viewportHeight: number): number {
  return bottom < 0 ? -bottom : top > viewportHeight ? top - viewportHeight : 0;
}

function offscreen(el: HTMLElement | null): boolean {
  if (!el) return false;
  const { top, bottom } = el.getBoundingClientRect();
  return viewportDistance(top, bottom, window.innerHeight) > 0;
}

// nearestInViewport picks the id whose rect is closest to the viewport band
// [0, viewportHeight] (an intersecting one scores 0), ties resolving to the
// topmost. It resolves where a null cursor enters the ring, so j/k lands on the
// visible block, never a ring end. Rects in, id out — no layout needed.
export function nearestInViewport(
  candidates: { id: string; top: number; bottom: number }[],
  viewportHeight: number,
): string | null {
  let best: string | null = null;
  let bestDist = Infinity;
  for (const c of candidates) {
    const dist = viewportDistance(c.top, c.bottom, viewportHeight);
    if (dist < bestDist) {
      bestDist = dist;
      best = c.id;
    }
  }
  return best;
}

export interface KeyboardProviderProps {
  blocks: Block[];
  interactions: Interactions;
  closed: boolean;
  round: number;
  onViewToggle?: () => void;
  onExpandAll?: () => boolean;
  children: ReactNode;
}

export function KeyboardProvider({ blocks, interactions, closed, round, onViewToggle, onExpandAll, children }: KeyboardProviderProps) {
  const [cursorId, setCursorId] = useState<string | null>(null);
  const [helpOpen, setHelpOpen] = useState(false);
  const [liveMsg, setLiveMsg] = useState('');
  const registry = useRef(new Map<string, DecidableHandle>()).current;
  const submitFnRef = useRef<(() => void) | null>(null);
  const noteFnRef = useRef<(() => void) | null>(null);
  const escapeFnRef = useRef<(() => boolean) | null>(null);
  const stepNavRef = useRef<StepNav | null>(null);
  const viewToggleRef = useRef(onViewToggle);
  viewToggleRef.current = onViewToggle;
  const expandAllRef = useRef(onExpandAll);
  expandAllRef.current = onExpandAll;

  const packInteractive = useInteractivePackTypes();
  const ring = useMemo(() => decidableIds(blocks, packInteractive), [blocks, packInteractive]);
  const undecided = useMemo(() => {
    const set = new Set<string>();
    for (const item of submitItems(blocks, interactions, packInteractive)) if (!item.decided) set.add(item.id);
    return set;
  }, [blocks, interactions, packInteractive]);
  const effectiveCursor = cursorId !== null && ring.includes(cursorId) ? cursorId : null;

  const ringRef = useRef(ring);
  const undecidedRef = useRef(undecided);
  const cursorRef = useRef(effectiveCursor);
  const closedRef = useRef(closed);
  ringRef.current = ring;
  undecidedRef.current = undecided;
  cursorRef.current = effectiveCursor;
  closedRef.current = closed;

  const register = useCallback(
    (handle: DecidableHandle) => {
      registry.set(handle.id, handle);
    },
    [registry],
  );
  const unregister = useCallback(
    (handle: DecidableHandle) => {
      if (registry.get(handle.id) === handle) registry.delete(handle.id);
    },
    [registry],
  );
  const registerSubmit = useCallback((fn: (() => void) | null) => {
    submitFnRef.current = fn;
  }, []);
  const registerNote = useCallback((fn: (() => void) | null) => {
    noteFnRef.current = fn;
  }, []);
  const registerEscape = useCallback((fn: (() => boolean) | null) => {
    escapeFnRef.current = fn;
  }, []);
  const registerStepNav = useCallback((nav: StepNav | null) => {
    stepNavRef.current = nav;
  }, []);
  const setCursor = useCallback((id: string | null) => setCursorId(id), []);

  const announce = useCallback((msg: string) => setLiveMsg(msg), []);
  const scrollToId = useCallback(
    (id: string, block: ScrollLogicalPosition) => {
      registry.get(id)?.elRef.current?.scrollIntoView({ block, behavior: scrollBehavior() });
    },
    [registry],
  );
  const reveal = useCallback((id: string) => scrollToId(id, 'nearest'), [scrollToId]);
  // Focus mode registers a StepNav: a jump then targets the step owning the id
  // (its anchor or any nested decidable) via setStep, never a scroll or a registry
  // lookup against the unmounted off-step blocks.
  const jumpTo = useCallback(
    (id: string) => {
      if (stepNavRef.current) {
        stepNavRef.current.jump(id);
        return;
      }
      if (!ringRef.current.includes(id)) {
        [...document.querySelectorAll<HTMLElement>('[data-block-id]')]
          .filter((row) => row.dataset.blockId === id)
          .at(-1)
          ?.scrollIntoView({ block: 'center', behavior: scrollBehavior() });
        return;
      }
      setCursorId(id);
      scrollToId(id, 'center');
      announce(`Item ${ringRef.current.indexOf(id) + 1} of ${ringRef.current.length}`);
    },
    [announce, scrollToId],
  );
  const jumpNextUndecided = useCallback(() => {
    if (stepNavRef.current) {
      stepNavRef.current.next();
      return;
    }
    const target = nextUndecided(ringRef.current, undecidedRef.current, cursorRef.current);
    if (!target) {
      announce('All answered');
      return;
    }
    jumpTo(target);
  }, [announce, jumpTo]);
  const moveCursor = useCallback(
    (delta: 1 | -1) => {
      const ring = ringRef.current;
      const from = cursorRef.current;
      // No in-ring cursor: enter on the member nearest the viewport rather than the
      // ring end step() would pick, so j/k never teleports past the page.
      const next =
        from !== null
          ? step(ring, from, delta)
          : nearestInViewport(
              ring.flatMap((id) => {
                const el = registry.get(id)?.elRef.current;
                if (!el) return [];
                const { top, bottom } = el.getBoundingClientRect();
                return [{ id, top, bottom }];
              }),
              window.innerHeight,
            );
      if (next === null) return;
      setCursorId(next);
      scrollToId(next, 'nearest');
    },
    [registry, scrollToId],
  );

  const prevRound = useRef(round);
  useEffect(() => {
    if (prevRound.current !== round) {
      prevRound.current = round;
      announce(`Round ${round}`);
    }
  }, [round, announce]);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.defaultPrevented) return;
      const typing = isTypingTarget(e.target);
      const action = interpretKey(descriptorOf(e), typing, closedRef.current, stepNavRef.current !== null);
      if (!action) return;
      // An open modal dialog (help, an image lightbox) owns the page: shortcuts stay
      // suppressed behind it, Esc rides its native cancel, and `?` closes help.
      const dialog = document.querySelector('dialog[open]');
      const closingHelp = action.kind === 'help-toggle' && dialog?.classList.contains('help-dialog');
      if (dialog && action.kind !== 'escape' && !closingHelp) return;

      const handle = cursorRef.current ? registry.get(cursorRef.current) : undefined;
      const revealIfOffscreen = (h: DecidableHandle): boolean => {
        if (!offscreen(h.elRef.current)) return false;
        scrollToId(h.id, 'center');
        return true;
      };
      switch (action.kind) {
        case 'move':
          e.preventDefault();
          if (stepNavRef.current) stepNavRef.current.move(action.delta);
          else moveCursor(action.delta);
          break;
        case 'next-undecided':
          e.preventDefault();
          jumpNextUndecided();
          break;
        case 'verdict':
          if (handle?.kind === 'approval' || handle?.kind === 'triage') {
            e.preventDefault();
            if (!revealIfOffscreen(handle)) handle.specRef.current.verdict?.(action.verdict);
          }
          break;
        case 'clear':
          if (handle?.kind === 'approval' || handle?.kind === 'triage') {
            e.preventDefault();
            if (!revealIfOffscreen(handle)) handle.specRef.current.clear?.();
          }
          break;
        case 'choose':
          if (handle?.kind === 'choice' || handle?.kind === 'triage') {
            e.preventDefault();
            if (!revealIfOffscreen(handle)) handle.specRef.current.choose?.(action.option);
          }
          break;
        case 'engage': {
          const engage = handle?.specRef.current.engage ?? noteFnRef.current;
          if (engage) {
            e.preventDefault();
            engage();
          }
          break;
        }
        case 'submit': {
          const run = submitFnRef.current ?? stepNavRef.current?.end;
          if (run) {
            e.preventDefault();
            // A focused input commits only on blur; blur it first so its draft
            // posts before submit, mirroring the mouse path when focus leaves.
            const active = document.activeElement;
            if (active instanceof HTMLElement && isTypingTarget(active)) active.blur();
            run();
          }
          break;
        }
        case 'view-toggle':
          e.preventDefault();
          viewToggleRef.current?.();
          break;
        case 'expand-all': {
          const toggle = expandAllRef.current;
          if (toggle) {
            e.preventDefault();
            announce(toggle() ? 'Expanded all' : 'Collapsed all');
          }
          break;
        }
        case 'help-toggle':
          e.preventDefault();
          setHelpOpen((v) => !v);
          break;
        case 'escape':
          // With an input to blur, Esc leaves the field; otherwise — and only when
          // no dialog owns Esc — it cancels an armed submit confirm, which the bar
          // itself catches only while its own div holds focus.
          if (typing && e.target instanceof HTMLElement) {
            e.preventDefault();
            e.target.blur();
          } else if (!dialog && escapeFnRef.current?.()) {
            e.preventDefault();
          }
          break;
      }
    }
    function onFocusIn(e: FocusEvent) {
      const node = e.target;
      if (!(node instanceof Node)) return;
      for (const handle of registry.values()) {
        if (handle.elRef.current?.contains(node)) {
          setCursorId(handle.id);
          return;
        }
      }
    }
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('focusin', onFocusIn);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('focusin', onFocusIn);
    };
  }, [registry, moveCursor, jumpNextUndecided, scrollToId]);

  const api = useMemo<KeyboardApi>(
    () => ({
      register,
      unregister,
      registerSubmit,
      registerNote,
      registerEscape,
      registerStepNav,
      setCursor,
      reveal,
      jumpTo,
      jumpNextUndecided,
      announce,
    }),
    [
      register,
      unregister,
      registerSubmit,
      registerNote,
      registerEscape,
      registerStepNav,
      setCursor,
      reveal,
      jumpTo,
      jumpNextUndecided,
      announce,
    ],
  );

  return (
    <ApiContext.Provider value={api}>
      <CursorContext.Provider value={effectiveCursor}>
        <ClosedContext.Provider value={closed}>{children}</ClosedContext.Provider>
        <div className="sr-only" role="status" aria-live="polite" aria-atomic="true">
          {liveMsg}
        </div>
        <HelpOverlay open={helpOpen} onClose={() => setHelpOpen(false)} />
      </CursorContext.Provider>
    </ApiContext.Provider>
  );
}
