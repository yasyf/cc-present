import { describe, expect, it } from 'vitest';
import {
  choiceToggle,
  decidableIds,
  flatten,
  isDecided,
  nextUndecided,
  step,
  submitItems,
  verdictToggle,
} from './decide';
import type { Block } from './schema';
import type { Interactions } from './events';

const approval = (id: string): Block => ({ id, type: 'approval' });
const choice = (id: string): Block => ({
  id,
  type: 'choice',
  options: [
    { id: `${id}o1`, label: 'one' },
    { id: `${id}o2`, label: 'two' },
  ],
});
const input = (id: string): Block => ({ id, type: 'input', label: id });
const markdown = (id: string): Block => ({ id, type: 'markdown', md: id });
const card = (id: string, children: Block[]): Block => ({
  id,
  type: 'card',
  children: children as never,
});

const emptyInteractions = (): Interactions => ({
  decisions: {},
  choices: {},
  inputs: {},
  packs: {},
  feedback: {},
  replies: {},
  submitted: { value: false, revision: 0 },
  closed: { value: false },
});

describe('flatten', () => {
  it('inlines card children one level below their parent', () => {
    const blocks = [approval('a1'), card('c1', [choice('ch1'), markdown('m1')])];
    expect(flatten(blocks).map((b) => b.id)).toEqual(['a1', 'c1', 'ch1', 'm1']);
  });
});

const pack = (id: string, type = 'ex.rating'): Block => ({ id, type } as Block);

describe('decidableIds', () => {
  it('rings approvals, choices, and inputs in document order, skipping content', () => {
    const blocks = [
      markdown('m1'),
      approval('a1'),
      card('c1', [choice('ch1'), input('in1'), markdown('m2')]),
      input('in2'),
    ];
    expect(decidableIds(blocks, new Set())).toEqual(['a1', 'ch1', 'in1', 'in2']);
  });

  it('rings only the pack types flagged interactive by the registry', () => {
    const blocks = [approval('a1'), pack('r1', 'ex.rating'), pack('c1', 'ex.callout')];
    expect(decidableIds(blocks, new Set(['ex.rating']))).toEqual(['a1', 'r1']);
  });
});

describe('isDecided', () => {
  const base = emptyInteractions();
  const decided: Interactions = {
    ...base,
    decisions: { a1: { verdict: 'approved' } },
    choices: { ch1: { optionIds: ['ch1o1'] }, ch2: { optionIds: [] } },
    packs: { r1: { payload: { value: 4 } } },
  };
  const cases: { name: string; block: Block; interactions: Interactions; expected: boolean }[] = [
    { name: 'approval with a verdict is decided', block: approval('a1'), interactions: decided, expected: true },
    { name: 'approval with no verdict is undecided', block: approval('a2'), interactions: decided, expected: false },
    { name: 'choice with a selection is decided', block: choice('ch1'), interactions: decided, expected: true },
    { name: 'choice with an empty selection is undecided', block: choice('ch2'), interactions: decided, expected: false },
    { name: 'input is never decided', block: input('in1'), interactions: decided, expected: false },
    { name: 'pack with a stored interaction is decided', block: pack('r1'), interactions: decided, expected: true },
    { name: 'pack with no interaction is undecided', block: pack('r2'), interactions: decided, expected: false },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(isDecided(c.block, c.interactions)).toBe(c.expected);
    });
  }
});

describe('submitItems', () => {
  it('tallies approvals and choices only, carrying kind and decided state', () => {
    const blocks = [approval('a1'), input('in1'), card('c1', [choice('ch1'), approval('a2')])];
    const interactions: Interactions = {
      ...emptyInteractions(),
      decisions: { a1: { verdict: 'rejected' } },
      choices: { ch1: { optionIds: ['ch1o1'] } },
    };
    expect(submitItems(blocks, interactions, new Set())).toEqual([
      { id: 'a1', kind: 'approval', decided: true },
      { id: 'ch1', kind: 'choice', decided: true },
      { id: 'a2', kind: 'approval', decided: false },
    ]);
  });

  it('tallies interactive pack blocks, decided when a pack interaction is stored', () => {
    const blocks = [approval('a1'), pack('r1', 'ex.rating'), pack('c1', 'ex.callout')];
    const interactions: Interactions = { ...emptyInteractions(), packs: { r1: { payload: { value: 4 } } } };
    expect(submitItems(blocks, interactions, new Set(['ex.rating']))).toEqual([
      { id: 'a1', kind: 'approval', decided: false },
      { id: 'r1', kind: 'pack', decided: true },
    ]);
  });
});

