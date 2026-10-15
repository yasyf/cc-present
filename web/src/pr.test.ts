import { describe, expect, it } from 'vitest';
import { checksSummary, fetchedAgo, filePatch, verdictTone } from './pr';

describe('pr helpers', () => {
  it('summarizes checks, omitting empty buckets', () => {
    expect(checksSummary({ state: 'success', total: 4, passed: 4, failed: 0, pending: 0, skipped: 0 })).toBe('4/4 passed');
    expect(checksSummary({ state: 'failure', total: 6, passed: 2, failed: 1, pending: 2, skipped: 1 })).toBe(
      '2/6 passed · 1 failed · 2 pending · 1 skipped',
    );
  });

  it('heads a patch with the old and new paths', () => {
    expect(filePatch({ path: 'b.ts', previousPath: 'a.ts', status: 'renamed', additions: 1, deletions: 1, patch: '@@ -1 +1 @@\n-x\n+y' })).toBe(
      '--- a/a.ts\n+++ b/b.ts\n@@ -1 +1 @@\n-x\n+y',
    );
  });

  it('maps verdicts to tones', () => {
    expect(['approved', 'changes_requested', 'review_required', 'none'].map(verdictTone)).toEqual(['good', 'bad', 'warn', 'dim']);
  });

  it('ages the fetch instant', () => {
    const now = Date.parse('2026-10-14T21:00:00Z');
    expect(fetchedAgo('2026-10-14T20:59:30Z', now)).toBe('just now');
    expect(fetchedAgo('2026-10-14T20:45:00Z', now)).toBe('15m ago');
    expect(fetchedAgo('2026-10-14T18:00:00Z', now)).toBe('3h ago');
  });
});
