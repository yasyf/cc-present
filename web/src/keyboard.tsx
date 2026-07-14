// The review layer's provider: a single cursor over the live round's decidable
// ring, a registry the interactive blocks join, and one document-level keydown
// listener that folds each key through interpretKey and dispatches. The cursor is
// derived at render — raw state clamped to the ring — so a block leaving the ring
// drops the cursor with no reset effect. The listener bubbles (never captures) and
// respects e.defaultPrevented, so Choice's per-option keys and Clamped's guarded
// buttons win before it ever sees the event.

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode, RefObject } from 'react';
import { decidableIds, nextUndecided, step, submitItems } from './decide';
import { interpretKey } from './keymap';
import type { KeyDescriptor } from './keymap';
import { HelpOverlay } from './components/HelpOverlay';
import { useInteractivePackTypes } from './packs/registry';
import type { Block } from './schema';
import type { Interactions } from './events';

export type DecidableKind = 'approval' | 'choice' | 'input' | 'pack';

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
}

export interface KeyboardApi {
  register: (handle: DecidableHandle) => void;
  unregister: (handle: DecidableHandle) => void;
  registerSubmit: (fn: (() => void) | null) => void;
  registerEscape: (fn: (() => boolean) | null) => void;
  registerStepNav: (nav: StepNav | null) => void;
  setCursor: (id: string | null) => void;
  jumpTo: (id: string) => void;
  jumpNextUndecided: () => void;
  announce: (msg: string) => void;
}

const ApiContext = createContext<KeyboardApi | null>(null);
const CursorContext = createContext<string | null>(null);

export function useKeyboardApi(): KeyboardApi {
  const api = useContext(ApiContext);
  if (!api) throw new Error('useKeyboardApi must be used within a KeyboardProvider');
  return api;
}

// useDecidable joins the ring for `id` while enabled and reports whether the
// cursor rests on it. Registration is gated on !disabled so a frozen history
// round — which re-renders the same block ids read-only — never registers, and
// the cursor flag folds in !disabled so only the live instance draws the ring.
export function useDecidable(id: string, spec: DecidableSpec): {
  ref: (el: HTMLElement | null) => void;
  cursor: boolean;
} {
  const api = useKeyboardApi();
  const cursorId = useContext(CursorContext);
  const elRef = useRef<HTMLElement | null>(null);
  const specRef = useRef(spec);
  specRef.current = spec;

  useEffect(() => {
    if (spec.disabled) return;
    const handle: DecidableHandle = { id, kind: spec.kind, elRef, specRef };
    api.register(handle);
    return () => api.unregister(handle);
  }, [api, id, spec.kind, spec.disabled]);

  const ref = useCallback((el: HTMLElement | null) => {
    elRef.current = el;
  }, []);

  return { ref, cursor: !spec.disabled && cursorId === id };
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
  // Focus mode registers a StepNav: a jump then targets the step owning the id
  // (its anchor or any nested decidable) via setStep, never a scroll or a registry
  // lookup against the unmounted off-step blocks.
  const jumpTo = useCallback(
    (id: string) => {
      if (stepNavRef.current) {
        stepNavRef.current.jump(id);
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
      announce('All items decided');
      return;
    }
    jumpTo(target);
  }, [announce, jumpTo]);
  const moveCursor = useCallback(
    (delta: 1 | -1) => {
      const next = step(ringRef.current, cursorRef.current, delta);
      if (next === null) return;
      setCursorId(next);
      scrollToId(next, 'nearest');
    },
    [scrollToId],
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
      const action = interpretKey(descriptorOf(e), typing, closedRef.current);
      if (!action) return;
      // Any open modal dialog (help, an image lightbox) owns the page: global
      // shortcuts stay suppressed behind it, and Esc rides the dialog's native
      // cancel back to state.
      const dialogOpen = document.querySelector('dialog[open]') !== null;
      if (dialogOpen && action.kind !== 'escape') return;

      const handle = cursorRef.current ? registry.get(cursorRef.current) : undefined;
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
          if (handle?.kind === 'approval') {
            e.preventDefault();
            handle.specRef.current.verdict?.(action.verdict);
          }
          break;
        case 'clear':
          if (handle?.kind === 'approval') {
            e.preventDefault();
            handle.specRef.current.clear?.();
          }
          break;
        case 'choose':
          if (handle?.kind === 'choice') {
            e.preventDefault();
            handle.specRef.current.choose?.(action.option);
          }
          break;
        case 'engage':
          if (handle && (handle.kind === 'approval' || handle.kind === 'input' || handle.kind === 'pack')) {
            e.preventDefault();
            handle.specRef.current.engage?.();
          }
          break;
        case 'submit':
          if (submitFnRef.current) {
            e.preventDefault();
            // A focused input commits only on blur; blur it first so its draft
            // posts before submit, mirroring the mouse path when focus leaves.
            const active = document.activeElement;
            if (active instanceof HTMLElement && isTypingTarget(active)) active.blur();
            submitFnRef.current();
          }
          break;
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
          } else if (!dialogOpen && escapeFnRef.current?.()) {
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
  }, [registry, moveCursor, jumpNextUndecided]);

  const api = useMemo<KeyboardApi>(
    () => ({ register, unregister, registerSubmit, registerEscape, registerStepNav, setCursor, jumpTo, jumpNextUndecided, announce }),
    [register, unregister, registerSubmit, registerEscape, registerStepNav, setCursor, jumpTo, jumpNextUndecided, announce],
  );

  return (
    <ApiContext.Provider value={api}>
      <CursorContext.Provider value={effectiveCursor}>
        {children}
        <div className="sr-only" role="status" aria-live="polite" aria-atomic="true">
          {liveMsg}
        </div>
        <HelpOverlay open={helpOpen} onClose={() => setHelpOpen(false)} />
      </CursorContext.Provider>
    </ApiContext.Provider>
  );
}
