import { describe, expect, test } from 'bun:test';
import { cardFields, commitsFields, landedShaFrom, needsLandedSearch, selectPatch, splitFiles } from './pr';
import type { GhPullRequest } from './pr';

const FETCHED = '2026-10-01T07:55:00.000Z';

function pr(patch: Partial<GhPullRequest> = {}): GhPullRequest {
  return {
    number: 28605,
    title: 'release: ship only picked stacks',
    url: 'https://github.com/Forge-AI/monorepo/pull/28605',
    state: 'OPEN',
    isDraft: false,
    mergedAt: null,
    author: { login: 'yasyf' },
    baseRefName: 'dev',
    headRefName: 'yasyf/picks-only',
    mergeCommit: null,
    additions: 465,
    deletions: 590,
    changedFiles: 61,
    reviewDecision: 'APPROVED',
    labels: { nodes: [{ name: 'externally-merged' }] },
    latestOpinionatedReviews: {
      nodes: [
        { author: { login: 'poetic-svc' }, state: 'APPROVED' },
        { author: null, state: 'CHANGES_REQUESTED' },
      ],
    },
    commits: {
      totalCount: 2,
      nodes: [
        { commit: { oid: 'aaaaaaaaaa11', message: 'first\n\nbody', author: { name: 'Y M', user: { login: 'yasyf' } } } },
        { commit: { oid: 'bbbbbbbbbb22', message: 'second', author: { name: 'Bot', user: null } } },
      ],
    },
    head: {
      nodes: [
        {
          commit: {
            statusCheckRollup: {
              state: 'FAILURE',
              contexts: {
                nodes: [
                  { __typename: 'CheckRun', name: 'go', status: 'COMPLETED', conclusion: 'SUCCESS' },
                  { __typename: 'CheckRun', name: 'web', status: 'COMPLETED', conclusion: 'FAILURE' },
                  { __typename: 'CheckRun', name: 'docs', status: 'COMPLETED', conclusion: 'SKIPPED' },
                  { __typename: 'CheckRun', name: 'ios', status: 'IN_PROGRESS', conclusion: null },
                  { __typename: 'StatusContext', context: 'buildkite/ci', state: 'ERROR' },
                  { __typename: 'StatusContext', context: 'graphite/stack', state: 'PENDING' },
                ],
              },
            },
          },
        },
      ],
    },
    ...patch,
  };
}

describe('cardFields', () => {
  test('maps an open PR with mixed checks and reviews', () => {
    expect(cardFields(pr(), null, FETCHED)).toEqual({
      title: 'release: ship only picked stacks',
      url: 'https://github.com/Forge-AI/monorepo/pull/28605',
      author: 'yasyf',
      state: 'open',
      base: 'dev',
      head: 'yasyf/picks-only',
      additions: 465,
      deletions: 590,
      changed_files: 61,
      checks: {
        state: 'failure',
        total: 6,
        passed: 1,
        failed: 2,
        pending: 2,
        skipped: 1,
        failing: ['web', 'buildkite/ci'],
      },
      review_decision: 'approved',
      reviews: [
        { author: 'poetic-svc', state: 'approved' },
        { author: 'ghost', state: 'changes_requested' },
      ],
      labels: ['externally-merged'],
      fetched_at: FETCHED,
    });
  });

  test.each([
    [{ state: 'OPEN', isDraft: true }, null, 'draft', undefined],
    [{ state: 'MERGED', mergedAt: '2026-10-01T04:00:00Z', mergeCommit: { oid: 'cafe1234567' } }, null, 'merged', 'cafe1234567'],
    [{ state: 'CLOSED' }, 'bffe99b24f04', 'landed', 'bffe99b24f04'],
    [{ state: 'CLOSED' }, null, 'closed', undefined],
  ] as const)('state %o with landed %s is %s', (patch, landed, state, sha) => {
    const fields = cardFields(pr(patch as Partial<GhPullRequest>), landed, FETCHED);
    expect(fields.state).toBe(state);
    expect(fields.merged_sha).toBe(sha);
  });

  test('a PR without a rollup has no checks', () => {
    const fields = cardFields(pr({ head: { nodes: [{ commit: { statusCheckRollup: null } }] } }), null, FETCHED);
    expect(fields.checks).toEqual({ state: 'none', total: 0, passed: 0, failed: 0, pending: 0, skipped: 0 });
  });

  test('an unknown conclusion fails loudly', () => {
    const odd = pr({
      head: {
        nodes: [
          {
            commit: {
              statusCheckRollup: {
                state: 'SUCCESS',
                contexts: { nodes: [{ __typename: 'CheckRun', name: 'x', status: 'COMPLETED', conclusion: 'MYSTERY' }] },
              },
            },
          },
        ],
      },
    });
    expect(() => cardFields(odd, null, FETCHED)).toThrow('unknown check conclusion MYSTERY');
  });
});

