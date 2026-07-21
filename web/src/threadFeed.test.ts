import { describe, expect, it } from 'vitest';
import { threadFeed } from './threadFeed';
import { emptyState } from './reduce';
import type { Approval, Block, Choice } from './schema';
import type { Feedback, PresentState, Reply, RoundRecord } from './events';

const approval = (id: string, prompt?: string, allowFeedback?: boolean): Approval => ({
  id,
  type: 'approval',
  ...(prompt !== undefined ? { prompt } : {}),
  ...(allowFeedback !== undefined ? { allowFeedback } : {}),
});
const choice = (id: string, prompt?: string): Choice => ({
  id,
  type: 'choice',
  ...(prompt !== undefined ? { prompt } : {}),
  options: [{ id: 'o0', label: 'A' }],
});
const fb = (id: string, text: string): Feedback => ({ id, text });
const reply = (id: string, md: string): Reply => ({ id, md });

function state(over: {
  blocks?: Block[];
  feedback?: Record<string, Feedback[]>;
  replies?: Record<string, Reply[]>;
  history?: RoundRecord[];
}): PresentState {
  const s = emptyState();
  s.doc.blocks = over.blocks ?? [];
  s.interactions.feedback = over.feedback ?? {};
  s.interactions.replies = over.replies ?? {};
  s.rounds.history = over.history ?? [];
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

  it('marks an approval that forbids feedback as locked', () => {
    const s = state({ blocks: [approval('a1', 'Ship', false)] });
    expect(threadFeed(s, 'a1').pinned?.locked).toBe(true);
    expect(threadFeed(state({ blocks: [approval('a2', 'Ship')] }), 'a2').pinned?.locked).toBe(false);
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

  it('only includes blocks that carry a note or reply', () => {
    const s = state({
      blocks: [approval('a1'), approval('a2'), approval('a3')],
      feedback: { a1: [fb('f1', 'x')] },
      replies: { a3: [reply('r1', 'ok')] },
    });
    // a2 has nothing, so it never enters the feed; a3's lone reply still counts.
    expect(threadFeed(s, null).feed.map((e) => e.blockId)).toEqual(['a1', 'a3']);
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
