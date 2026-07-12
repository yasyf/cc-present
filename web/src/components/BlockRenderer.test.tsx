// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { BlockRenderer } from './BlockRenderer';
import { emptyState } from '../reduce';
import type { Interactions, Reply } from '../events';
import type { Approval, Block, Input, Markdown } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// jsdom ships no ResizeObserver; Clamped (used by every reply row) constructs one
// on mount, so stub the boundary the layout effect measures against.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

const markdownBlock = (id: string, md: string): Markdown => ({ id, type: 'markdown', md });
const approvalBlock = (id: string): Approval => ({ id, type: 'approval' });
const inputBlock = (id: string, label: string): Input => ({ id, type: 'input', label });

function withReplies(replies: Record<string, Reply[]>): Interactions {
  return { ...emptyState().interactions, replies };
}

function api(over: Partial<PresentApi> = {}): PresentApi {
  return { post: async () => true, closed: false, currentRound: 1, ...over };
}

function Board({ blocks, interactions }: { blocks: Block[]; interactions: Interactions }) {
  const present = api();
  return (
    <PresentContext.Provider value={present}>
      <KeyboardProvider blocks={blocks} interactions={interactions} closed={present.closed} round={present.currentRound}>
        {blocks.map((b) => (
          <BlockRenderer key={b.id} block={b} interactions={interactions} />
        ))}
      </KeyboardProvider>
    </PresentContext.Provider>
  );
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
});

function render(blocks: Block[], interactions: Interactions): void {
  act(() => root.render(<Board blocks={blocks} interactions={interactions} />));
}

describe('BlockRenderer reply threading', () => {
  it('threads an agent reply under a markdown block, rendering its markdown', () => {
    const interactions = withReplies({
      m1: [
        { id: 'r1', md: '**bold** reply' },
        { id: 'r2', md: 'plain reply' },
      ],
    });
    render([markdownBlock('m1', 'hello world')], interactions);

    const thread = container.querySelector('.thread');
    expect(container.querySelectorAll('.thread')).toHaveLength(1);
    expect(thread?.querySelectorAll('.reply-item')).toHaveLength(2);
    expect(container.querySelector('.reply-item strong')?.textContent).toBe('bold');
    expect(container.textContent).toContain('plain reply');
  });

  it('renders no thread for a block without replies', () => {
    render([markdownBlock('m1', 'hi')], emptyState().interactions);

    expect(container.querySelector('.thread')).toBeNull();
    expect(container.querySelector('.markdown-block')).not.toBeNull();
  });

  it('renders exactly one thread for an approval block with replies (no double render)', () => {
    render([approvalBlock('a1')], withReplies({ a1: [{ id: 'r1', md: 'agent reply' }] }));

    expect(container.querySelectorAll('.thread')).toHaveLength(1);
    expect(container.querySelectorAll('.reply-item')).toHaveLength(1);
    expect(container.textContent).toContain('agent reply');
  });

  it('threads replies after an input block, following its label', () => {
    render([inputBlock('i1', 'Your name')], withReplies({ i1: [{ id: 'r1', md: 'agent note' }] }));

    const label = container.querySelector('.input-block');
    const thread = container.querySelector('.thread');
    expect(label).not.toBeNull();
    expect(thread).not.toBeNull();
    expect(thread?.querySelectorAll('.reply-item')).toHaveLength(1);
    expect(container.textContent).toContain('agent note');
    const following = label!.compareDocumentPosition(thread!) & Node.DOCUMENT_POSITION_FOLLOWING;
    expect(following).toBeTruthy();
  });
});
