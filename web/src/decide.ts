// Pure derivations for the review layer: the decidable cursor ring, the submit
// tally, and the toggle semantics shared by clicks and keystrokes. Every export
// is a plain function over blocks + the reduced interactions so it can be tested
// without a DOM (see decide.test.ts).

import type { Block } from './schema';
import type { Interactions, Verdict } from './events';

export interface SubmitItem {
  id: string;
  kind: 'approval' | 'choice';
  decided: boolean;
}

// flatten yields every top-level block plus every card child, so a tally or the
// cursor ring spans interactive blocks wherever they nest one level deep.
export function flatten(blocks: Block[]): Block[] {
  const out: Block[] = [];
  for (const block of blocks) {
    out.push(block);
    if (block.type === 'card') out.push(...block.children);
  }
  return out;
}

// decidableIds is the cursor ring: every approval, choice, and input in document
// order, card children inlined. Inputs join the ring so `f` can focus them even
// though they never count toward the submit tally.
export function decidableIds(blocks: Block[]): string[] {
  return flatten(blocks)
    .filter((b) => b.type === 'approval' || b.type === 'choice' || b.type === 'input')
    .map((b) => b.id);
}

// isDecided mirrors the SubmitBar tally for one block: an approval with any
// verdict (reduce.ts deletes cleared decisions, so presence is decidedness) or a
// choice with at least one selected option. Every other block is never decided.
export function isDecided(block: Block, interactions: Interactions): boolean {
  switch (block.type) {
    case 'approval':
      return interactions.decisions[block.id] !== undefined;
    case 'choice':
      return (interactions.choices[block.id]?.optionIds.length ?? 0) > 0;
    default:
      return false;
  }
}

// submitItems is the tally set — approvals and choices in document order with
// their decided state — driving both the count and the SubmitBar progress dots.
export function submitItems(blocks: Block[], interactions: Interactions): SubmitItem[] {
  const out: SubmitItem[] = [];
  for (const block of flatten(blocks)) {
    if (block.type === 'approval' || block.type === 'choice') {
      out.push({ id: block.id, kind: block.type, decided: isDecided(block, interactions) });
    }
  }
  return out;
}

// step moves the cursor one place along the ring, clamped at both ends. A null
// cursor enters at the first (forward) or last (back) member; a cursor that has
// left the ring re-enters the same way.
export function step(ring: string[], from: string | null, delta: 1 | -1): string | null {
  if (ring.length === 0) return null;
  const i = from === null ? -1 : ring.indexOf(from);
  if (i === -1) return delta === 1 ? ring[0]! : ring[ring.length - 1]!;
  const next = i + delta;
  return next < 0 || next >= ring.length ? from : ring[next]!;
}

// nextUndecided is the next ring member strictly after `from` (wrapping to the
// head) whose id is undecided; null when nothing is undecided. A null or
// off-ring `from` starts the search at the ring's head.
export function nextUndecided(ring: string[], undecided: Set<string>, from: string | null): string | null {
  if (undecided.size === 0) return null;
  const start = from === null ? -1 : ring.indexOf(from);
  for (let hop = 1; hop <= ring.length; hop++) {
    const id = ring[(start + hop) % ring.length];
    if (id !== undefined && undecided.has(id)) return id;
  }
  return null;
}

// verdictToggle gives the verdict a re-press produces: the same target clears,
// the opposite switches — parity with the click path.
export function verdictToggle(current: Verdict | undefined, target: 'approved' | 'rejected'): Verdict {
  return current === target ? 'cleared' : target;
}

// choiceToggle gives the option-id set a toggle of `optionId` produces: multi
// adds or removes, single replaces or clears.
export function choiceToggle(selected: readonly string[], optionId: string, multi: boolean): string[] {
  if (multi) {
    return selected.includes(optionId)
      ? selected.filter((id) => id !== optionId)
      : [...selected, optionId];
  }
  return selected.includes(optionId) ? [] : [optionId];
}
