// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { BlockRenderer } from './BlockRenderer';
import { ThreadHostContext } from './threadHost';
import type { ThreadHost } from './threadHost';
import { emptyState } from '../reduce';
import { markPacksLoaded, registerPack, resetPacksForTest } from '../packs/registry';
import type { Interactions, Reply } from '../events';
import type { Approval, Block, Card, ChildBlock, Input, Markdown, PackBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// jsdom ships no ResizeObserver; Clamped (the markdown block's body) constructs one
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
const cardBlock = (id: string, children: ChildBlock[]): Card => ({ id, type: 'card', children });

function withReplies(replies: Record<string, Reply[]>): Interactions {
  return { ...emptyState().interactions, replies };
}

function api(over: Partial<PresentApi> = {}): PresentApi {
  return { post: async () => true, closed: false, currentRound: 1, ...over };
}

interface BoardProps {
  blocks: Block[];
  interactions: Interactions;
  note?: boolean;
  host?: ThreadHost;
  closed?: boolean;
}

function Board({ blocks, interactions, note, host = 'inline', closed = false }: BoardProps) {
  const present = api({ closed });
  return (
    <PresentContext.Provider value={present}>
      <ThreadHostContext.Provider value={host}>
        <KeyboardProvider blocks={blocks} interactions={interactions} closed={present.closed} round={present.currentRound}>
          {blocks.map((b) => (
            <BlockRenderer key={b.id} block={b} interactions={interactions} note={note} />
          ))}
        </KeyboardProvider>
      </ThreadHostContext.Provider>
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
  resetPacksForTest();
});

function render(blocks: Block[], interactions: Interactions, note?: boolean, over: Partial<BoardProps> = {}): void {
  act(() => root.render(<Board blocks={blocks} interactions={interactions} note={note} {...over} />));
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

  it('renders exactly one thread for an interactive pack block with replies', () => {
    registerPack(
      {
        name: 'ex',
        version: '0',
        description: '',
        bundle: '/packs/ex/dist/pack.js',
        blocks: [{ type: 'ex.fork', interactive: true, optional: false, schema: {} }],
      },
      { fork: () => <span>fork-body</span> },
    );
    markPacksLoaded();
    const fork = { id: 'k1', type: 'ex.fork' } as PackBlock;
    render([fork], withReplies({ k1: [{ id: 'r1', md: 'agent reply' }] }));

    expect(container.textContent).toContain('fork-body');
    expect(container.querySelectorAll('.thread')).toHaveLength(1);
    expect(container.querySelectorAll('.reply-item')).toHaveLength(1);
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

describe('BlockRenderer note channel', () => {
  it('renders no note control unless the caller asks for one', () => {
    render([markdownBlock('m1', 'hi')], emptyState().interactions);
    expect(container.querySelector('.feedback-affordance')).toBeNull();
  });

  it('places the note control between the block and its replies', () => {
    render([markdownBlock('m1', 'hi')], withReplies({ m1: [{ id: 'r1', md: 'agent reply' }] }), true);
    const body = container.querySelector('.markdown-block')!;
    const note = container.querySelector('.feedback-affordance')!;
    const reply = container.querySelector('.reply-item')!;
    expect(note.querySelector('.link-btn')!.textContent).toBe('Add note');
    expect(body.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(note.compareDocumentPosition(reply) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('adds no second note control to a block that owns its thread', () => {
    render([approvalBlock('a1')], emptyState().interactions, true);
    expect(container.querySelectorAll('.feedback-affordance')).toHaveLength(1);
  });

  it('renders each reply once beside the inline note control', () => {
    render([markdownBlock('m1', 'hi')], withReplies({ m1: [{ id: 'r1', md: 'agent reply' }] }), true);
    expect(container.querySelectorAll('.reply-item')).toHaveLength(1);
  });

  it('counts a lone agent reply on the rail chip, and keeps the chip once the board closes', () => {
    const replied = withReplies({ m1: [{ id: 'r1', md: 'agent reply' }] });
    render([markdownBlock('m1', 'hi')], replied, true, { host: 'rail' });
    expect(container.querySelector('.comment-chip')?.textContent).toBe('1 comment');
    expect(container.querySelectorAll('.reply-item')).toHaveLength(1);

    render([markdownBlock('m1', 'hi')], replied, true, { host: 'rail', closed: true });
    expect(container.querySelector('.comment-chip')?.textContent).toBe('1 comment');
  });

  it('counts the notes and replies on a card child that owns no thread on the card chip', () => {
    const interactions: Interactions = {
      ...withReplies({ m1: [{ id: 'r1', md: 'agent reply' }] }),
      feedback: { m1: [{ id: 'f1', text: 'on the child' }] },
    };
    render([cardBlock('k1', [markdownBlock('m1', 'hi')])], interactions, true, { host: 'rail' });
    expect(container.querySelector('.comment-chip')?.textContent).toBe('2 comments');
  });

  it('gives a card decision no note control of its own and counts its notes on the card chip', () => {
    const interactions: Interactions = {
      ...emptyState().interactions,
      feedback: { a1: [{ id: 'f1', text: 'on the decision' }] },
    };
    render([cardBlock('k1', [approvalBlock('a1')])], interactions, true, { host: 'rail' });
    expect([...container.querySelectorAll('.comment-chip')].map((chip) => chip.textContent)).toEqual(['1 comment']);
    expect(container.querySelector('.approval .comment-chip')).toBeNull();
  });

  it('shows the note each inline reply answers under the rail, once per note', () => {
    const interactions: Interactions = {
      ...withReplies({
        k1: [
          { id: 'r1', md: 'first answer', re: 'f1' },
          { id: 'r2', md: 'correction', re: 'f1' },
        ],
      }),
      feedback: { m1: [{ id: 'f1', text: 'what is a link slot?' }] },
    };
    render([cardBlock('k1', [markdownBlock('m1', 'hi')])], interactions, true, { host: 'rail' });
    const exchanges = [...container.querySelectorAll('.thread .note-thread')].map((n) =>
      [...n.querySelectorAll('.thread-text')].map((t) => t.textContent!.trim()),
    );
    expect(exchanges).toEqual([['what is a link slot?', 'first answer'], ['correction']]);
  });

  it('folds all but the newest two inline replies behind Show N earlier', () => {
    const replies = ['r1', 'r2', 'r3', 'r4'].map((id) => ({ id, md: id }));
    render([markdownBlock('m1', 'hi')], withReplies({ m1: replies }), true, { host: 'rail' });
    const texts = () => [...container.querySelectorAll('.reply-item .thread-text')].map((t) => t.textContent!.trim());
    expect(texts()).toEqual(['r3', 'r4']);
    const more = container.querySelector('.note-replies-more') as HTMLElement;
    expect(more.textContent).toBe('Show 2 earlier');
    act(() => more.click());
    expect(texts()).toEqual(['r1', 'r2', 'r3', 'r4']);
  });
});
