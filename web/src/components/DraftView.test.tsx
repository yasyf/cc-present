// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { DraftView } from './DraftView';
import { emptyState } from '../reduce';
import { anchorOf, formatRangeAnchor } from '../anchor';
import type { Annotation, Interactions } from '../events';
import type { Draft } from '../schema';

// Shiki is heavy and irrelevant to the annotation logic; a null lang skips it so
// rows render as raw text.
vi.mock('../highlight', () => ({
  resolveLang: () => null,
  tokenizeLines: async () => [],
}));

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
if (!globalThis.crypto?.randomUUID) {
  (globalThis as { crypto: Crypto }).crypto = { randomUUID: () => 'uuid-fixed' } as unknown as Crypto;
}
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

const empty = (): Interactions => emptyState().interactions;
const TEXT = 'line one\nline two\nline three';
const draft = (over: Partial<Draft> = {}): Draft => ({ id: 'd1', type: 'draft', lang: 'text', text: TEXT, ...over });

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

function renderCapturing(block: Draft, interactions: Interactions, closed = false): unknown[] {
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
          <DraftView block={block} interactions={interactions} />
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

function mouseDown(el: Element, shiftKey = false): void {
  act(() => el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, shiftKey })));
}

function gutter(n: number): HTMLButtonElement {
  return container.querySelector(`.draft-row[data-line="${n}"] .draft-gutter`) as HTMLButtonElement;
}

describe('DraftView annotation composer', () => {
  it('posts a single-line annotation with the ranged anchor and stamped quote', () => {
    const posted = renderCapturing(draft(), empty());
    mouseDown(gutter(2));
    const ta = container.querySelector('.draft-composer-input') as HTMLTextAreaElement;
    expect(container.querySelector('.draft-composer-range')?.textContent).toBe('Line 2');
    setValue(ta, 'needs a citation');
    act(() => (container.querySelector('.draft-composer .primary') as HTMLElement).click());
    expect(posted).toHaveLength(1);
    expect(posted[0]).toMatchObject({
      type: 'annotation.created',
      blockId: 'd1',
      anchor: formatRangeAnchor(2, 2, anchorOf('line two')),
      text: 'needs a citation',
      quote: 'line two',
    });
    expect(typeof (posted[0] as { id: string }).id).toBe('string');
  });

  it('sweeps a shift-click range and stamps the multi-line quote', () => {
    const posted = renderCapturing(draft(), empty());
    mouseDown(gutter(1));
    mouseDown(gutter(3), true);
    expect(container.querySelector('.draft-composer-range')?.textContent).toBe('Lines 1–3');
    setValue(container.querySelector('.draft-composer-input') as HTMLTextAreaElement, 'span note');
    act(() => (container.querySelector('.draft-composer .primary') as HTMLElement).click());
    expect(posted[0]).toMatchObject({
      anchor: formatRangeAnchor(1, 3, anchorOf('line one')),
      quote: 'line one\nline two\nline three',
    });
  });
});

describe('DraftView re-anchoring', () => {
  it('marks an annotation whose anchored line moved with a "was L<n>" tag', () => {
    // Anchor points at line 1 but its content now lives at line 3 → resolves moved.
    const ann: Annotation = { id: 'a1', anchor: formatRangeAnchor(1, 1, anchorOf('gamma')), text: 'moved note', quote: 'gamma' };
    renderCapturing(draft({ text: 'alpha\nbeta\ngamma' }), { ...empty(), annotations: { d1: [ann] } });
    const note = container.querySelector('.draft-note') as HTMLElement;
    expect(note.querySelector('.draft-note-line')?.textContent).toBe('L3');
    expect(note.querySelector('.draft-note-moved')?.textContent).toBe('moved · was L1');
    expect(container.querySelector('.draft-detached')).toBeNull();
  });

  it('drops an unresolvable annotation into the Detached notes section with its quote', () => {
    const ann: Annotation = {
      id: 'a1',
      anchor: formatRangeAnchor(1, 1, anchorOf('a line that is gone')),
      text: 'orphaned note',
      quote: 'a line that is gone',
    };
    renderCapturing(draft({ text: 'alpha\nbeta' }), { ...empty(), annotations: { d1: [ann] } });
    const detached = container.querySelector('.draft-detached') as HTMLElement;
    expect(detached.querySelector('.draft-detached-head')?.textContent).toBe('Detached notes');
    expect(detached.querySelector('.draft-note-quote')?.textContent).toBe('a line that is gone');
    expect(detached.querySelector('.draft-note-text')?.textContent).toBe('orphaned note');
    expect(detached.querySelector('.link-btn')).not.toBeNull();
  });
});

describe('DraftView edit and remove', () => {
  const seeded = (): Interactions => ({
    ...empty(),
    annotations: {
      d1: [{ id: 'a1', anchor: formatRangeAnchor(2, 2, anchorOf('line two')), text: 'old', quote: 'line two' }],
    },
  });

  it('re-posts the same annotation id when an existing note is edited', () => {
    const posted = renderCapturing(draft(), seeded());
    const editBtn = [...container.querySelectorAll('.draft-note-actions .link-btn')].find(
      (b) => b.textContent === 'Edit',
    ) as HTMLElement;
    act(() => editBtn.click());
    const ta = container.querySelector('.draft-composer-input') as HTMLTextAreaElement;
    expect(ta.value).toBe('old');
    setValue(ta, 'revised');
    act(() => (container.querySelector('.draft-composer .primary') as HTMLElement).click());
    expect(posted[0]).toMatchObject({ type: 'annotation.created', id: 'a1', text: 'revised' });
  });

  it('posts annotation.removed for a note', () => {
    const posted = renderCapturing(draft(), seeded());
    const removeBtn = [...container.querySelectorAll('.draft-note-actions .link-btn')].find(
      (b) => b.textContent === 'Remove',
    ) as HTMLElement;
    act(() => removeBtn.click());
    expect(posted[0]).toEqual({ type: 'annotation.removed', id: 'a1', blockId: 'd1' });
  });
});

describe('DraftView locked', () => {
  it('disables gutters and hides the composer and note actions when closed', () => {
    const withNote: Interactions = {
      ...empty(),
      annotations: {
        d1: [{ id: 'a1', anchor: formatRangeAnchor(2, 2, anchorOf('line two')), text: 'note', quote: 'line two' }],
      },
    };
    renderCapturing(draft(), withNote, true);
    expect((gutter(1) as HTMLButtonElement).disabled).toBe(true);
    mouseDown(gutter(1));
    expect(container.querySelector('.draft-composer')).toBeNull();
    expect(container.querySelector('.draft-note-actions')).toBeNull();
  });
});
