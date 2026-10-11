import { describe, expect, it } from 'vitest';
import { unansweredSummary, undecidedKey } from './submit';
import type { SubmitItem } from './decide';

const items = (...kinds: [SubmitItem['kind'], boolean][]): SubmitItem[] =>
  kinds.map(([kind, decided], i) => ({ id: `i${i}`, kind, decided }));

describe('undecidedKey', () => {
  it('is order-independent', () => {
    expect(undecidedKey(['a2', 'a1'])).toBe(undecidedKey(['a1', 'a2']));
  });

  it('changes when a same-count block swap replaces an undecided id', () => {
    expect(undecidedKey(['a1', 'a2'])).not.toBe(undecidedKey(['a1', 'a3']));
  });

  it('is empty when nothing is undecided', () => {
    expect(undecidedKey([])).toBe('');
  });
});

describe('unansweredSummary', () => {
  it.each<[string, SubmitItem[], string]>([
    ['one approval', items(['approval', false]), '1 approval unanswered'],
    ['plural approvals', items(['approval', false], ['approval', false]), '2 approvals unanswered'],
    ['a lone choice', items(['choice', false]), '1 choice unanswered'],
    ['a lone triage', items(['triage', false]), '1 triage list unanswered'],
    ['plural triages', items(['triage', false], ['triage', false]), '2 triage lists unanswered'],
    ['a lone pack', items(['pack', false]), '1 block unanswered'],
    ['plural packs', items(['pack', false], ['pack', false]), '2 blocks unanswered'],
    [
      'kinds in approval, choice, triage, pack order whatever the document order',
      items(['pack', false], ['triage', false], ['choice', false], ['approval', false], ['approval', false]),
      '2 approvals, 1 choice, 1 triage list, 1 block unanswered',
    ],
    [
      'only the undecided of each kind',
      items(['approval', true], ['approval', false], ['choice', true], ['choice', false], ['choice', false]),
      '1 approval, 2 choices unanswered',
    ],
  ])('%s', (_name, given, summary) => {
    expect(unansweredSummary(given)).toBe(summary);
  });
});
