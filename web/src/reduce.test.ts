// Cross-language fixture parity: the TypeScript reducer must produce the same
// State the Go reducer does over the very same internal/state/testdata/*.json
// fixtures. An expected state may omit an interaction map or the submit/close
// signals that stay at their zero value; normalizeInteractions fills them, the
// same way the Go test's initMaps does, and a JSON round-trip drops the
// undefined optionals (note, summary) the way Go's omitempty tags do.

import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { reduce } from './reduce';
import type { Doc } from './schema';
import type { Interactions, PresentEvent, PresentState } from './events';

const testdataDir = fileURLToPath(new URL('../../internal/state/testdata/', import.meta.url));

interface Fixture {
  name: string;
  events: PresentEvent[];
  expected: { doc: Doc; interactions?: Partial<Interactions> };
}

function normalizeInteractions(i: Partial<Interactions> | undefined): Interactions {
  return {
    decisions: i?.decisions ?? {},
    choices: i?.choices ?? {},
    inputs: i?.inputs ?? {},
    feedback: i?.feedback ?? {},
    replies: i?.replies ?? {},
    submitted: i?.submitted ?? { value: false, revision: 0 },
    closed: i?.closed ?? { value: false },
  };
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
      };
      expect(canonical(got)).toEqual(canonical(want));
    });
  }
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
