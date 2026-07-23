// The desktop margin rail's open-state machine. `open` is the union of four
// intents — a pinned latch, hover intent, focus within, and an active composer:
//
//   open = pinnedOpen || hoverIntent || focusWithin || composing
//
// hoverIntent and focusWithin are derived here from pointer/focus events on the
// rail container; pinnedOpen and composing are supplied by the caller (the chip and
// the `f` compose latch). Esc while focus is inside the rail (and not composing) and
// a pointerdown outside it both ask the caller to drop the pin through onDismiss.

import { useCallback, useEffect, useRef, useState } from 'react';

// A pointerleave grace so a diagonal slip between the strip and the panel, or a
// small overshoot, does not snap the rail shut mid-reach. Cleared on re-enter.
const HOVER_GRACE_MS = 250;

export interface RailOpen {
  // True while the rail should show its expanded panel.
  open: boolean;
  // Attach to the rail container (strip + panel); the machine wires itself off it.
  ref: (el: HTMLElement | null) => void;
}

// useRailOpen wires the rail container's hover/focus events into the open union and
// dispatches the two dismiss gestures (Esc inside, pointerdown outside) to onDismiss.
export function useRailOpen({
  pinnedOpen,
  composing,
  onDismiss,
}: {
  pinnedOpen: boolean;
  composing: boolean;
  onDismiss: () => void;
}): RailOpen {
  const [container, setContainer] = useState<HTMLElement | null>(null);
  const [hoverIntent, setHoverIntent] = useState(false);
  const [focusWithin, setFocusWithin] = useState(false);
  const graceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const containerRef = useRef<HTMLElement | null>(null);
  const focusWithinRef = useRef(false);
  const returnFocusRef = useRef<HTMLElement | null>(null);
  const composingRef = useRef(composing);
  composingRef.current = composing;
  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;
  const open = pinnedOpen || hoverIntent || focusWithin || composing;
  const clearGrace = useCallback(() => {
    if (graceRef.current !== null) {
      clearTimeout(graceRef.current);
      graceRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!container) return;
    const onEnter = () => {
      clearGrace();
      setHoverIntent(true);
    };
    const onLeave = () => {
      clearGrace();
      graceRef.current = setTimeout(() => {
        graceRef.current = null;
        setHoverIntent(false);
      }, HOVER_GRACE_MS);
    };
    // focusin/focusout bubble, so one pair on the container covers every descendant.
    // The first entry records where focus came from, so Esc can hand it back.
    const onFocusIn = (e: FocusEvent) => {
      if (!focusWithinRef.current && e.relatedTarget instanceof HTMLElement) returnFocusRef.current = e.relatedTarget;
      focusWithinRef.current = true;
      setFocusWithin(true);
    };
    const onFocusOut = (e: FocusEvent) => {
      if (e.relatedTarget instanceof Node && container.contains(e.relatedTarget)) return;
      focusWithinRef.current = false;
      setFocusWithin(false);
    };
    container.addEventListener('pointerenter', onEnter);
    container.addEventListener('pointerleave', onLeave);
    container.addEventListener('focusin', onFocusIn);
    container.addEventListener('focusout', onFocusOut);
    return () => {
      clearGrace();
      container.removeEventListener('pointerenter', onEnter);
      container.removeEventListener('pointerleave', onLeave);
      container.removeEventListener('focusin', onFocusIn);
      container.removeEventListener('focusout', onFocusOut);
    };
  }, [clearGrace, container]);

  // Esc inside any open rail (unless composing) closes and returns focus.
  useEffect(() => {
    if (!container || !open) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || composingRef.current || e.defaultPrevented) return;
      if (!container.contains(document.activeElement)) return;
      e.preventDefault();
      onDismissRef.current();
      clearGrace();
      setHoverIntent(false);
      focusWithinRef.current = false;
      setFocusWithin(false);
      const ret = returnFocusRef.current;
      returnFocusRef.current = null;
      if (ret?.isConnected) ret.focus();
      else (document.activeElement as HTMLElement | null)?.blur();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [clearGrace, container, open]);

  // A pointerdown anywhere outside a pinned rail closes it, but a click on a chip
  // (the rail's own re-anchor control) is not an outside dismiss.
  useEffect(() => {
    if (!container || !pinnedOpen) return;
    const onPointerDown = (e: PointerEvent) => {
      if (e.target instanceof Node && container.contains(e.target)) return;
      if (e.target instanceof Element && e.target.closest('[data-rail-anchor]')) return;
      onDismissRef.current();
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => document.removeEventListener('pointerdown', onPointerDown);
  }, [container, pinnedOpen]);

  const ref = useCallback(
    (el: HTMLElement | null) => {
      if (containerRef.current === el) return;
      containerRef.current = el;
      clearGrace();
      setHoverIntent(false);
      focusWithinRef.current = false;
      setFocusWithin(false);
      returnFocusRef.current = null;
      setContainer(el);
    },
    [clearGrace],
  );
  return { open, ref };
}
