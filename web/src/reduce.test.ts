// Cross-language fixture parity: the TypeScript reducer must produce the same
// State the Go reducer does over the very same internal/state/testdata/*.json
// fixtures. An expected state may omit an interaction map or the submit/close
// signals that stay at their zero value; normalizeInteractions fills them, the
// same way the Go test's initMaps does, and a JSON round-trip drops the
// undefined optionals (note, summary) the way Go's omitempty tags do.

import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { applyEvent, applyInteraction, reduce } from './reduce';
import type { Doc } from './schema';
import type { Interactions, PresentEvent, PresentState, Revising, Rounds } from './events';

const testdataDir = fileURLToPath(new URL('../../internal/state/testdata/', import.meta.url));

interface Fixture {
  name: string;
  events: PresentEvent[];
  expected: { doc: Doc; interactions?: Partial<Interactions>; rounds?: Partial<Rounds>; revising?: Revising };
}

function normalizeInteractions(i: Partial<Interactions> | undefined): Interactions {
  return {
    decisions: i?.decisions ?? {},
    choices: i?.choices ?? {},
    inputs: i?.inputs ?? {},
    packs: i?.packs ?? {},
    feedback: i?.feedback ?? {},
    replies: i?.replies ?? {},
    annotations: i?.annotations ?? {},
    triage: i?.triage ?? {},
    submitted: i?.submitted ?? { value: false, revision: 0 },
    closed: i?.closed ?? { value: false },
  };
}

// normalizeRounds fills a round record's `packs`, `annotations`, and `triage`
// maps the same way Go's initMaps does (reduce_test.go), so a fixture that omits
// an empty snapshot still matches the reducer, which always emits them.
function normalizeRounds(r: Partial<Rounds> | undefined): Rounds {
  return {
    current: r?.current ?? 1,
    currentTitle: r?.currentTitle,
    blockRounds: r?.blockRounds ?? {},
    history: (r?.history ?? []).map((rec) => ({
      ...rec,
      packs: rec.packs ?? {},
      annotations: rec.annotations ?? {},
      triage: rec.triage ?? {},
    })),
  };
}

// normalizeRevising fills the empty working set a fixture omits, the same way the
// Go test's initMaps normalizes State.Revising.
function normalizeRevising(r: Revising | undefined): Revising {
  return { blockIds: r?.blockIds ?? [], note: r?.note };
}

function canonical(state: PresentState): unknown {
  return JSON.parse(JSON.stringify(state)) as unknown;
}

function loadFixtures(): Fixture[] {
  return readdirSync(testdataDir)
    .filter((f) => f.endsWith('.json'))
    .sort()
    .map((f) => JSON.parse(readFileSync(testdataDir + f, 'utf8')) as Fixture);
}

describe('reduce fixture parity', () => {
  const fixtures = loadFixtures();

  it('finds the shared fixtures', () => {
    expect(fixtures.length).toBeGreaterThan(0);
  });

  for (const fx of fixtures) {
    it(fx.name, () => {
      const got = reduce(fx.events);
      const want: PresentState = {
        doc: fx.expected.doc,
        interactions: normalizeInteractions(fx.expected.interactions),
        rounds: normalizeRounds(fx.expected.rounds),
        revising: normalizeRevising(fx.expected.revising),
      };
      expect(canonical(got)).toEqual(canonical(want));
    });
  }
});

describe('applyInteraction optimistic path', () => {
  const base = reduce([
    { origin: 'agent', type: 'block.upserted', seq: 1, payload: { block: { id: 'a1', type: 'approval' } } },
  ]);

  it('applies a decision optimistically before the echo', () => {
    const next = applyInteraction(base, { type: 'decision.created', blockId: 'a1', verdict: 'approved' });
    expect(next.interactions.decisions.a1).toEqual({ verdict: 'approved' });
  });

  it('carries an other write-in through the optimistic choice echo', () => {
    const next = applyInteraction(base, { type: 'choice.selected', blockId: 'ch1', optionIds: [], other: 'custom' });
    expect(next.interactions.choices.ch1).toEqual({ optionIds: [], other: 'custom' });
  });

  it('defers submit to the echo, leaving the cache unchanged', () => {
    // A failed POST must not strand the UI past the round; the optimistic patch
    // is a no-op and the SSE echo advances the round instead.
    expect(applyInteraction(base, { type: 'submit', revision: 3 })).toBe(base);
  });

  it('advances the round when the submit event itself replays', () => {
    const echoed = applyEvent(base, { origin: 'human', type: 'submit', seq: 2, payload: { revision: 3 } });
    expect(echoed.rounds.current).toBe(2);
    expect(echoed.rounds.history.map((r) => r.submittedRevision)).toEqual([3]);
  });
});

describe('reduce errors', () => {
  it('rejects an unknown event type', () => {
    const events = [
      { origin: 'agent', type: 'bogus.event', seq: 1, payload: {} },
    ] as unknown as PresentEvent[];
    expect(() => reduce(events)).toThrow(/unknown event type/);
  });

  it('rejects an invalid verdict', () => {
    const events = [
      { origin: 'human', type: 'decision.created', seq: 1, payload: { blockId: 'a1', verdict: 'maybe' } },
    ] as unknown as PresentEvent[];
    expect(() => reduce(events)).toThrow(/invalid verdict/);
  });
});
