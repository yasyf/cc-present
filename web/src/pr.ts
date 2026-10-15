import { withToken } from './token';

export type PRStateName = 'open' | 'draft' | 'merged' | 'landed' | 'closed';

export interface PRChecks {
  state: string;
  total: number;
  passed: number;
  failed: number;
  pending: number;
  skipped: number;
  failing?: string[];
}

export interface PRFile {
  path: string;
  previousPath?: string;
  status: string;
  additions: number;
  deletions: number;
  patch?: string;
}

export interface PRLive {
  repo: string;
  number: number;
  title: string;
  url: string;
  author: string;
  state: PRStateName;
  base: string;
  head: string;
  mergedSha?: string;
  additions: number;
  deletions: number;
  changedFiles: number;
  checks: PRChecks;
  reviewDecision: string;
  reviews: { author: string; state: string }[];
  labels: string[];
  files: PRFile[];
  fetchedAt: string;
}

export function prUrl(repo: string, number: number): string {
  return `https://github.com/${repo}/pull/${number}`;
}

export function commitUrl(repo: string, sha: string): string {
  return `https://github.com/${repo}/commit/${sha}`;
}

// fetchPR throws the daemon's own error text rather than the request URL, which
// carries the page token off loopback.
export async function fetchPR(subject: string, block: { id: string; repo: string; number: number }): Promise<PRLive> {
  const query = new URLSearchParams({ subject, block: block.id, repo: block.repo, number: String(block.number) });
  const res = await fetch(withToken(`/api/pr?${query.toString()}`));
  if (!res.ok) throw new Error((await res.text()).trim() || `GitHub fetch failed (${res.status})`);
  return (await res.json()) as PRLive;
}

export function filePatch(file: PRFile): string {
  return `--- a/${file.previousPath ?? file.path}\n+++ b/${file.path}\n${file.patch ?? ''}`;
}

export function checksSummary(c: PRChecks): string {
  const parts = [`${c.passed}/${c.total} passed`];
  if (c.failed > 0) parts.push(`${c.failed} failed`);
  if (c.pending > 0) parts.push(`${c.pending} pending`);
  if (c.skipped > 0) parts.push(`${c.skipped} skipped`);
  return parts.join(' · ');
}

export type Tone = 'good' | 'warn' | 'bad' | 'dim';

export function verdictTone(verdict: string): Tone {
  if (verdict === 'approved' || verdict === 'success') return 'good';
  if (verdict === 'changes_requested' || verdict === 'failure' || verdict === 'error') return 'bad';
  if (verdict === 'pending' || verdict === 'expected' || verdict === 'review_required') return 'warn';
  return 'dim';
}

export function fetchedAgo(iso: string, now: number): string {
  const seconds = Math.max(0, Math.round((now - Date.parse(iso)) / 1000));
  if (seconds < 60) return 'just now';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  return `${Math.round(minutes / 60)}h ago`;
}
