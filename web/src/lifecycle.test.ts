// boardPhase over states folded from real events through the shared reducer. The
// reducer and its fixtures are load-bearing; this only asserts the phase the UI
// derives from the state it produces.

import { describe, expect, it } from 'vitest';
import { reduce } from './reduce';
import { boardPhase } from './lifecycle';
import type { BoardPhase } from './lifecycle';
import type { Approval } from './schema';
import type { PresentEvent } from './events';

const a1: Approval = { id: 'a1', type: 'approval' };
const a2: Approval = { id: 'a2', type: 'approval' };

const upserted = (block: Approval, seq: number): PresentEvent => ({
  origin: 'agent',
  type: 'block.upserted',
  seq,
  payload: { block },
});
const submit = (revision: number, seq: number): PresentEvent => ({
  origin: 'human',
  type: 'submit',
  seq,
  payload: { revision },
});
const started = (seq: number, title: string): PresentEvent => ({
  origin: 'agent',
  type: 'round.started',
  seq,
  payload: { title },
});
const closed = (seq: number, summary?: string): PresentEvent => ({
  origin: 'system',
  type: 'present.closed',
  seq,
  payload: summary ? { summary } : {},
});

interface Case {
  name: string;
  events: PresentEvent[];
  expected: BoardPhase;
}

const cases: Case[] = [
  {
    name: 'fresh board waits with no last round',
    events: [],
    expected: { kind: 'waiting', lastRound: undefined },
  },
  {
    name: 'round-1 blocks are live',
    events: [upserted(a1, 1)],
    expected: { kind: 'live', blocks: [a1] },
  },
  {
    name: 'a dirty submit waits, carrying the submitted round',
    events: [upserted(a1, 1), submit(3, 2)],
    expected: {
      kind: 'waiting',
      lastRound: {
        number: 1,
        blocks: [a1],
        decisions: {},
        choices: {},
        inputs: {},
        packs: {},
        feedback: {},
        annotations: {},
        triage: {},
        submittedRevision: 3,
      },
    },
  },
  {
    name: 'a fresh round with content is live again',
    events: [upserted(a1, 1), submit(1, 2), started(3, 'Round 2'), upserted(a2, 4)],
    expected: { kind: 'live', blocks: [a2] },
  },
  {
    name: 'a carry-advance keeps only the carried block live',
    events: [
      upserted(a1, 1),
      upserted(a2, 2),
      { origin: 'human', type: 'decision.created', seq: 3, payload: { blockId: 'a1', verdict: 'approved' } },
      { origin: 'agent', type: 'round.started', seq: 4, payload: { carry: ['a2'] } },
    ],
    expected: { kind: 'live', blocks: [a2] },
  },
  {
    name: 'closing mid-round is closed, keeping the live blocks read-only',
    events: [upserted(a1, 1), closed(2, 'All set')],
    expected: { kind: 'closed', blocks: [a1], summary: 'All set' },
  },
  {
    name: 'closing after a submit is closed and empty',
    events: [upserted(a1, 1), submit(1, 2), closed(3)],
    expected: { kind: 'closed', blocks: [], summary: undefined },
  },
];

describe('boardPhase', () => {
  for (const c of cases) {
    it(c.name, () => {
      expect(boardPhase(reduce(c.events))).toEqual(c.expected);
    });
  }
});