describe('step', () => {
  const ring = ['a', 'b', 'c'];
  const cases: { name: string; ring: string[]; from: string | null; delta: 1 | -1; expected: string | null }[] = [
    { name: 'null forward enters at the head', ring, from: null, delta: 1, expected: 'a' },
    { name: 'null back enters at the tail', ring, from: null, delta: -1, expected: 'c' },
    { name: 'forward advances', ring, from: 'a', delta: 1, expected: 'b' },
    { name: 'back retreats', ring, from: 'b', delta: -1, expected: 'a' },
    { name: 'forward clamps at the tail', ring, from: 'c', delta: 1, expected: 'c' },
    { name: 'back clamps at the head', ring, from: 'a', delta: -1, expected: 'a' },
    { name: 'an off-ring cursor re-enters at the head', ring, from: 'gone', delta: 1, expected: 'a' },
    { name: 'an empty ring has no members', ring: [], from: null, delta: 1, expected: null },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(step(c.ring, c.from, c.delta)).toBe(c.expected);
    });
  }
});

describe('nextUndecided', () => {
  const ring = ['a', 'b', 'c', 'd'];
  const undecided = new Set(['b', 'd']);
  const cases: { name: string; from: string | null; undecided: Set<string>; expected: string | null }[] = [
    { name: 'from null starts at the head', from: null, undecided, expected: 'b' },
    { name: 'advances past the cursor', from: 'b', undecided, expected: 'd' },
    { name: 'wraps to the head', from: 'd', undecided, expected: 'b' },
    { name: 'skips decided members', from: 'a', undecided, expected: 'b' },
    { name: 'an off-ring cursor starts at the head', from: 'gone', undecided, expected: 'b' },
    { name: 'nothing undecided returns null', from: 'a', undecided: new Set<string>(), expected: null },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(nextUndecided(ring, c.undecided, c.from)).toBe(c.expected);
    });
  }
});

describe('verdictToggle', () => {
  const cases: { name: string; current: 'approved' | 'rejected' | 'cleared' | undefined; target: 'approved' | 'rejected'; expected: string }[] = [
    { name: 'fresh approve sets approved', current: undefined, target: 'approved', expected: 'approved' },
    { name: 're-press approve clears', current: 'approved', target: 'approved', expected: 'cleared' },
    { name: 'approve over reject switches', current: 'rejected', target: 'approved', expected: 'approved' },
    { name: 're-press reject clears', current: 'rejected', target: 'rejected', expected: 'cleared' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(verdictToggle(c.current, c.target)).toBe(c.expected);
    });
  }
});

describe('choiceToggle', () => {
  const cases: { name: string; selected: string[]; option: string; multi: boolean; expected: string[] }[] = [
    { name: 'single select replaces', selected: [], option: 'o1', multi: false, expected: ['o1'] },
    { name: 'single re-select clears', selected: ['o1'], option: 'o1', multi: false, expected: [] },
    { name: 'single switch replaces', selected: ['o1'], option: 'o2', multi: false, expected: ['o2'] },
    { name: 'multi adds', selected: ['o1'], option: 'o2', multi: true, expected: ['o1', 'o2'] },
    { name: 'multi removes', selected: ['o1', 'o2'], option: 'o1', multi: true, expected: ['o2'] },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(choiceToggle(c.selected, c.option, c.multi)).toEqual(c.expected);
    });
  }
});
