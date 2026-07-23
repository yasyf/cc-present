// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { useRailOpen } from './railOpen';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;
let dismissCount: number;

function Harness({
  pinnedOpen,
  composing,
  mounted = true,
}: {
  pinnedOpen: boolean;
  composing: boolean;
  mounted?: boolean;
}) {
  const rail = useRailOpen({ pinnedOpen, composing, onDismiss: () => (dismissCount += 1) });
  if (!mounted) return null;
  return (
    <div ref={rail.ref} data-testid="rail" data-open={String(rail.open)}>
      <button data-testid="rail-focus" type="button" />
    </div>
  );
}

function renderHarness(props: { pinnedOpen: boolean; composing: boolean; mounted?: boolean }): void {
  act(() => root.render(<Harness {...props} />));
}

const railEl = (): HTMLElement => container.querySelector('[data-testid="rail"]')!;
const railFocusEl = (): HTMLButtonElement => container.querySelector('[data-testid="rail-focus"]')!;
const open = (): string | null => railEl().getAttribute('data-open');
const fire = (target: EventTarget, type: string): void =>
  act(() => {
    target.dispatchEvent(new Event(type, { bubbles: true }));
  });
const pressEscape = (defaultPrevented = false): void =>
  act(() => {
    const event = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'Escape' });
    if (defaultPrevented) event.preventDefault();
    railFocusEl().dispatchEvent(event);
  });

beforeEach(() => {
  vi.useFakeTimers();
  dismissCount = 0;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.useRealTimers();
});

describe('useRailOpen hover intent', () => {
  it('opens on pointerenter and closes only after the grace elapses', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    expect(open()).toBe('false');

    fire(railEl(), 'pointerenter');
    expect(open()).toBe('true');

    fire(railEl(), 'pointerleave');
    // The grace still holds it open until the timer fires.
    expect(open()).toBe('true');
    act(() => vi.advanceTimersByTime(250));
    expect(open()).toBe('false');
  });

  it('cancels the grace when the pointer re-enters', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    fire(railEl(), 'pointerenter');
    fire(railEl(), 'pointerleave');
    act(() => vi.advanceTimersByTime(100));
    fire(railEl(), 'pointerenter');
    act(() => vi.advanceTimersByTime(250));
    expect(open()).toBe('true');
  });
});

describe('useRailOpen focus within', () => {
  it('opens on focusin and closes on focusout', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    fire(railEl(), 'focusin');
    expect(open()).toBe('true');
    fire(railEl(), 'focusout');
    expect(open()).toBe('false');
  });
});

describe('useRailOpen latches', () => {
  it('stays open while pinned or composing regardless of hover/focus', () => {
    renderHarness({ pinnedOpen: true, composing: false });
    expect(open()).toBe('true');
    renderHarness({ pinnedOpen: false, composing: true });
    expect(open()).toBe('true');
    renderHarness({ pinnedOpen: false, composing: false });
    expect(open()).toBe('false');
  });
});

describe('useRailOpen dismiss', () => {
  it('dismisses on a pointerdown outside the rail but not inside it', () => {
    renderHarness({ pinnedOpen: true, composing: false });
    fire(railEl(), 'pointerdown');
    expect(dismissCount).toBe(0);
    fire(document.body, 'pointerdown');
    expect(dismissCount).toBe(1);
  });

  it('exempts a pointerdown on a rail-anchor control (a chip re-opening the rail)', () => {
    renderHarness({ pinnedOpen: true, composing: false });
    const anchor = document.createElement('button');
    anchor.setAttribute('data-rail-anchor', '');
    document.body.appendChild(anchor);
    fire(anchor, 'pointerdown');
    expect(dismissCount).toBe(0);
    anchor.remove();
  });

  it('does not dismiss an unpinned rail on outside pointerdown', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    fire(document.body, 'pointerdown');
    expect(dismissCount).toBe(0);
  });

  it('dismisses an open unpinned rail on Escape and clears its open intents', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    fire(railEl(), 'pointerenter');
    act(() => railFocusEl().focus());
    expect(open()).toBe('true');

    pressEscape();

    expect(dismissCount).toBe(1);
    expect(open()).toBe('false');
  });

  it('ignores Escape when another handler already prevented it', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    act(() => railFocusEl().focus());
    expect(open()).toBe('true');

    pressEscape(true);

    expect(dismissCount).toBe(0);
    expect(open()).toBe('true');
  });

  it('blurs the rail and closes when the recorded return target is disconnected', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    const returnTarget = document.createElement('button');
    document.body.appendChild(returnTarget);
    act(() => {
      returnTarget.focus();
      railFocusEl().focus();
    });
    expect(open()).toBe('true');
    returnTarget.remove();

    pressEscape();

    expect(dismissCount).toBe(1);
    expect(open()).toBe('false');
    expect(document.activeElement).not.toBe(railFocusEl());
  });
});

describe('useRailOpen container lifecycle', () => {
  it('resets hover and focus intent and clears the grace timer across a remount', () => {
    renderHarness({ pinnedOpen: false, composing: false });
    fire(railEl(), 'pointerenter');
    act(() => railFocusEl().focus());
    fire(railEl(), 'pointerleave');
    expect(open()).toBe('true');
    const timerCountWithGrace = vi.getTimerCount();

    renderHarness({ pinnedOpen: false, composing: false, mounted: false });
    expect(vi.getTimerCount()).toBe(timerCountWithGrace - 1);
    renderHarness({ pinnedOpen: false, composing: false });

    expect(open()).toBe('false');
  });
});
