// Pure derivations for the SubmitBar's armed confirm.

import type { SubmitItem } from './decide';

// undecidedKey folds a set of undecided item ids into a stable string, so the
// armed confirm keys on the exact set: a same-count block swap still changes it.
export function undecidedKey(ids: readonly string[]): string {
  return [...ids].sort().join('\n');
}

const SUMMARY_NOUNS: readonly [kind: SubmitItem['kind'], one: string, many: string][] = [
  ['approval', 'approval', 'approvals'],
  ['choice', 'choice', 'choices'],
  ['triage', 'triage list', 'triage lists'],
  ['pack', 'block', 'blocks'],
];

export function unansweredSummary(items: readonly SubmitItem[]): string {
  const counts = SUMMARY_NOUNS.flatMap(([kind, one, many]) => {
    const count = items.filter((i) => i.kind === kind && !i.decided).length;
    return count === 0 ? [] : [`${count} ${count === 1 ? one : many}`];
  });
  return `${counts.join(', ')} unanswered`;
}