describe('landed detection', () => {
  test('only a closed, unmerged PR searches for its squash', () => {
    expect(needsLandedSearch(pr({ state: 'CLOSED' }))).toBe(true);
    expect(needsLandedSearch(pr({ state: 'MERGED', mergedAt: '2026-10-01T04:00:00Z' }))).toBe(false);
    expect(needsLandedSearch(pr())).toBe(false);
  });

  test('picks the commit whose subject ends with the PR suffix', () => {
    const items = [
      { sha: '8d10ea25', commit: { message: 'ci: answer DMs (#28623)\n\nmentions (#28605) in the body' } },
      { sha: 'bffe99b2', commit: { message: 'release: ship only picked stacks (#28605)\n\nContext: …' } },
    ];
    expect(landedShaFrom(28605, items)).toBe('bffe99b2');
    expect(landedShaFrom(28606, items)).toBeNull();
  });
});

test('commitsFields keeps order and falls back to the author name', () => {
  expect(commitsFields(pr(), FETCHED)).toEqual({
    commits: [
      { sha: 'aaaaaaaaaa11', subject: 'first', author: 'yasyf' },
      { sha: 'bbbbbbbbbb22', subject: 'second', author: 'Bot' },
    ],
    total: 2,
    fetched_at: FETCHED,
  });
});

const DIFF = `diff --git a/go/ci/release.go b/go/ci/release.go
index 1111111..2222222 100644
--- a/go/ci/release.go
+++ b/go/ci/release.go
@@ -10,3 +10,3 @@ func Release() {
-	pull := deps
+	pull := picks
 	return pull
@@ -40,2 +40,3 @@ func Footer() {
 	x := 1
+	warn("footer")
diff --git a/go/ci/internal/release/selection/selection.go b/go/ci/internal/release/selection/selection.go
index 3333333..4444444 100644
--- a/go/ci/internal/release/selection/selection.go
+++ b/go/ci/internal/release/selection/selection.go
@@ -1,2 +1,2 @@
-// closure pulls deps
+// picks only
 package selection
diff --git a/docs/release.md b/docs/release.md
index 5555555..6666666 100644
--- a/docs/release.md
+++ b/docs/release.md
@@ -5 +5 @@
-Deps pull.
+Picks only.
`;

describe('selectPatch', () => {
  test('splits per file with the new path', () => {
    expect(splitFiles(DIFF).map((f) => [f.path, f.hunks.length])).toEqual([
      ['go/ci/release.go', 2],
      ['go/ci/internal/release/selection/selection.go', 1],
      ['docs/release.md', 1],
    ]);
  });

  test('no selector returns the whole diff', () => {
    expect(selectPatch(DIFF, {})).toBe(DIFF.trimEnd());
  });

  test('paths order the files as listed, by glob or directory prefix', () => {
    const out = selectPatch(DIFF, { paths: ['docs/*.md', 'go/ci/internal/release'] });
    expect(out.split('\n').filter((l) => l.startsWith('diff --git'))).toEqual([
      'diff --git a/docs/release.md b/docs/release.md',
      'diff --git a/go/ci/internal/release/selection/selection.go b/go/ci/internal/release/selection/selection.go',
    ]);
  });

  test('match keeps only matching hunks and drops emptied files', () => {
    const out = selectPatch(DIFF, { match: 'warn\\(' });
    expect(out).toBe(
      [
        'diff --git a/go/ci/release.go b/go/ci/release.go',
        'index 1111111..2222222 100644',
        '--- a/go/ci/release.go',
        '+++ b/go/ci/release.go',
        '@@ -40,2 +40,3 @@ func Footer() {',
        ' \tx := 1',
        '+\twarn("footer")',
      ].join('\n'),
    );
  });

  test('a selector that matches nothing yields an empty patch', () => {
    expect(selectPatch(DIFF, { paths: ['web/**'] })).toBe('');
  });
});
