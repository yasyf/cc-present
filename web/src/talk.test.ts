import { describe, expect, it } from 'vitest';
import type { Block } from './schema';
import { talkEntries } from './talk';

const talk: Block = { id: 'talk', type: 'section', title: 'Talk' };
const card = (id: string, re?: string): Block => ({ id, type: 'card', re, children: [] });

function ids(entries: ReturnType<typeof talkEntries>): string[] {
  return entries.map((e) => (e.kind === 'note' ? e.note.id : e.kind === 'reply' ? e.reply.id : e.block.id));
}

describe('talkEntries', () => {
  it('draws each reply and card beneath the note it answers, in send order', () => {
    const entries = talkEntries(
      [talk, card('c2', 'n2'), card('c1', 'n1')],
      'talk',
      [
        { id: 'n1', text: 'first' },
        { id: 'n2', text: 'second' },
      ],
      [
        { id: 'r2', md: 'b', re: 'n2' },
        { id: 'r1', md: 'a', re: 'n1' },
      ],
    );
    expect(ids(entries)).toEqual(['n1', 'r1', 'c1', 'n2', 'r2', 'c2']);
  });

  it('puts what answers no note first, and a card naming an unknown note with it', () => {
    const entries = talkEntries(
      [talk, card('c1'), card('c2', 'gone')],
      'talk',
      [{ id: 'n1', text: 'first' }],
      [{ id: 'r1', md: 'a' }],
    );
    expect(ids(entries)).toEqual(['r1', 'c1', 'c2', 'n1']);
  });

  it('never draws the talk section itself', () => {
    expect(talkEntries([talk], 'talk', [], [])).toEqual([]);
  });
});
