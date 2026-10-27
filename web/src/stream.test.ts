// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { TOAST_TEXT, toastFor } from './stream';
import { emptyState } from './reduce';
import { setThreadOpener } from './threadOpen';
import type { PresentState, WireFrame } from './events';
import type { Block } from './schema';

// The wire union as data. `satisfies` proves every member is real; the key-set
// assertion below proves it lists them all.
const ALL_WIRE_TYPES = [
  'doc.replaced',
  'block.upserted',
  'block.removed',
  'reply.created',
  'round.started',
  'revising.changed',
  'present.closed',
  'decision.created',
  'choice.selected',
  'feedback.created',
  'annotation.created',
  'annotation.removed',
  'triage.decided',
  'input.submitted',
  'pack.interaction',
  'submit',
  'channel.changed',
] as const satisfies readonly WireFrame['type'][];

// A missing wire type makes this fail to compile, keeping the list complete.
type Uncovered = Exclude<WireFrame['type'], (typeof ALL_WIRE_TYPES)[number]>;
const _exhaustive: Uncovered[] = [];

describe('TOAST_TEXT', () => {
  it('covers exactly the wire union', () => {
    void _exhaustive;
    expect(new Set(Object.keys(TOAST_TEXT))).toEqual(new Set(ALL_WIRE_TYPES));
  });

  it('toasts agent-authored activity and lifecycle', () => {
    expect(TOAST_TEXT['doc.replaced']).toBe('The board was redrafted');
    expect(TOAST_TEXT['block.upserted']).toBe('A block was updated');
    expect(TOAST_TEXT['block.removed']).toBe('A block was removed');
    expect(TOAST_TEXT['reply.created']).toBe('The agent replied');
    expect(TOAST_TEXT['round.started']).toBe('A new round started');
    expect(TOAST_TEXT['present.closed']).toBe('The session closed');
  });

  it('stays silent on the agent’s revising announcement (per-step markers replace it)', () => {
    expect(TOAST_TEXT['revising.changed']).toBeNull();
  });

  it('stays silent on the browser’s own human echoes and channel presence', () => {
    for (const type of [
      'decision.created',
      'choice.selected',
      'feedback.created',
      'annotation.created',
      'annotation.removed',
      'triage.decided',
      'input.submitted',
      'pack.interaction',
      'submit',
      'channel.changed',
    ] as const) {
      expect(TOAST_TEXT[type]).toBeNull();
    }
  });
});

describe('toastFor focus-mode suppression', () => {
  const upserted: WireFrame = { schemaVersion: 1, type: 'block.upserted', block: { id: 'b1', type: 'markdown', md: 'x' } };

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('keeps the block.upserted toast in board mode (no focus deck mounted)', () => {
    expect(toastFor(upserted, emptyState(), emptyState())).toEqual({ kind: 'info', text: 'A block was updated' });
  });

  it('drops the block.upserted toast in focus mode — the per-step callout replaces it', () => {
    const deck = document.createElement('div');
    deck.className = 'focus-deck';
    document.body.appendChild(deck);
    expect(toastFor(upserted, emptyState(), emptyState())).toBeNull();
  });

  it('still fires other agent toasts in focus mode', () => {
    const deck = document.createElement('div');
    deck.className = 'focus-deck';
    document.body.appendChild(deck);
    const replaced: WireFrame = { schemaVersion: 1, type: 'doc.replaced', doc: { version: 1, title: '', blocks: [] }, revision: 1 };
    expect(toastFor(replaced, emptyState(), emptyState())).toEqual({ kind: 'info', text: 'The board was redrafted' });
  });

  it('never toasts a human echo, focus or board', () => {
    const choice: WireFrame = { schemaVersion: 1, type: 'choice.selected', blockId: 'c1', optionIds: ['o1'] };
    expect(toastFor(choice, emptyState(), emptyState())).toBeNull();
  });
});

