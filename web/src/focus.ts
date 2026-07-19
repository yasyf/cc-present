// The focus-mode step derivation: a pure walk over the document's top-level
// blocks that groups them into FocusSteps — one decision at a time, each with its
// non-decidable lead-in context — mirroring decide.ts's DOM-free style so it is
// unit-tested without a renderer (see focus.test.ts). FocusDeck drives its index
// off these; iOS mirrors this walk in FocusSteps.swift.

import { decidableIds, flatten, submitItems } from './decide';
import type { Block, ChoiceOption } from './schema';
import type { Interactions } from './events';

export type FocusStepKind = 'decision' | 'context';

export interface FocusStep {
  // Anchor block id — the focal block's id, stable across recomputes so the deck
  // keeps its place when the live document churns.
  id: string;
  kind: FocusStepKind;
  // Non-decidable lead-in blocks rendered above the focal block.
  context: Block[];
  // The card or leaf rendered as the step body.
  block: Block;
  // Every decidable id under this step in document order (card children inlined).
  decidables: string[];
  // The first decidable block; undefined for a context step.
  primary?: Block;
  // A lone approval — the only shape swipe-to-decide targets (P3).
  swipeable: boolean;
  // The nearest preceding section title.
  tier?: string;
}

function startsOwnStep(block: Block, packInteractive: ReadonlySet<string>): boolean {
  return block.type === 'card' || decidableIds([block], packInteractive).length > 0;
}

function anchorStep(
  block: Block,
  context: Block[],
  tier: string | undefined,
  packInteractive: ReadonlySet<string>,
): FocusStep {
  const decidables = decidableIds([block], packInteractive);
  const primary = decidables.length > 0 ? flatten([block]).find((b) => b.id === decidables[0]) : undefined;
  return {
    id: block.id,
    kind: decidables.length > 0 ? 'decision' : 'context',
    context,
    block,
    decidables,
    primary,
    swipeable: decidables.length === 1 && primary?.type === 'approval',
    tier,
  };
}

function contextStep(run: Block[], tier: string | undefined): FocusStep {
  const block = run[run.length - 1]!;
  return {
    id: block.id,
    kind: 'context',
    context: run.slice(0, -1),
    block,
    decidables: [],
    swipeable: false,
    tier,
  };
}

// focusSteps walks the top-level blocks in document order. A section flushes the
// pending run as a standalone context step and updates the tier; a card or
// decidable leaf becomes a step with the pending run as its context; every other
// block accumulates into the pending run, and a trailing run is its own context
// step.
export function focusSteps(blocks: Block[], packInteractive: ReadonlySet<string>): FocusStep[] {
  const steps: FocusStep[] = [];
  let pending: Block[] = [];
  let tier: string | undefined;

  const flush = () => {
    if (pending.length > 0) {
      steps.push(contextStep(pending, tier));
      pending = [];
    }
  };

  for (const block of blocks) {
    if (block.type === 'section') {
      flush();
      tier = block.title;
      continue;
    }
    if (startsOwnStep(block, packInteractive)) {
      steps.push(anchorStep(block, pending, tier, packInteractive));
      pending = [];
    } else {
      pending.push(block);
    }
  }
  flush();
  return steps;
}

// stepTitle is the facade label FocusPeek and FocusSummary show without mounting
// the block — the focal block's own heading, falling back to its kind.
export function stepTitle(step: FocusStep): string {
  const b = step.block;
  switch (b.type) {
    case 'card':
      return b.title ?? 'Card';
    case 'approval':
      return b.prompt ?? 'Approval';
    case 'choice':
      return b.prompt ?? 'Choice';
    case 'input':
      return b.label;
    case 'section':
      return b.title;
    default:
      return 'Details';
  }
}

// The hoisted step headline: text is the <h2> the deck pins above the scroll body,
// suppressId the decidable whose inline prompt it replaces, fromCard whether the
// text is the step card's own title (so the meta row omits a duplicate eyebrow).
export interface FocusHeadline {
  text: string | null;
  suppressId: string | null;
  fromCard: boolean;
}

// stepHeadline resolves the question a step leads with: a lone decidable's own
// prompt (hoisted, its inline copy suppressed), else a card's title (multi-decidable
// prompts stay inline as sub-headings), else null for a bare content leaf.
export function stepHeadline(step: FocusStep): FocusHeadline {
  const p = step.primary;
  if (step.decidables.length === 1 && p) {
    const text = p.type === 'input' ? p.label : p.type === 'choice' || p.type === 'approval' ? p.prompt ?? null : null;
    if (text) return { text, suppressId: p.id, fromCard: false };
  }
  if (step.block.type === 'card') return { text: step.block.title ?? null, suppressId: null, fromCard: true };
  return { text: null, suppressId: null, fromCard: false };
}

// factAxes is the aligned-grid gate: the shared ordered label list when at least
// two fact-carrying options declare the same non-empty label sequence, else null —
// any mismatch drops the comparison grid and the per-option fallback renders.
export function factAxes(options: ChoiceOption[]): string[] | null {
  const withFacts = options.filter((o) => o.facts && o.facts.length > 0);
  if (withFacts.length < 2) return null;
  const labelsOf = (o: ChoiceOption) => o.facts!.map((f) => f.label ?? '');
  const axes = labelsOf(withFacts[0]!);
  if (axes.some((l) => l === '')) return null;
  for (const o of withFacts) {
    const labels = labelsOf(o);
    if (labels.length !== axes.length || labels.some((l, i) => l !== axes[i])) return null;
  }
  return axes;
}

export type StepStatus = 'approved' | 'rejected' | 'decided' | 'undecided' | null;

// stepStatus classifies a step for the progress dots and summary receipts: null
// for a step with nothing to tally (context runs, input-only steps — inputs are
// never decided, matching the SubmitBar tally), otherwise decided/undecided, with
// approve/reject for a lone approval so its dot fills with the verdict color.
export function stepStatus(
  step: FocusStep,
  interactions: Interactions,
  packInteractive: ReadonlySet<string>,
): StepStatus {
  const items = submitItems([step.block], interactions, packInteractive);
  if (items.length === 0) return null;
  if (!items.every((i) => i.decided)) return 'undecided';
  if (step.decidables.length === 1 && step.primary?.type === 'approval') {
    return interactions.decisions[step.primary.id]?.verdict === 'rejected' ? 'rejected' : 'approved';
  }
  return 'decided';
}

const SWIPE_OFFSET = 120;
const SWIPE_VELOCITY = 600;

// swipeVerdict is the pure commit rule for a swipe on a lone approval: a drag
// past ±120px or a flick past ±600px/s commits, its sign the verdict (right =
// approved); anything short snaps back. Distance wins the direction when it
// reaches threshold, else the flick does — so a fast, short flick still commits
// toward its own direction. Unit-tested in focus.test.ts against a renderer-free
// table, mirrored by iOS.
export function swipeVerdict(offsetX: number, velocityX: number): 'approved' | 'rejected' | null {
  const byOffset = Math.abs(offsetX) >= SWIPE_OFFSET;
  const byVelocity = Math.abs(velocityX) >= SWIPE_VELOCITY;
  if (!byOffset && !byVelocity) return null;
  const dir = byOffset ? Math.sign(offsetX) : Math.sign(velocityX);
  return dir >= 0 ? 'approved' : 'rejected';
}
