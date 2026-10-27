// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { threadFeed } from './threadFeed';
import { emptyState } from './reduce';
import type { Approval, Block, Card, ChildBlock, Choice, Input, Markdown, PackBlock } from './schema';
import type { Feedback, PresentState, Reply, RoundEntry, RoundRecord, RoundStub } from './events';

const approval = (id: string, prompt?: string): Approval => ({
  id,
  type: 'approval',
  ...(prompt !== undefined ? { prompt } : {}),
});
const card = (id: string, children: ChildBlock[], title?: string): Card => ({
  id,
  type: 'card',
  ...(title !== undefined ? { title } : {}),
  children,
});
const markdown = (id: string): Markdown => ({ id, type: 'markdown', md: 'prose' });
const input = (id: string, label: string): Input => ({ id, type: 'input', label });
const choice = (id: string, prompt?: string): Choice => ({
  id,
  type: 'choice',
  ...(prompt !== undefined ? { prompt } : {}),
  options: [{ id: 'o0', label: 'A' }],
});
const pack = (id: string, type: string, extra: Record<string, unknown> = {}): PackBlock =>
  ({ id, type, ...extra }) as PackBlock;
const fb = (id: string, text: string): Feedback => ({ id, text });
const reply = (id: string, md: string): Reply => ({ id, md });

function state(over: {
  blocks?: Block[];
  feedback?: Record<string, Feedback[]>;
  replies?: Record<string, Reply[]>;
  history?: RoundEntry[];
  closed?: boolean;
  current?: number;
  blockRounds?: Record<string, number>;
}): PresentState {
  const s = emptyState();
  s.doc.blocks = over.blocks ?? [];
  s.interactions.feedback = over.feedback ?? {};
  s.interactions.replies = over.replies ?? {};
  s.interactions.closed = { value: over.closed ?? false };
  s.rounds.history = over.history ?? [];
  s.rounds.current = over.current ?? s.rounds.current;
  s.rounds.blockRounds = over.blockRounds ?? {};
  return s;
}

function round(number: number, blocks: Block[], feedback: Record<string, Feedback[]>): RoundRecord {
  return {
    number,
    blocks,
    decisions: {},
    choices: {},
    inputs: {},
    packs: {},
    feedback,
    annotations: {},
    triage: {},
    viewed: {},
  };
}

describe('threadFeed pinned', () => {
  it('pins the active live block and drops it from the feed', () => {
    const s = state({
      blocks: [approval('a1', 'Ship one'), approval('a2', 'Ship two')],
      feedback: { a1: [fb('f1', 'go')], a2: [fb('f2', 'wait')] },
    });
    const { pinned, feed } = threadFeed(s, 'a1');
    expect(pinned?.blockId).toBe('a1');
    expect(pinned?.label).toBe('Ship one');
    expect(pinned?.feedback).toHaveLength(1);
    expect(feed.map((e) => e.blockId)).toEqual(['a2']);
  });

  it('returns a pinned entry with no conversation so its composer stays reachable', () => {
    const s = state({ blocks: [approval('a1', 'Ship one')] });
    const { pinned, feed } = threadFeed(s, 'a1');
    expect(pinned?.blockId).toBe('a1');
    expect(pinned?.feedback).toHaveLength(0);
    expect(feed).toHaveLength(0);
  });

  it('yields a null pinned for a missing or null active id', () => {
    const s = state({ blocks: [approval('a1')], feedback: { a1: [fb('f1', 'x')] } });
    expect(threadFeed(s, null).pinned).toBeNull();
    expect(threadFeed(s, 'ghost').pinned).toBeNull();
    // The unpinned conversation still surfaces in the feed.
    expect(threadFeed(s, null).feed.map((e) => e.blockId)).toEqual(['a1']);
  });

  it('locks every live thread once the board is closed', () => {
    const blocks = [approval('a1', 'Ship'), markdown('m1')];
    expect(threadFeed(state({ blocks }), 'a1').pinned?.locked).toBe(false);
    const closed = state({ blocks, closed: true });
    expect(threadFeed(closed, 'a1').pinned?.locked).toBe(true);
    expect(threadFeed(closed, 'm1').pinned?.locked).toBe(true);
  });

  it('locks a live thread whose top-level block sits in a stale round', () => {
    const s = state({
      blocks: [card('k1', [approval('a1', 'Old call')]), approval('a2', 'New call')],
      current: 2,
      blockRounds: { k1: 1, a2: 2 },
    });
    expect(threadFeed(s, 'k1').pinned?.locked).toBe(true);
    expect(threadFeed(s, 'a1').pinned?.locked).toBe(true);
    expect(threadFeed(s, 'a2').pinned?.locked).toBe(false);
  });
});

