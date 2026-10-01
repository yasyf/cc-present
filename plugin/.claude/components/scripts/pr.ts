// PR_QUERY fetches everything pr.card and pr.commits render in one GraphQL call.
export const PR_QUERY = `query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      number title url state isDraft mergedAt
      author { login }
      baseRefName headRefName
      mergeCommit { oid }
      additions deletions changedFiles
      reviewDecision
      labels(first: 20) { nodes { name } }
      latestOpinionatedReviews(first: 20) { nodes { author { login } state } }
      commits(last: 100) {
        totalCount
        nodes { commit { oid message author { name user { login } } } }
      }
      head: commits(last: 1) {
        nodes { commit { statusCheckRollup { state contexts(first: 100) { nodes {
          __typename
          ... on CheckRun { name status conclusion }
          ... on StatusContext { context state }
        } } } } }
      }
    }
  }
}`;

type CheckContext =
  | { __typename: 'CheckRun'; name: string; status: string; conclusion: string | null }
  | { __typename: 'StatusContext'; context: string; state: string };

export interface GhPullRequest {
  number: number;
  title: string;
  url: string;
  state: 'OPEN' | 'CLOSED' | 'MERGED';
  isDraft: boolean;
  mergedAt: string | null;
  author: { login: string } | null;
  baseRefName: string;
  headRefName: string;
  mergeCommit: { oid: string } | null;
  additions: number;
  deletions: number;
  changedFiles: number;
  reviewDecision: 'APPROVED' | 'CHANGES_REQUESTED' | 'REVIEW_REQUIRED' | null;
  labels: { nodes: { name: string }[] };
  latestOpinionatedReviews: { nodes: { author: { login: string } | null; state: string }[] };
  commits: {
    totalCount: number;
    nodes: { commit: { oid: string; message: string; author: { name: string; user: { login: string } | null } | null } }[];
  };
  head: {
    nodes: { commit: { statusCheckRollup: { state: string; contexts: { nodes: CheckContext[] } } | null } }[];
  };
}

export interface Checks {
  state: string;
  total: number;
  passed: number;
  failed: number;
  pending: number;
  skipped: number;
  failing?: string[];
}

const FAILED_CONCLUSIONS = new Set(['FAILURE', 'TIMED_OUT', 'CANCELLED', 'ACTION_REQUIRED', 'STARTUP_FAILURE', 'STALE']);
const SKIPPED_CONCLUSIONS = new Set(['NEUTRAL', 'SKIPPED']);

type Outcome = 'passed' | 'failed' | 'pending' | 'skipped';

function outcome(ctx: CheckContext): Outcome {
  if (ctx.__typename === 'StatusContext') {
    if (ctx.state === 'SUCCESS') return 'passed';
    if (ctx.state === 'FAILURE' || ctx.state === 'ERROR') return 'failed';
    return 'pending';
  }
  if (ctx.status !== 'COMPLETED' || ctx.conclusion === null) return 'pending';
  if (ctx.conclusion === 'SUCCESS') return 'passed';
  if (FAILED_CONCLUSIONS.has(ctx.conclusion)) return 'failed';
  if (SKIPPED_CONCLUSIONS.has(ctx.conclusion)) return 'skipped';
  throw new Error(`unknown check conclusion ${ctx.conclusion}`);
}

export function summarizeChecks(pr: GhPullRequest): Checks {
  const rollup = pr.head.nodes[0]?.commit.statusCheckRollup ?? null;
  const checks: Checks = { state: 'none', total: 0, passed: 0, failed: 0, pending: 0, skipped: 0 };
  if (!rollup) return checks;
  checks.state = rollup.state.toLowerCase();
  const failing: string[] = [];
  for (const ctx of rollup.contexts.nodes) {
    const o = outcome(ctx);
    checks[o] += 1;
    checks.total += 1;
    if (o === 'failed') failing.push(ctx.__typename === 'CheckRun' ? ctx.name : ctx.context);
  }
  if (failing.length > 0) checks.failing = failing.slice(0, 10);
  return checks;
}

function prState(pr: GhPullRequest, landedSha: string | null): string {
  if (pr.state === 'MERGED') return 'merged';
  if (pr.state === 'OPEN') return pr.isDraft ? 'draft' : 'open';
  return landedSha ? 'landed' : 'closed';
}

// needsLandedSearch reports whether a closed PR may have landed outside GitHub's
// merge button, as a merge queue's squash commit ending "(#N)" on the base.
export function needsLandedSearch(pr: GhPullRequest): boolean {
  return pr.state === 'CLOSED' && pr.mergedAt === null;
}

