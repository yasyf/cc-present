// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { TOAST_TEXT, toastFor } from './stream';
import type { WireFrame } from './events';

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
    expect(toastFor(upserted)).toEqual({ kind: 'info', text: 'A block was updated' });
  });

  it('drops the block.upserted toast in focus mode — the per-step callout replaces it', () => {
    const deck = document.createElement('div');
    deck.className = 'focus-deck';
    document.body.appendChild(deck);
    expect(toastFor(upserted)).toBeNull();
  });

  it('still fires other agent toasts in focus mode', () => {
    const deck = document.createElement('div');
    deck.className = 'focus-deck';
    document.body.appendChild(deck);
    const replaced: WireFrame = { schemaVersion: 1, type: 'doc.replaced', doc: { version: 1, title: '', blocks: [] }, revision: 1 };
    expect(toastFor(replaced)).toEqual({ kind: 'info', text: 'The board was redrafted' });
  });

  it('never toasts a human echo, focus or board', () => {
    const choice: WireFrame = { schemaVersion: 1, type: 'choice.selected', blockId: 'c1', optionIds: ['o1'] };
    expect(toastFor(choice)).toBeNull();
  });
});