describe('threadFeed ownership', () => {
  const blocks: Block[] = [
    card('k1', [markdown('m1'), approval('a1', 'Ship'), input('i1', 'Why')], 'Rollout'),
    markdown('m2'),
    input('i2', 'Your name'),
  ];
  const noted = Object.fromEntries(['k1', 'm1', 'a1', 'i1', 'm2', 'i2'].map((id) => [id, [fb(`f-${id}`, id)]]));

  it('gives a card, a top-level content block, and a top-level input each an entry', () => {
    const { feed } = threadFeed(state({ blocks, feedback: noted }), null);
    expect(feed.map((e) => [e.blockId, e.type, e.label])).toEqual([
      ['k1', 'card', 'Rollout'],
      ['m2', 'markdown', 'Markdown'],
      ['i2', 'input', 'Your name'],
    ]);
  });

  it('gives a card child no entry of its own, a decision included', () => {
    const { feed } = threadFeed(state({ blocks, feedback: noted }), null);
    expect(feed.some((e) => ['m1', 'a1', 'i1'].includes(e.blockId))).toBe(false);
  });

  it('pins the card for every child, and a top-level block itself', () => {
    const s = state({ blocks });
    expect(threadFeed(s, 'm1').pinned?.blockId).toBe('k1');
    expect(threadFeed(s, 'i1').pinned?.blockId).toBe('k1');
    expect(threadFeed(s, 'a1').pinned?.blockId).toBe('k1');
    expect(threadFeed(s, 'i2').pinned?.blockId).toBe('i2');
  });

  it('folds the notes and replies on every card child into the card, in document order', () => {
    const s = state({
      blocks,
      feedback: noted,
      replies: { i1: [reply('r3', 'on the input')], k1: [reply('r1', 'on the card')], m1: [reply('r2', 'on the prose')] },
    });
    const { pinned } = threadFeed(s, 'k1');
    expect(pinned?.feedback).toEqual([fb('f-k1', 'k1'), fb('f-m1', 'm1'), fb('f-a1', 'a1'), fb('f-i1', 'i1')]);
    expect(pinned?.replies).toEqual([reply('r1', 'on the card'), reply('r2', 'on the prose'), reply('r3', 'on the input')]);
    expect(pinned?.lastComment).toBe('on the input');
  });

  it('lists a card whose only conversation sits on a child', () => {
    const s = state({ blocks, replies: { m1: [reply('r1', 'about the prose')] } });
    expect(threadFeed(s, null).feed.map((e) => [e.blockId, e.lastComment])).toEqual([['k1', 'about the prose']]);
  });

  it('folds a frozen card child the same way, reading notes from the round and replies live', () => {
    const s = state({
      replies: { hm1: [reply('r1', 'a late reply')] },
      history: [round(1, [card('h1', [markdown('hm1')], 'Past')], { h1: [fb('f1', 'on the card')], hm1: [fb('f2', 'on the child')] })],
    });
    expect(threadFeed(s, null).feed).toMatchObject([
      {
        blockId: 'h1',
        feedback: [fb('f1', 'on the card'), fb('f2', 'on the child')],
        replies: [reply('r1', 'a late reply')],
        locked: true,
        live: false,
      },
    ]);
  });

  it('pins the frozen card for a history child that owns no thread', () => {
    const s = state({ history: [round(1, [card('h1', [markdown('hm1')])], { h1: [fb('f1', 'past')] })] });
    expect(threadFeed(s, 'hm1').pinned).toMatchObject({ blockId: 'h1', locked: true, live: false });
  });
});

