// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { BoardBlocks } from './BoardBlocks';
import { emptyState } from '../reduce';
import { BOARD_DWELL_MS, viewedStore } from '../viewed';
import type { Interactions, Verdict } from '../events';
import type { Approval, Block, Card, ChildBlock, Markdown } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// Clamped (mounted under every reply row) constructs a ResizeObserver on mount.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

class IntersectionObserverStub {
  static live = new Set<IntersectionObserverStub>();
  private readonly targets = new Set<Element>();
  constructor(
    private readonly callback: IntersectionObserverCallback,
    private readonly init: IntersectionObserverInit = {},
  ) {}
  observe(el: Element): void {
    this.targets.add(el);
    IntersectionObserverStub.live.add(this);
  }
  disconnect(): void {
    IntersectionObserverStub.live.delete(this);
  }
  // emit plays one geometry for el against every live observer: the ratio observer
  // reads `ratio`, the middle-line observer reads whether el spans that line.
  static emit(el: Element, ratio: number, spansMiddle: boolean): void {
    for (const obs of [...IntersectionObserverStub.live].filter((o) => o.targets.has(el))) {
      const middle = obs.init.rootMargin !== undefined;
      const entry = {
        isIntersecting: middle ? spansMiddle : ratio > 0,
        intersectionRatio: middle ? (spansMiddle ? 1 : 0) : ratio,
      } as IntersectionObserverEntry;
      act(() => obs.callback([entry], obs as unknown as IntersectionObserver));
    }
  }
}
(globalThis as { IntersectionObserver?: unknown }).IntersectionObserver = IntersectionObserverStub;

const approval = (id: string): Approval => ({ id, type: 'approval', prompt: `Approve ${id}` });
const card = (id: string, children: ChildBlock[]): Card => ({ id, type: 'card', children });
const markdown = (id: string): Markdown => ({ id, type: 'markdown', md: 'note' });
const section = (id: string, title: string): Block => ({ id, type: 'section', title });

const empty = (): Interactions => emptyState().interactions;
const withVerdicts = (over: Record<string, Verdict>): Interactions => ({
  ...empty(),
  decisions: Object.fromEntries(Object.entries(over).map(([id, verdict]) => [id, { verdict }])),
});

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  viewedStore.reset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function render(blocks: Block[], interactions: Interactions): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={blocks} interactions={interactions} closed={present.closed} round={present.currentRound}>
          <BoardBlocks blocks={blocks} interactions={interactions} packInteractive={new Set()} />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

function row(id: string): HTMLElement {
  return container.querySelector<HTMLElement>(`.block-row[data-flip-key="${id}"]`)!;
}

function marked(id: string): boolean {
  const row = container.querySelector<HTMLElement>(`.block-row[data-flip-key="${id}"]`);
  expect(row).not.toBeNull();
  return row!.hasAttribute('data-decided');
}

describe('BoardBlocks data-decided attribution', () => {
  it('marks a decided approval row', () => {
    render([approval('a1')], withVerdicts({ a1: 'approved' }));
    expect(marked('a1')).toBe(true);
  });

  it('leaves an undecided approval row unmarked', () => {
    render([approval('a1')], empty());
    expect(marked('a1')).toBe(false);
  });

  it('leaves a card with mixed decided/undecided decidables unmarked', () => {
    render([card('c1', [approval('a1'), approval('a2')])], withVerdicts({ a1: 'approved' }));
    expect(marked('c1')).toBe(false);
  });

  it('marks a card once every decidable child is decided', () => {
    render([card('c1', [approval('a1'), approval('a2')])], withVerdicts({ a1: 'approved', a2: 'rejected' }));
    expect(marked('c1')).toBe(true);
  });

  it('never marks a row with no decidables', () => {
    render([markdown('m1')], empty());
    expect(marked('m1')).toBe(false);
  });
});

describe('BoardBlocks section hoisting', () => {
  it('renders a section as a bare child, never inside a flip-tracked row', () => {
    render([section('s1', 'Intro'), approval('a1')], empty());
    const sec = container.querySelector('.doc-section');
    expect(sec).not.toBeNull();
    // the section leaves the FLIP set: no row wraps it and none carries its key
    expect(sec!.closest('.block-row')).toBeNull();
    expect(container.querySelector('.block-row[data-flip-key="s1"]')).toBeNull();
    // a non-section block still gets its flip-tracked row
    expect(container.querySelector('.block-row[data-flip-key="a1"]')).not.toBeNull();
  });
});

describe('BoardBlocks viewed tracking', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('marks a card and its children once the row stays half visible for the dwell', () => {
    render([card('c1', [approval('a1')]), markdown('m1')], empty());
    IntersectionObserverStub.emit(row('c1'), 0.6, false);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS - 1));
    expect(viewedStore.viewed()).toEqual([]);
    act(() => vi.advanceTimersByTime(1));
    expect(viewedStore.viewed()).toEqual(['c1', 'a1']);
  });

  it('cancels the dwell when the row scrolls away first', () => {
    render([markdown('m1')], empty());
    IntersectionObserverStub.emit(row('m1'), 0.6, false);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS / 2));
    IntersectionObserverStub.emit(row('m1'), 0.2, false);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS));
    expect(viewedStore.viewed()).toEqual([]);
  });

  it('counts a row many viewports tall once it spans the middle of the viewport', () => {
    render([markdown('m1')], empty());
    IntersectionObserverStub.emit(row('m1'), 0.0001, false);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS));
    expect(viewedStore.viewed()).toEqual([]);
    IntersectionObserverStub.emit(row('m1'), 0.012, true);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS));
    expect(viewedStore.viewed()).toEqual(['m1']);
  });

  it('keeps the dwell running while either test still holds', () => {
    render([markdown('m1')], empty());
    IntersectionObserverStub.emit(row('m1'), 0.6, true);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS / 2));
    IntersectionObserverStub.emit(row('m1'), 0.4, true);
    act(() => vi.advanceTimersByTime(BOARD_DWELL_MS / 2));
    expect(viewedStore.viewed()).toEqual(['m1']);
  });
});
