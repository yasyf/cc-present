import { describe, expect, it } from 'vitest';
import { focusSteps, stepStatus, stepTitle, swipeVerdict } from './focus';
import type { FocusStep } from './focus';
import type { Block } from './schema';
import type { Interactions } from './events';

const approval = (id: string): Block => ({ id, type: 'approval' });
const choice = (id: string): Block => ({
  id,
  type: 'choice',
  options: [{ id: `${id}o1`, label: 'one' }],
});
const input = (id: string): Block => ({ id, type: 'input', label: id });
const markdown = (id: string): Block => ({ id, type: 'markdown', md: id });
const section = (id: string, title: string): Block => ({ id, type: 'section', title });
const card = (id: string, children: Block[]): Block => ({ id, type: 'card', children: children as never });
const pack = (id: string, type = 'ex.rating'): Block => ({ id, type } as Block);

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

interface Projected {
  id: string;
  kind: FocusStep['kind'];
  tier: string | undefined;
  context: string[];
  decidables: string[];
  primary: string | undefined;
  swipeable: boolean;
}

const project = (s: FocusStep): Projected => ({
  id: s.id,
  kind: s.kind,
  tier: s.tier,
  context: s.context.map((b) => b.id),
  decidables: s.decidables,
  primary: s.primary?.id,
  swipeable: s.swipeable,
});

describe('focusSteps', () => {
  const cases: { name: string; blocks: Block[]; pack: Set<string>; expected: Projected[] }[] = [
    { name: 'empty doc yields no steps', blocks: [], pack: new Set(), expected: [] },
    {
      name: 'a lone top-level approval is one swipeable decision step',
      blocks: [approval('a1')],
      pack: new Set(),
      expected: [
        { id: 'a1', kind: 'decision', tier: undefined, context: [], decidables: ['a1'], primary: 'a1', swipeable: true },
      ],
    },
    {
      name: 'a content run before a decision attaches as its context',
      blocks: [markdown('m1'), markdown('m2'), approval('a1')],
      pack: new Set(),
      expected: [
        {
          id: 'a1',
          kind: 'decision',
          tier: undefined,
          context: ['m1', 'm2'],
          decidables: ['a1'],
          primary: 'a1',
          swipeable: true,
        },
      ],
    },
    {
      name: 'a trailing content run is one read-only context step',
      blocks: [approval('a1'), markdown('m1'), markdown('m2')],
      pack: new Set(),
      expected: [
        { id: 'a1', kind: 'decision', tier: undefined, context: [], decidables: ['a1'], primary: 'a1', swipeable: true },
        { id: 'm2', kind: 'context', tier: undefined, context: ['m1'], decidables: [], primary: undefined, swipeable: false },
      ],
    },
    {
      name: 'sections set the tier and never become steps',
      blocks: [section('s1', 'First'), card('c1', [approval('c1a')]), section('s2', 'Second'), approval('a1')],
      pack: new Set(),
      expected: [
        { id: 'c1', kind: 'decision', tier: 'First', context: [], decidables: ['c1a'], primary: 'c1a', swipeable: true },
        { id: 'a1', kind: 'decision', tier: 'Second', context: [], decidables: ['a1'], primary: 'a1', swipeable: true },
      ],
    },
    {
      name: 'a section flushes a pending run as a standalone context step',
      blocks: [markdown('m1'), section('s1', 'Later'), approval('a1')],
      pack: new Set(),
      expected: [
        { id: 'm1', kind: 'context', tier: undefined, context: [], decidables: [], primary: undefined, swipeable: false },
        { id: 'a1', kind: 'decision', tier: 'Later', context: [], decidables: ['a1'], primary: 'a1', swipeable: true },
      ],
    },
    {
      name: 'a card with multiple decidables is one step, not swipeable',
      blocks: [card('c1', [approval('c1a'), choice('c1c'), markdown('c1m')])],
      pack: new Set(),
      expected: [
        {
          id: 'c1',
          kind: 'decision',
          tier: undefined,
          context: [],
          decidables: ['c1a', 'c1c'],
          primary: 'c1a',
          swipeable: false,
        },
      ],
    },
    {
      name: 'a card with no decidables is a context step',
      blocks: [card('c1', [markdown('c1m')])],
      pack: new Set(),
      expected: [
        { id: 'c1', kind: 'context', tier: undefined, context: [], decidables: [], primary: undefined, swipeable: false },
      ],
    },
    {
      name: 'a content-only board is all context steps',
      blocks: [markdown('m1'), markdown('m2')],
      pack: new Set(),
      expected: [
        { id: 'm2', kind: 'context', tier: undefined, context: ['m1'], decidables: [], primary: undefined, swipeable: false },
      ],
    },
    {
      name: 'a top-level input is its own decision step',
      blocks: [input('i1')],
      pack: new Set(),
      expected: [
        { id: 'i1', kind: 'decision', tier: undefined, context: [], decidables: ['i1'], primary: 'i1', swipeable: false },
      ],
    },
    {
      name: 'an interactive pack decides while a static pack accumulates',
      blocks: [pack('r1', 'ex.rating'), pack('c1', 'ex.callout'), approval('a1')],
      pack: new Set(['ex.rating']),
      expected: [
        { id: 'r1', kind: 'decision', tier: undefined, context: [], decidables: ['r1'], primary: 'r1', swipeable: false },
        {
          id: 'a1',
          kind: 'decision',
          tier: undefined,
          context: ['c1'],
          decidables: ['a1'],
          primary: 'a1',
          swipeable: true,
        },
      ],
    },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(focusSteps(c.blocks, c.pack).map(project)).toEqual(c.expected);
    });
  }
});