describe('threadFeed order', () => {
  it('lists live blocks in document order, then history rounds', () => {
    const s = state({
      blocks: [approval('a1'), choice('c1')],
      feedback: { a1: [fb('f1', 'x')], c1: [fb('f2', 'y')] },
      history: [round(1, [approval('h1')], { h1: [fb('f3', 'z')] })],
    });
    const { feed } = threadFeed(s, null);
    expect(feed.map((e) => e.blockId)).toEqual(['a1', 'c1', 'h1']);
    expect(feed[2]!.locked).toBe(true);
  });

  it('marks doc blocks live and history entries not', () => {
    const s = state({
      blocks: [approval('a1')],
      feedback: { a1: [fb('f1', 'x')] },
      history: [round(1, [approval('h1')], { h1: [fb('f2', 'y')] })],
    });
    const { feed } = threadFeed(s, null);
    expect(feed.map((e) => [e.blockId, e.live])).toEqual([
      ['a1', true],
      ['h1', false],
    ]);
  });

  it('marks a doc block whose round has closed not live, so a jump reveals its round', () => {
    const s = state({
      blocks: [card('k1', [approval('a1', 'Old call')]), approval('a2', 'New call')],
      feedback: { k1: [fb('f1', 'x')], a1: [fb('f2', 'y')], a2: [fb('f3', 'z')] },
      current: 2,
      blockRounds: { k1: 1, a2: 2 },
      history: [round(1, [card('k1', [approval('a1', 'Old call')])], { k1: [fb('f1', 'x')], a1: [fb('f2', 'y')] })],
    });
    expect(threadFeed(s, null).feed.map((e) => [e.blockId, e.live])).toEqual([
      ['k1', false],
      ['a2', true],
    ]);
  });

  it('only includes blocks that carry a note or reply', () => {
    const s = state({
      blocks: [approval('a1'), approval('a2'), approval('a3')],
      feedback: { a1: [fb('f1', 'x')] },
      replies: { a3: [reply('r1', 'ok')] },
    });
    // a2 has nothing, so it never enters the feed; a3's lone reply still counts.
    expect(threadFeed(s, null).feed.map((e) => e.blockId)).toEqual(['a1', 'a3']);
  });

  it('excerpts the latest reply as the last comment once the agent has answered', () => {
    const s = state({
      blocks: [approval('a1', 'Ship')],
      feedback: { a1: [fb('f1', 'first'), fb('f2', 'second')] },
      replies: { a1: [reply('r1', 'ok'), reply('r2', 'done')] },
    });
    expect(threadFeed(s, null).feed[0]!.lastComment).toBe('done');
  });

  it('excerpts a reply as plain text, without its markdown', () => {
    const s = state({
      blocks: [approval('a1', 'Ship')],
      replies: { a1: [reply('r1', 'Second reply with *emphasis* and a list:\n\n- one\n- two')] },
    });
    expect(threadFeed(s, null).feed[0]!.lastComment).toBe('Second reply with emphasis and a list: one two');
  });

  it('falls back to the latest note when no reply exists', () => {
    const s = state({ blocks: [approval('a1', 'Ship')], feedback: { a1: [fb('f1', 'one'), fb('f2', 'two')] } });
    expect(threadFeed(s, null).feed[0]!.lastComment).toBe('two');
  });

  it('leaves lastComment null for a pinned entry with no conversation', () => {
    const s = state({ blocks: [approval('a1', 'Ship')] });
    expect(threadFeed(s, 'a1').pinned!.lastComment).toBeNull();
  });

  it('threads live replies onto a frozen history entry and dedups a carried id', () => {
    const s = state({
      blocks: [approval('a1', 'Live')],
      feedback: { a1: [fb('f1', 'live note')] },
      replies: { a1: [reply('r1', 'agent reply')], h1: [reply('r2', 'later reply')] },
      history: [
        round(1, [approval('a1', 'Frozen'), approval('h1')], { a1: [fb('old', 'stale')], h1: [fb('f2', 'past')] }),
      ],
    });
    const { feed } = threadFeed(s, null);
    // a1 lives in the doc, so its live entry wins and the frozen copy is skipped.
    const a1 = feed.find((e) => e.blockId === 'a1')!;
    expect(a1.locked).toBe(false);
    expect(a1.label).toBe('Live');
    expect(a1.feedback).toEqual([fb('f1', 'live note')]);
    // h1 is history-only: frozen feedback, live replies.
    const h1 = feed.find((e) => e.blockId === 'h1')!;
    expect(h1.locked).toBe(true);
    expect(h1.feedback).toEqual([fb('f2', 'past')]);
    expect(h1.replies).toEqual([reply('r2', 'later reply')]);
  });
});

