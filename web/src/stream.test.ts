import { describe, expect, it } from 'vitest';
import { TOAST_TEXT } from './stream';
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
      'input.submitted',
      'pack.interaction',
      'submit',
      'channel.changed',
    ] as const) {
      expect(TOAST_TEXT[type]).toBeNull();
    }
  });
});
