// The focus-mode step derivation: a pure walk over the document's top-level
// blocks that groups them into FocusSteps — one decision at a time, each with its
// non-decidable lead-in context — mirroring decide.ts's DOM-free style so it is
// unit-tested without a renderer (see focus.test.ts). FocusDeck drives its index
// off these; iOS mirrors this walk in FocusSteps.swift.

import { decidableIds, flatten, submitItems } from './decide';
import type { Block } from './schema';
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
