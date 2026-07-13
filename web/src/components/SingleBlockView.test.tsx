// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import { EventStreamProvider } from '../stream';
import { presentKey, queryClient } from '../api';
import { reduce } from '../reduce';
import { SingleBlockView } from './SingleBlockView';
import { packToast, setPackToastSink } from '../packs/toasts';
import type { PresentEvent, PresentState } from '../events';
import type { Doc, Markdown } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// jsdom ships no ResizeObserver (Clamped, under every reply row, constructs one)
// and no EventSource (EventStreamProvider opens one on mount). Stub both.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
class EventSourceStub {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {}
  addEventListener(): void {}
  close(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;
(globalThis as { EventSource?: unknown }).EventSource = EventSourceStub;

const markdownBlock = (id: string, md: string): Markdown => ({ id, type: 'markdown', md });
const doc = (blocks: Markdown[]): Doc => ({ version: 1, title: '', blocks });

// historyState is the regression case: block h1 lives only in a closed round's
// frozen blocks (absent from the live doc), while its agent reply persists in
// live interactions. Live block l1 is the current round.
function historyState(): PresentState {
  const events: PresentEvent[] = [
    { origin: 'agent', type: 'doc.replaced', seq: 1, payload: { doc: doc([markdownBlock('h1', 'historical block')]), revision: 1 } },
    { origin: 'agent', type: 'reply.created', seq: 2, payload: { id: 'r1', blockId: 'h1', md: 'reply on the historical block' } },
    { origin: 'agent', type: 'round.started', seq: 3, payload: {} },
    { origin: 'agent', type: 'doc.replaced', seq: 4, payload: { doc: doc([markdownBlock('l1', 'live block')]), revision: 2 } },
  ];
  return reduce(events);
}

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
  queryClient.clear();
  setPackToastSink(null);
});

function render(subject: string, blockId: string, state: PresentState): void {
  queryClient.setQueryData(presentKey(subject), state);
  act(() => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <EventStreamProvider subject={subject}>
          <SingleBlockView subject={subject} blockId={blockId} />
        </EventStreamProvider>
      </QueryClientProvider>,
    );
  });
}

describe('SingleBlockView closed-round fallback', () => {
  it('renders a block that survives only in a closed round, with its reply thread', () => {
    const state = historyState();
    expect(state.doc.blocks.some((b) => b.id === 'h1')).toBe(false);
    expect(state.rounds.history[0]?.blocks.some((b) => b.id === 'h1')).toBe(true);

    render('s', 'h1', state);

    const markdown = container.querySelector('.markdown-block');
    expect(markdown).not.toBeNull();
    expect(container.textContent).toContain('historical block');

    const thread = container.querySelector('.thread');
    expect(thread).not.toBeNull();
    expect(thread?.querySelectorAll('.reply-item')).toHaveLength(1);
    expect(container.textContent).toContain('reply on the historical block');

    expect(container.textContent).not.toContain('No block');
    expect(container.textContent).not.toContain('live block');
  });

  it('renders the live block when the id is in the current doc', () => {
    render('s', 'l1', historyState());

    expect(container.querySelector('.markdown-block')).not.toBeNull();
    expect(container.textContent).toContain('live block');
    expect(container.textContent).not.toContain('No block');
    expect(container.textContent).not.toContain('historical block');
  });
});

describe('SingleBlockView pack toasts', () => {
  it('registers the toast sink and flows the stack inside the measured root', () => {
    render('s', 'l1', historyState());
    act(() => packToast({ kind: 'info', text: 'draft saved' }));

    const stack = container.querySelector('.single-block .toast-stack');
    expect(stack).not.toBeNull();
    expect(stack?.textContent).toContain('draft saved');
    // The stack is a child of the ResizeObserver'd root, never a sibling escaping it.
    const escaped = container.querySelector('.single-block')?.contains(container.querySelector('.toast-stack'));
    expect(escaped).toBe(true);
  });
});
