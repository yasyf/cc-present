// Pure derivations for the pr-groups block: verdict keys, head matching, and
// whether a recorded verdict still stands for a row's current head.

import type { Decision, Verdict } from './events';
import type { PRGroup, PRGroups, PRRow } from './schema';

export function prRowId(number: number): string {
  return String(number);
}

// sameHead mirrors doc.SameHead: two shas name one commit when the shorter is a
// prefix of the longer.
export function sameHead(a: string | undefined, b: string | undefined): boolean {
  if (!a || !b) return false;
  const [short, long] = a.length <= b.length ? [a, b] : [b, a];
  return long.startsWith(short);
}

export function shortHead(head: string): string {
  return head.slice(0, 7);
}

export type RowVerdict =
  | { kind: 'none' }
  | { kind: 'current'; decision: Decision; carried: boolean }
  | { kind: 'stale'; decision: Decision };

// rowVerdict reads a row's recorded verdict against its head: given at this
// head, or at a head the agent marked carried, it stands; given at any other
// head, it is stale and the row needs a fresh tap.
export function rowVerdict(row: PRRow, decision: Decision | undefined): RowVerdict {
  if (!decision) return { kind: 'none' };
  if (sameHead(decision.head, row.head)) return { kind: 'current', decision, carried: false };
  if (row.carried?.some((h) => sameHead(h, decision.head))) return { kind: 'current', decision, carried: true };
  return { kind: 'stale', decision };
}

export function verdictWord(verdict: Verdict): string {
  return verdict === 'approved' ? 'Approved' : 'Held';
}

export function formatAt(iso: string | undefined): string {
  if (!iso) return 'an earlier time';
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

export function staleLine(row: PRRow, decision: Decision): string {
  const tapped = decision.carriedFrom ?? decision.head;
  const given = tapped ? ` at ${shortHead(tapped)}` : '';
  return `${verdictWord(decision.verdict)}${given} on ${formatAt(decision.at)}; new head ${shortHead(row.head)} needs your tap.`;
}

// givenLine says where a standing verdict was given; a verdict the agent carried
// to a new head names the head the human tapped and the agent's reason.
export function givenLine(v: Extract<RowVerdict, { kind: 'current' }>, row: PRRow): string {
  const { decision } = v;
  const at = decision.at ? ` on ${formatAt(decision.at)}` : '';
  if (decision.carriedFrom) {
    return `Your tap at ${shortHead(decision.carriedFrom)}${at} carries to ${shortHead(decision.head ?? row.head)}: ${decision.carryNote ?? ''}`;
  }
  const carried = v.carried ? `; the agent carried it to ${shortHead(row.head)}` : '';
  return `${verdictWord(decision.verdict)} at ${shortHead(decision.head ?? row.head)}${at}${carried}`;
}

export interface Tally {
  approved: number;
  held: number;
  open: number;
}

export function tally(rows: readonly PRRow[], verdicts: Record<string, Decision>): Tally {
  const out: Tally = { approved: 0, held: 0, open: 0 };
  for (const row of rows) {
    const v = rowVerdict(row, verdicts[prRowId(row.number)]);
    if (v.kind !== 'current') out.open++;
    else if (v.decision.verdict === 'approved') out.approved++;
    else out.held++;
  }
  return out;
}

export function allRows(block: PRGroups): PRRow[] {
  return block.groups.flatMap((g) => g.prs);
}

export function narrowToRow(block: PRGroups, number: number): PRGroups | undefined {
  const group = block.groups.find((g) => g.prs.some((r) => r.number === number));
  if (!group) return undefined;
  return { ...block, groups: [{ ...group, prs: group.prs.filter((r) => r.number === number) }] };
}

// allDecided is the submit tally: every row carries a verdict that stands.
export function allDecided(block: PRGroups, verdicts: Record<string, Decision> | undefined): boolean {
  return allRows(block).every((row) => rowVerdict(row, verdicts?.[prRowId(row.number)]).kind === 'current');
}

export interface VerdictEntry {
  verdict: Verdict;
  head?: string;
  note?: string;
}

// toggleEntry is what one tap on Approve or Hold posts: the same standing verdict
// clears, anything else records target at the row's head with the note carried.
export function toggleEntry(row: PRRow, decision: Decision | undefined, target: 'approved' | 'rejected'): VerdictEntry {
  const v = rowVerdict(row, decision);
  if (v.kind === 'current' && v.decision.verdict === target) return { verdict: 'cleared' };
  const entry: VerdictEntry = { verdict: target, head: row.head };
  if (decision?.note) entry.note = decision.note;
  return entry;
}

// approveGroup fans one group-level tap out to every row not already approved
// at its head, carrying each row's note forward.
export function approveGroup(group: PRGroup, verdicts: Record<string, Decision>): Record<string, VerdictEntry> {
  const out: Record<string, VerdictEntry> = {};
  for (const row of group.prs) {
    const id = prRowId(row.number);
    const v = rowVerdict(row, verdicts[id]);
    if (v.kind === 'current' && v.decision.verdict === 'approved') continue;
    const entry: VerdictEntry = { verdict: 'approved', head: row.head };
    if (verdicts[id]?.note) entry.note = verdicts[id]!.note;
    out[id] = entry;
  }
  return out;
}
