// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MarginRail } from './MarginRail';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;
let rootMounted: boolean;
let resize: ResizeObserverCallback | null;

class ResizeObserverStub {
  constructor(callback: ResizeObserverCallback) {
    resize = callback;
  }

  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

beforeEach(() => {
  resize = null;
  vi.stubGlobal('ResizeObserver', ResizeObserverStub);
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  rootMounted = true;
});

afterEach(() => {
  if (rootMounted) act(() => root.unmount());
  container.remove();
  document.querySelector('.doc-header')?.remove();
  document.documentElement.style.removeProperty('--rail-top');
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('MarginRail', () => {
  it('tracks the visible masthead bottom on resize and scroll, clamped to the viewport', () => {
    const header = document.createElement('header');
    header.className = 'doc-header';
    document.body.appendChild(header);
    let bottom = 180;
    vi.spyOn(header, 'getBoundingClientRect').mockImplementation(
      () => ({ bottom } as DOMRect),
    );

    act(() => {
      root.render(
        <MarginRail
          open={false}
          projection={{ pinned: null, feed: [] }}
          activeId={null}
          total={0}
          onToggle={() => {}}
          railRef={() => {}}
        >
          <div>Threads</div>
        </MarginRail>,
      );
    });
    expect(document.documentElement.style.getPropertyValue('--rail-top')).toBe('180px');

    bottom = 230;
    act(() => resize?.([], {} as ResizeObserver));
    expect(document.documentElement.style.getPropertyValue('--rail-top')).toBe('230px');

    bottom = -12;
    act(() => window.dispatchEvent(new Event('scroll')));
    expect(document.documentElement.style.getPropertyValue('--rail-top')).toBe('0px');

    act(() => root.unmount());
    rootMounted = false;
    expect(document.documentElement.style.getPropertyValue('--rail-top')).toBe('');
    bottom = 50;
    act(() => window.dispatchEvent(new Event('scroll')));
    expect(document.documentElement.style.getPropertyValue('--rail-top')).toBe('');

  });
});