describe('stepTitle', () => {
  const only = (blocks: Block[]) => focusSteps(blocks, new Set<string>())[0]!;
  const cases: { name: string; step: FocusStep; expected: string }[] = [
    { name: 'untitled card falls back to Card', step: only([card('c1', [approval('c1a')])]), expected: 'Card' },
    { name: 'approval prompt fallback', step: only([approval('a1')]), expected: 'Approval' },
    { name: 'input label', step: only([input('i1')]), expected: 'i1' },
    { name: 'content run label', step: only([markdown('m1')]), expected: 'Details' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(stepTitle(c.step)).toBe(c.expected);
    });
  }

  it('uses a card title when present', () => {
    const step = focusSteps([{ id: 'c1', type: 'card', title: 'Ship it', children: [approval('c1a')] as never }], new Set())[0]!;
    expect(stepTitle(step)).toBe('Ship it');
  });
});

describe('stepStatus', () => {
  const [approvalStep] = focusSteps([approval('a1')], new Set());
  const [choiceStep] = focusSteps([choice('ch1')], new Set());
  const [inputStep] = focusSteps([input('i1')], new Set());
  const [contextRun] = focusSteps([markdown('m1')], new Set());

  const withDecisions = (over: Partial<Interactions>): Interactions => ({ ...emptyInteractions(), ...over });

  const cases: { name: string; step: FocusStep; interactions: Interactions; expected: ReturnType<typeof stepStatus> }[] = [
    { name: 'undecided approval', step: approvalStep!, interactions: emptyInteractions(), expected: 'undecided' },
    {
      name: 'approved approval fills approve',
      step: approvalStep!,
      interactions: withDecisions({ decisions: { a1: { verdict: 'approved' } } }),
      expected: 'approved',
    },
    {
      name: 'rejected approval fills reject',
      step: approvalStep!,
      interactions: withDecisions({ decisions: { a1: { verdict: 'rejected' } } }),
      expected: 'rejected',
    },
    {
      name: 'decided choice reads decided',
      step: choiceStep!,
      interactions: withDecisions({ choices: { ch1: { optionIds: ['ch1o1'] } } }),
      expected: 'decided',
    },
    { name: 'input steps never fill', step: inputStep!, interactions: emptyInteractions(), expected: null },
    { name: 'context runs never fill', step: contextRun!, interactions: emptyInteractions(), expected: null },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(stepStatus(c.step, c.interactions, new Set())).toBe(c.expected);
    });
  }
});

describe('swipeVerdict', () => {
  const cases: { name: string; offset: number; velocity: number; expected: ReturnType<typeof swipeVerdict> }[] = [
    { name: 'offset at the right threshold approves', offset: 120, velocity: 0, expected: 'approved' },
    { name: 'offset at the left threshold rejects', offset: -120, velocity: 0, expected: 'rejected' },
    { name: 'offset just past the right threshold approves', offset: 200, velocity: 0, expected: 'approved' },
    { name: 'offset one below threshold snaps back', offset: 119, velocity: 0, expected: null },
    { name: 'offset one below threshold to the left snaps back', offset: -119, velocity: 0, expected: null },
    { name: 'a rightward flick alone commits', offset: 0, velocity: 600, expected: 'approved' },
    { name: 'a leftward flick alone commits', offset: 0, velocity: -600, expected: 'rejected' },
    { name: 'a flick one below threshold snaps back', offset: 40, velocity: 599, expected: null },
    { name: 'sub-threshold in both dimensions snaps back', offset: 40, velocity: 100, expected: null },
    { name: 'distance wins direction over a contrary flick', offset: 200, velocity: -900, expected: 'approved' },
    { name: 'a short leftward flick past velocity rejects', offset: -30, velocity: -800, expected: 'rejected' },
    { name: 'no movement snaps back', offset: 0, velocity: 0, expected: null },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(swipeVerdict(c.offset, c.velocity)).toBe(c.expected);
    });
  }
});