describe('toastFor block changes', () => {
  const board = (blocks: Block[]): PresentState => ({ ...emptyState(), doc: { version: 1, title: '', blocks } });
  const plan: Block = { id: 'k1', type: 'card', title: 'Rollout plan', children: [{ id: 'm1', type: 'markdown', md: 'steps' }] };
  const upsert = (block: Block): WireFrame => ({ schemaVersion: 1, type: 'block.upserted', block });
  const remove = (id: string): WireFrame => ({ schemaVersion: 1, type: 'block.removed', id });

  it('titles an update with the block’s label after the change', () => {
    const retitled: Block = { ...plan, title: 'Rollout plan, revised' };
    expect(toastFor(upsert(retitled), board([plan]), board([retitled]))).toEqual({
      kind: 'info',
      title: 'Rollout plan, revised',
      text: 'A block was updated',
    });
  });

  it('titles a removal with the label the block had before it went', () => {
    expect(toastFor(remove('k1'), board([plan]), board([]))).toEqual({
      kind: 'info',
      title: 'Rollout plan',
      text: 'A block was removed',
    });
  });

  it('titles a change to an untitled card child with its card', () => {
    expect(toastFor(remove('m1'), board([plan]), board([]))?.title).toBe('Rollout plan');
  });

  it('keeps the generic copy when the removed block was never on the board', () => {
    expect(toastFor(remove('gone'), board([plan]), board([plan]))).toEqual({ kind: 'info', text: 'A block was removed' });
  });
});

describe('toastFor reply', () => {
  const reply = (blockId: string, md: string): WireFrame => ({ schemaVersion: 1, type: 'reply.created', id: 'r1', blockId, md });
  const blocks: Block[] = [
    { id: 'a1', type: 'approval', prompt: 'Ship the migration?' },
    { id: 'k1', type: 'card', title: 'Rollout plan', children: [{ id: 'm1', type: 'markdown', md: 'steps' }] },
  ];

  function board(notes: string[]): PresentState {
    const state = emptyState();
    state.doc.blocks = blocks;
    state.interactions.feedback = { a1: notes.map((text, i) => ({ id: `f${i}`, text })) };
    return state;
  }

  afterEach(() => setThreadOpener(null));

  it('titles the toast with the block’s label', () => {
    expect(toastFor(reply('a1', 'Done'), board([]), board([]))?.title).toBe('Ship the migration?');
  });

  it('titles a reply on a card’s child with the card that owns its thread', () => {
    expect(toastFor(reply('m1', 'Done'), board([]), board([]))?.title).toBe('Rollout plan');
  });

  it('quotes the latest note above the reply', () => {
    const state = board(['first pass', 'use the blue one']);
    expect(toastFor(reply('a1', 'Done'), state, state)?.text).toBe('You: use the blue one\nAgent: Done');
  });

  it('shows the reply alone when the thread has no notes', () => {
    expect(toastFor(reply('a1', 'Done'), board([]), board([]))?.text).toBe('Agent: Done');
  });

  it('shows the reply as plain text, not markdown', () => {
    const md = '**Done**, see [the diff](https://example.com) and `go vet`';
    expect(toastFor(reply('a1', md), board([]), board([]))?.text).toBe('Agent: Done, see the diff and go vet');
  });

  it('stays up for twelve seconds', () => {
    expect(toastFor(reply('a1', 'Done'), board([]), board([]))?.durationMs).toBe(12_000);
  });

  it('falls back to the generic copy when no thread owns the block', () => {
    expect(toastFor(reply('gone', 'Done'), board([]), board([]))).toEqual({ kind: 'info', text: 'The agent replied' });
  });

  it('has no click-through until a shell registers a thread opener', () => {
    expect(toastFor(reply('a1', 'Done'), board([]), board([]))?.onClick).toBeUndefined();
  });

  it('opens the block’s thread on click once an opener is registered', () => {
    const opened: string[] = [];
    setThreadOpener((entry) => opened.push(entry.blockId));
    toastFor(reply('m1', 'Done'), board([]), board([]))?.onClick?.();
    expect(opened).toEqual(['k1']);
  });
});
