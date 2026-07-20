// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { Triage } from './Triage';
import { isDecided, submitItems } from '../decide';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { Triage as TriageBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
Element.prototype.scrollIntoView = () => {};
window.matchMedia = ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener() {},
  removeEventListener() {},
  addListener() {},
  removeListener() {},
  dispatchEvent: () => false,
})) as typeof window.matchMedia;
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

const empty = (): Interactions => emptyState().interactions;
const triage = (id: string, labels: string[], over: Partial<TriageBlock> = {}): TriageBlock => ({
  id,
  type: 'triage',
  items: labels.map((label, i) => ({ id: `i${i}`, label })),
  ...over,
});
const withTriage = (block: TriageBlock, map: Interactions['triage']): Interactions => ({ ...empty(), triage: map });

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

function renderCapturing(block: TriageBlock, interactions: Interactions, closed = false): unknown[] {
  const posted: unknown[] = [];
  const present: PresentApi = {
    post: async (ev) => {
      posted.push(ev);
      return true;
    },
    closed,
    currentRound: 1,
  };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[block]} interactions={interactions} closed={closed} round={1}>
          <Triage block={block} interactions={interactions} />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
  return posted;
}

function setValue(el: HTMLTextAreaElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function row(i: number): HTMLElement {
  return container.querySelectorAll('.triage-item')[i] as HTMLElement;
}

describe('Triage per-item verdict', () => {
  it('posts a one-key merge for a single item verdict', () => {
    const posted = renderCapturing(triage('t1', ['First', 'Second']), empty());
    act(() => (row(0).querySelector('.verdict-approve') as HTMLElement).click());
    expect(posted[0]).toEqual({ type: 'triage.decided', blockId: 't1', verdicts: { i0: { verdict: 'approved' } } });
  });

  it('re-presses the same verdict to clear it', () => {
    const posted = renderCapturing(triage('t1', ['First']), withTriage(triage('t1', ['First']), { t1: { i0: { verdict: 'approved' } } }));
    act(() => (row(0).querySelector('.verdict-approve') as HTMLElement).click());
    expect(posted[0]).toEqual({ type: 'triage.decided', blockId: 't1', verdicts: { i0: { verdict: 'cleared' } } });
  });

  it('carries an existing note forward when the verdict flips', () => {
    const block = triage('t1', ['First'], { allowNotes: true });
    const posted = renderCapturing(block, withTriage(block, { t1: { i0: { verdict: 'approved', note: 'looks fine' } } }));
    act(() => (row(0).querySelector('.verdict-reject') as HTMLElement).click());
    expect(posted[0]).toEqual({
      type: 'triage.decided',
      blockId: 't1',
      verdicts: { i0: { verdict: 'rejected', note: 'looks fine' } },
    });
  });
});

describe('Triage bulk', () => {
  it('accepts all in one merge, carrying prior notes forward', () => {
    const block = triage('t1', ['First', 'Second', 'Third'], { allowNotes: true });
    const posted = renderCapturing(block, withTriage(block, { t1: { i1: { verdict: 'rejected', note: 'nope' } } }));
    const acceptAll = [...container.querySelectorAll('.triage-bulk-btn')].find((b) => b.textContent === 'Accept all') as HTMLElement;
    act(() => acceptAll.click());
    expect(posted[0]).toEqual({
      type: 'triage.decided',
      blockId: 't1',
      verdicts: {
        i0: { verdict: 'approved' },
        i1: { verdict: 'approved', note: 'nope' },
        i2: { verdict: 'approved' },
      },
    });
  });

  it('shows an N of M decided count', () => {
    const block = triage('t1', ['A', 'B', 'C']);
    renderCapturing(block, withTriage(block, { t1: { i0: { verdict: 'approved' }, i1: { verdict: 'rejected' } } }));
    expect(container.querySelector('.triage-progress')?.textContent).toBe('2 of 3 decided');
  });
});

describe('Triage note gating', () => {
  it('offers a note only once an item has a verdict', () => {
    const block = triage('t1', ['A'], { allowNotes: true });
    renderCapturing(block, empty());
    expect(container.querySelector('.triage-note-affordance')).toBeNull();

    renderCapturing(block, withTriage(block, { t1: { i0: { verdict: 'approved' } } }));
    expect(container.querySelector('.triage-note-affordance .link-btn')?.textContent).toBe('Add note');
  });

  it('hides the note channel entirely when allowNotes is false', () => {
    const block = triage('t1', ['A'], { allowNotes: false });
    renderCapturing(block, withTriage(block, { t1: { i0: { verdict: 'approved' } } }));
    expect(container.querySelector('.triage-note-affordance')).toBeNull();
  });

  it('posts the verdict with the note when a note is saved', () => {
    const block = triage('t1', ['A'], { allowNotes: true });
    const posted = renderCapturing(block, withTriage(block, { t1: { i0: { verdict: 'rejected' } } }));
    act(() => (container.querySelector('.triage-note-affordance .link-btn') as HTMLElement).click());
    setValue(container.querySelector('.triage-note-input') as HTMLTextAreaElement, 'flaky on CI');
    act(() => (container.querySelector('.triage-note-actions .primary') as HTMLElement).click());
    expect(posted[0]).toEqual({
      type: 'triage.decided',
      blockId: 't1',
      verdicts: { i0: { verdict: 'rejected', note: 'flaky on CI' } },
    });
  });
});

describe('Triage locked', () => {
  it('disables verdicts and hides bulk and notes when closed', () => {
    const block = triage('t1', ['A']);
    renderCapturing(block, withTriage(block, { t1: { i0: { verdict: 'approved' } } }), true);
    expect((row(0).querySelector('.verdict-approve') as HTMLButtonElement).disabled).toBe(true);
    expect(container.querySelector('.triage-bulk')).toBeNull();
    expect(container.querySelector('.triage-note-affordance')).toBeNull();
  });
});

describe('Triage decide semantics', () => {
  it('is decided only when every item has a verdict, and tallies as one block entry', () => {
    const block = triage('t1', ['A', 'B']);
    expect(isDecided(block, empty())).toBe(false);
    const partial = withTriage(block, { t1: { i0: { verdict: 'approved' } } });
    expect(isDecided(block, partial)).toBe(false);
    const all = withTriage(block, { t1: { i0: { verdict: 'approved' }, i1: { verdict: 'rejected' } } });
    expect(isDecided(block, all)).toBe(true);
    expect(submitItems([block], all, new Set())).toEqual([{ id: 't1', kind: 'triage', decided: true }]);
  });
});