// landedShaFrom picks the commit whose subject ends with the PR's "(#N)" suffix
// from a search/commits response.
export function landedShaFrom(number: number, items: { sha: string; commit: { message: string } }[]): string | null {
  const suffix = `(#${number})`;
  const hit = items.find((it) => (it.commit.message.split('\n')[0] ?? '').trimEnd().endsWith(suffix));
  return hit ? hit.sha : null;
}

export interface CardFields {
  title: string;
  url: string;
  author: string;
  state: string;
  base: string;
  head: string;
  merged_sha?: string;
  additions: number;
  deletions: number;
  changed_files: number;
  checks: Checks;
  review_decision: string;
  reviews: { author: string; state: string }[];
  labels: string[];
  fetched_at: string;
}

export function cardFields(pr: GhPullRequest, landedSha: string | null, fetchedAt: string): CardFields {
  const merged = pr.mergeCommit?.oid ?? landedSha;
  return {
    title: pr.title,
    url: pr.url,
    author: pr.author?.login ?? 'ghost',
    state: prState(pr, landedSha),
    base: pr.baseRefName,
    head: pr.headRefName,
    ...(merged ? { merged_sha: merged } : {}),
    additions: pr.additions,
    deletions: pr.deletions,
    changed_files: pr.changedFiles,
    checks: summarizeChecks(pr),
    review_decision: (pr.reviewDecision ?? 'none').toLowerCase(),
    reviews: pr.latestOpinionatedReviews.nodes.map((r) => ({
      author: r.author?.login ?? 'ghost',
      state: r.state.toLowerCase(),
    })),
    labels: pr.labels.nodes.map((l) => l.name),
    fetched_at: fetchedAt,
  };
}

export interface CommitsFields {
  commits: { sha: string; subject: string; author: string }[];
  total: number;
  fetched_at: string;
}

export function commitsFields(pr: GhPullRequest, fetchedAt: string): CommitsFields {
  return {
    commits: pr.commits.nodes.map(({ commit }) => ({
      sha: commit.oid,
      subject: commit.message.split('\n')[0] ?? '',
      author: commit.author?.user?.login ?? commit.author?.name ?? 'unknown',
    })),
    total: pr.commits.totalCount,
    fetched_at: fetchedAt,
  };
}

interface FileSection {
  path: string;
  header: string[];
  hunks: string[][];
}

const DIFF_GIT = /^diff --git a\/(.*) b\/(.*)$/;

export function splitFiles(diff: string): FileSection[] {
  const files: FileSection[] = [];
  let file: FileSection | null = null;
  for (const line of diff.split('\n')) {
    const head = DIFF_GIT.exec(line);
    if (head) {
      file = { path: head[2] ?? '', header: [line], hunks: [] };
      files.push(file);
      continue;
    }
    if (!file) continue;
    if (line.startsWith('@@')) file.hunks.push([line]);
    else if (file.hunks.length > 0) file.hunks[file.hunks.length - 1]?.push(line);
    else file.header.push(line);
  }
  for (const f of files) {
    const last = f.hunks[f.hunks.length - 1];
    while (last && last.length > 1 && last[last.length - 1] === '') last.pop();
  }
  return files;
}

function pathMatches(pattern: string, path: string): boolean {
  const dir = pattern.replace(/\/+$/, '');
  return path === dir || path.startsWith(`${dir}/`) || new Bun.Glob(pattern).match(path);
}

export interface Selector {
  paths?: string[];
  match?: string;
}

// selectPatch narrows a PR's unified diff to the files under `paths`, in the
// order the patterns are listed, then to the hunks whose text matches `match`.
export function selectPatch(diff: string, sel: Selector): string {
  const files = splitFiles(diff);
  let picked: FileSection[];
  if (sel.paths) {
    const seen = new Set<FileSection>();
    picked = [];
    for (const pattern of sel.paths) {
      for (const f of files) {
        if (!seen.has(f) && pathMatches(pattern, f.path)) {
          seen.add(f);
          picked.push(f);
        }
      }
    }
  } else {
    picked = files;
  }
  if (sel.match) {
    const re = new RegExp(sel.match, 'm');
    picked = picked
      .map((f) => ({ ...f, hunks: f.hunks.filter((h) => re.test(h.join('\n'))) }))
      .filter((f) => f.hunks.length > 0);
  }
  return picked.flatMap((f) => [...f.header, ...f.hunks.flat()]).join('\n');
}
