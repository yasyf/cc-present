// Pure derivations for the review layer: the decidable cursor ring, the submit
// tally, and the toggle semantics shared by clicks and keystrokes. Every export
// is a plain function over blocks + the reduced interactions so it can be tested
// without a DOM (see decide.test.ts).

import { isPackBlock } from './schema';
import type { Block, BuiltinBlockType } from './schema';
import type { Interactions, Verdict } from './events';

export interface SubmitItem {
  id: string;
  kind: 'approval' | 'choice' | 'pack' | 'triage';
  decided: boolean;
}

// BUILTIN_DECIDABLE marks which built-in types join the cursor ring; inputs ride
// it for focus. BUILTIN_TALLIED maps each built-in to its submit-tally kind (null
// = not tallied). Both `satisfies` an exhaustive record so a new built-in forces
// a classification.
const BUILTIN_DECIDABLE = {
  section: false,
  card: false,
  approval: true,
  choice: true,
  input: true,
  draft: true,
  triage: true,
  markdown: false,
  code: false,
  diff: false,
  diagram: false,
  image: false,
  table: false,
  progress: false,
  chart: false,
  term: false,
  filetree: false,
  record: false,
} satisfies Record<BuiltinBlockType, boolean>;

const BUILTIN_TALLIED = {
  section: null,
  card: null,
  approval: 'approval',
  choice: 'choice',
  input: null,
  draft: null,
  triage: 'triage',
  markdown: null,
  code: null,
  diff: null,
  diagram: null,
  image: null,
  table: null,
  progress: null,
  chart: null,
  term: null,
  filetree: null,
  record: null,
} satisfies Record<BuiltinBlockType, SubmitItem['kind'] | null>;

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

// decidableIds is the cursor ring: every approval, choice, input, and interactive
// pack block in document order, card children inlined. Inputs and packs join the
// ring so `f` can focus them; interactive pack types come from the registry
// (never PresentState), passed in as packInteractive.
export function decidableIds(blocks: Block[], packInteractive: ReadonlySet<string>): string[] {
  return flatten(blocks)
    .filter((b) => (isPackBlock(b) ? packInteractive.has(b.type) : BUILTIN_DECIDABLE[b.type]))
    .map((b) => b.id);
}

// isDecided mirrors the SubmitBar tally for one block: an approval with any
// verdict (reduce.ts deletes cleared decisions, so presence is decidedness), a
// choice with at least one selected option or an other write-in, or a pack block
// with a stored interaction. Every other block is never decided.
export function isDecided(block: Block, interactions: Interactions): boolean {
  if (isPackBlock(block)) return interactions.packs[block.id] !== undefined;
  switch (block.type) {
    case 'approval':
      return interactions.decisions[block.id] !== undefined;
    case 'choice': {
      const selection = interactions.choices[block.id];
      return selection !== undefined && (selection.optionIds.length > 0 || selection.other !== undefined);
    }
    case 'triage': {
      const verdicts = interactions.triage[block.id];
      return block.items.every((item) => verdicts?.[item.id] !== undefined);
    }
    default:
      return false;
  }
}

// submitItems is the tally set — approvals, choices, and interactive pack blocks
// in document order with their decided state — driving both the count and the
// SubmitBar progress dots.
export function submitItems(
  blocks: Block[],
  interactions: Interactions,
  packInteractive: ReadonlySet<string>,
): SubmitItem[] {
  const out: SubmitItem[] = [];
  for (const block of flatten(blocks)) {
    if (isPackBlock(block)) {
      if (packInteractive.has(block.type)) {
        out.push({ id: block.id, kind: 'pack', decided: isDecided(block, interactions) });
      }
      continue;
    }
    const kind = BUILTIN_TALLIED[block.type];
    if (kind) out.push({ id: block.id, kind, decided: isDecided(block, interactions) });
  }
  return out;
}

// blockDecided drives a board row's data-decided receipt: the block has at least
// one tallied decidable and every one is decided. Unlike the shallow isDecided
// (never true for a card), it aggregates a card over its children via submitItems.
export function blockDecided(
  block: Block,
  interactions: Interactions,
  packInteractive: ReadonlySet<string>,
): boolean {
  const items = submitItems([block], interactions, packInteractive);
  return items.length > 0 && items.every((i) => i.decided);
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
