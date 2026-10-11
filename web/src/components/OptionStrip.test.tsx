// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, createRef } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { OptionStrip, inView } from './OptionStrip';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const observers: { callback: ResizeObserverCallback; observed: Element[] }[] = [];
globalThis.ResizeObserver = class {
  private observed: Element[] = [];
  constructor(callback: ResizeObserverCallback) {
    observers.push({ callback, observed: this.observed });
  }
  observe(el: Element) {
    this.observed.push(el);
  }
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

function resize(el: Element): void {
  act(() => {
    for (const { callback, observed } of observers) {
      if (observed.includes(el)) callback([], {} as ResizeObserver);
    }
  });
}

function lay(el: Element, box: { offsetLeft?: number; offsetWidth?: number; clientWidth?: number }): void {
  for (const [key, value] of Object.entries(box)) Object.defineProperty(el, key, { value, configurable: true });
}

describe('inView', () => {
  const cases: { name: string; left: number; width: number; scrollLeft: number; clientWidth: number; want: boolean }[] = [
    { name: 'a card fully inside the window', left: 0, width: 200, scrollLeft: 0, clientWidth: 500, want: true },
    { name: 'a card exactly half past the trailing edge', left: 400, width: 200, scrollLeft: 0, clientWidth: 500, want: true },
    { name: 'a card peeking under half at the trailing edge', left: 420, width: 200, scrollLeft: 0, clientWidth: 500, want: false },
    { name: 'a card exactly half scrolled off the leading edge', left: 0, width: 200, scrollLeft: 100, clientWidth: 500, want: true },
    { name: 'a card mostly scrolled off the leading edge', left: 0, width: 200, scrollLeft: 120, clientWidth: 500, want: false },
    { name: 'a card wholly past the trailing edge', left: 600, width: 200, scrollLeft: 0, clientWidth: 500, want: false },
    { name: 'a card wholly before the leading edge', left: 0, width: 200, scrollLeft: 400, clientWidth: 500, want: false },
    { name: 'the last card at the scroll end', left: 880, width: 200, scrollLeft: 580, clientWidth: 500, want: true },
    { name: 'a zero-width card (no layout)', left: 0, width: 0, scrollLeft: 0, clientWidth: 0, want: true },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(inView(c.left, c.width, c.scrollLeft, c.clientWidth)).toBe(c.want);
    });
  }
});

describe('OptionStrip position dots', () => {
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
    observers.length = 0;
  });

  const lit = (): boolean[] => [...container.querySelectorAll('.strip-dot')].map((dot) => dot.classList.contains('on'));

  function mount(): { scroller: HTMLElement; cards: HTMLElement[] } {
    act(() =>
      root.render(
        <OptionStrip strip role="radiogroup" cardCount={3} picked={new Set()} containerRef={createRef<HTMLDivElement>()}>
          <div className="option" />
          <div className="option" />
          <div className="option" />
        </OptionStrip>,
      ),
    );
    const scroller = container.querySelector('.options') as HTMLElement;
    return { scroller, cards: [...scroller.children] as HTMLElement[] };
  }

  it('lights the cards in view again when the scroller resizes', () => {
    const { scroller, cards } = mount();
    expect(lit()).toEqual([true, true, true]);

    cards.forEach((card, i) => lay(card, { offsetLeft: i * 220, offsetWidth: 200 }));
    lay(scroller, { clientWidth: 420 });
    resize(scroller);
    expect(lit()).toEqual([true, true, false]);

    lay(scroller, { clientWidth: 200 });
    resize(scroller);
    expect(lit()).toEqual([true, false, false]);
  });

  it('lights the cards in view again when a card resizes', () => {
    const { scroller, cards } = mount();
    cards.forEach((card, i) => lay(card, { offsetLeft: i * 220, offsetWidth: 200 }));
    lay(scroller, { clientWidth: 640 });
    resize(cards[2]!);
    expect(lit()).toEqual([true, true, true]);

    lay(cards[2]!, { offsetLeft: 440, offsetWidth: 600 });
    resize(cards[2]!);
    expect(lit()).toEqual([true, true, false]);
  });
});