describe('threadFeed snapshot stubs', () => {
  const stub: RoundStub = {
    number: 1,
    title: 'Past',
    outline: [{ id: 'h1', type: 'card', label: 'Rollout', children: ['ha1'] }],
    feedback: { h1: [fb('f1', 'on the card')], ha1: [fb('f2', 'on the decision')] },
  };

  it('threads a stubbed round like a frozen one, folding its card children', () => {
    const s = state({ replies: { ha1: [reply('r1', 'a late reply')] }, history: [stub] });
    expect(threadFeed(s, null).feed).toMatchObject([
      {
        blockId: 'h1',
        type: 'card',
        label: 'Rollout',
        feedback: [fb('f1', 'on the card'), fb('f2', 'on the decision')],
        replies: [reply('r1', 'a late reply')],
        locked: true,
        live: false,
      },
    ]);
  });

  it('pins the stubbed card for one of its children', () => {
    expect(threadFeed(state({ history: [stub] }), 'ha1').pinned?.blockId).toBe('h1');
  });
});

describe('threadFeed pack blocks', () => {
  it('threads an interactive pack block under its question', () => {
    const s = state({
      blocks: [pack('k1', 'dd.fork', { question: 'Where does it run?' })],
      feedback: { k1: [fb('f1', 'neither')] },
    });
    const { pinned, feed } = threadFeed(s, 'k1');
    expect(pinned).toMatchObject({ blockId: 'k1', type: 'dd.fork', label: 'Where does it run?', locked: false });
    expect(feed).toEqual([]);
  });

  it('folds a nested pack block into its card, interactive or not', () => {
    const s = state({
      blocks: [card('c1', [pack('k1', 'dd.fork'), pack('k2', 'dd.callout')])],
      feedback: { k1: [fb('f1', 'neither')], k2: [fb('f2', 'stray')] },
    });
    expect(threadFeed(s, null).feed.map((e) => e.blockId)).toEqual(['c1']);
    expect(threadFeed(s, 'k1').pinned).toMatchObject({
      blockId: 'c1',
      feedback: [fb('f1', 'neither'), fb('f2', 'stray')],
    });
  });

  it('labels a card choice with no prompt by its card title', () => {
    const s = state({ blocks: [card('k1', [choice('c1')], 'How deep may forks go?')], feedback: { c1: [fb('f1', 'why')] } });
    expect(threadFeed(s, 'c1').pinned?.label).toBe('How deep may forks go?');
  });

  it('labels a pack block with no heading field by its dotted type', () => {
    const s = state({ blocks: [pack('k1', 'dd.fork')] });
    expect(threadFeed(s, 'k1').pinned?.label).toBe('dd.fork');
  });

  it('threads a top-level display-only pack block as plain content', () => {
    const s = state({ blocks: [pack('k1', 'dd.callout')], feedback: { k1: [fb('f1', 'noted')] } });
    expect(threadFeed(s, 'k1').pinned).toMatchObject({ blockId: 'k1', type: 'dd.callout' });
  });
});
