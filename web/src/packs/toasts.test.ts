// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, createElement, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { packToast, setPackToastSink, usePackToastSink } from './toasts';
import type { PackToast } from './toasts';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

afterEach(() => setPackToastSink(null));

describe('pack toasts', () => {
  it('throws before a sink is registered (fail loud)', () => {
    setPackToastSink(null);
    expect(() => packToast({ kind: 'info', text: 'x' })).toThrow(/before the shell mounted/);
  });

  it('delivers each toast to the registered sink in order', () => {
    const seen: PackToast[] = [];
    setPackToastSink((t) => seen.push(t));
    packToast({ kind: 'info', text: 'saved' });
    packToast({ kind: 'error', text: 'boom' });
    expect(seen).toEqual([
      { kind: 'info', text: 'saved' },
      { kind: 'error', text: 'boom' },
    ]);
  });

  it('throws again once the sink is cleared', () => {
    const seen: PackToast[] = [];
    setPackToastSink((t) => seen.push(t));
    setPackToastSink(null);
    expect(() => packToast({ kind: 'info', text: 'x' })).toThrow();
    expect(seen).toEqual([]);
  });
});

describe('usePackToastSink', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });
  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  // ToastOnMount stands in for a pack that raises ui.toast from its own mount
  // effect, which React runs before any ancestor's effect.
  function ToastOnMount() {
    useEffect(() => packToast({ kind: 'info', text: 'on mount' }), []);
    return null;
  }

  it('registers the sink before descendants mount, so a mount-effect toast lands', () => {
    const seen: PackToast[] = [];
    function Surface() {
      usePackToastSink((t) => seen.push(t));
      return createElement(ToastOnMount);
    }
    act(() => root.render(createElement(Surface)));
    expect(seen).toEqual([{ kind: 'info', text: 'on mount' }]);
  });

  it('clears the sink when the surface unmounts', () => {
    const local = createRoot(document.createElement('div'));
    function Surface() {
      usePackToastSink(() => {});
      return null;
    }
    act(() => local.render(createElement(Surface)));
    act(() => local.unmount());
    expect(() => packToast({ kind: 'info', text: 'x' })).toThrow(/before the shell mounted/);
  });
});
