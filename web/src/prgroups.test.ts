import { describe, expect, it } from 'vitest';
import { allDecided, approveGroup, rowVerdict, sameHead, staleLine, tally, toggleEntry } from './prgroups';
import type { PRGroups, PRRow } from './schema';

const row = (number: number, head: string, over: Partial<PRRow> = {}): PRRow => ({
  number,
  head,
  title: `PR ${number}`,
  change: 'C.',
  plan: 'Matches the plan.',
  risk: 'R.',
  review: 'V.',
  blocker: 'Nothing.',
  ...over,
});

const block: PRGroups = {
  id: 'pg',
  type: 'pr-groups',
  repo: 'a/b',
  groups: [
    { id: 'g1', title: 'G1', summary: 'S.', prs: [row(8, 'abc1234'), row(9, 'def5678')] },
    { id: 'g2', title: 'G2', summary: 'S.', prs: [row(10, '1112223', { carried: ['0009998'] })] },
  ],
};

describe('sameHead', () => {
  it.each([
    ['abc1234', 'abc1234def', true],
    ['abc1234def', 'abc1234', true],
    ['abc1234', 'abc1235', false],
    ['', 'abc1234', false],
  ])('%s vs %s', (a, b, want) => {
    expect(sameHead(a, b)).toBe(want);
  });
});

describe('rowVerdict', () => {
  const r = block.groups[0]!.prs[0]!;
  it('is none without a decision', () => {
    expect(rowVerdict(r, undefined)).toEqual({ kind: 'none' });
  });
  it('stands at the row head, matched by prefix', () => {
    expect(rowVerdict(r, { verdict: 'approved', head: 'abc1234ffff' }).kind).toBe('current');
  });
  it('goes stale without a recorded head', () => {
    expect(rowVerdict(r, { verdict: 'approved' }).kind).toBe('stale');
  });
  it('goes stale at another head', () => {
    expect(rowVerdict(r, { verdict: 'approved', head: '9999999' }).kind).toBe('stale');
  });
  it('stands at a carried head and says so', () => {
    const carried = block.groups[1]!.prs[0]!;
    expect(rowVerdict(carried, { verdict: 'approved', head: '0009998' })).toEqual({
      kind: 'current',
      decision: { verdict: 'approved', head: '0009998' },
      carried: true,
    });
  });
});

describe('staleLine', () => {
  it('names the old head, the time, and the new head', () => {
    const line = staleLine(block.groups[0]!.prs[0]!, { verdict: 'approved', head: '9999999aaaa', at: '2026-10-26T21:03:00Z' });
    expect(line).toMatch(/^Approved at 9999999 on .+; new head abc1234 needs your tap\.$/);
  });
});

describe('toggleEntry', () => {
  const r = block.groups[0]!.prs[0]!;
  it('records the target at the row head, keeping the note', () => {
    expect(toggleEntry(r, { verdict: 'rejected', head: 'abc1234', note: 'wait' }, 'approved')).toEqual({
      verdict: 'approved',
      head: 'abc1234',
      note: 'wait',
    });
  });
  it('clears the same standing verdict', () => {
    expect(toggleEntry(r, { verdict: 'approved', head: 'abc1234' }, 'approved')).toEqual({ verdict: 'cleared' });
  });
  it('re-records a stale approval at the new head instead of clearing it', () => {
    expect(toggleEntry(r, { verdict: 'approved', head: '9999999' }, 'approved')).toEqual({ verdict: 'approved', head: 'abc1234' });
  });
});

describe('approveGroup', () => {
  it('fans out to every row not already approved at its head', () => {
    const merge = approveGroup(block.groups[0]!, {
      '8': { verdict: 'approved', head: 'abc1234' },
      '9': { verdict: 'rejected', head: 'def5678', note: 'flaky' },
    });
    expect(merge).toEqual({ '9': { verdict: 'approved', head: 'def5678', note: 'flaky' } });
  });
});

describe('tally and allDecided', () => {
  it('counts a stale verdict as waiting', () => {
    const verdicts = {
      '8': { verdict: 'approved' as const, head: 'abc1234' },
      '9': { verdict: 'rejected' as const, head: 'def5678' },
      '10': { verdict: 'approved' as const, head: '7777777' },
    };
    expect(tally(block.groups.flatMap((g) => g.prs), verdicts)).toEqual({ approved: 1, held: 1, open: 1 });
    expect(allDecided(block, verdicts)).toBe(false);
    expect(allDecided(block, { ...verdicts, '10': { verdict: 'approved', head: '1112223' } })).toBe(true);
  });
});
