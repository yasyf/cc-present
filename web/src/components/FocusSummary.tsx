import { forwardRef } from 'react';
import { m, useIsPresent } from 'motion/react';
import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import { flatten } from '../decide';
import type { ChoiceOption } from '../schema';
import type { Interactions } from '../events';
import { cardVariants } from './focusMotion';

function optionLabel(options: ChoiceOption[], id: string): string {
  return options.find((o) => o.id === id)?.label ?? id;
}

// stepAnswer is the human's committed value, shown beneath the receipt title so the
// review surfaces the pick — a choice's chosen labels and any write-in (quoted), an
// input's text. An approval's verdict already shows in the state pill, so it adds none.
function stepAnswer(step: FocusStep, interactions: Interactions): string | null {
  const parts: string[] = [];
  for (const block of flatten([step.block])) {
    if (block.type === 'choice') {
      const selection = interactions.choices[block.id];
      if (!selection) continue;
      const labels = selection.optionIds.map((id) => optionLabel(block.options, id));
      if (selection.other) labels.push(`"${selection.other}"`);
      if (labels.length > 0) parts.push(labels.join(', '));
    } else if (block.type === 'input') {
      const text = interactions.inputs[block.id]?.text.trim();
      if (text) parts.push(text);
    }
  }
  return parts.length > 0 ? parts.join(' · ') : null;
}

export interface FocusSummaryProps {
  steps: FocusStep[];
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  onJump: (id: string) => void;
}

// FocusSummary is the deck-end receipt: one row per step with its verdict, and an
// undecided item as a jump-link back to its step. The SubmitBar stays mounted
// below as the single submit path. It forwards the ref AnimatePresence's popLayout
// mode injects so the outgoing summary can be measured and lifted out of flow.
export const FocusSummary = forwardRef<HTMLDivElement, FocusSummaryProps>(function FocusSummary(
  { steps, interactions, packInteractive, onJump },
  ref,
) {
  const present = useIsPresent();
  return (
    <m.div
      ref={ref}
      className="focus-summary"
      tabIndex={-1}
      data-exiting={!present || undefined}
      variants={cardVariants}
      initial="enter"
      animate="center"
      exit="exit"
    >
      <div className="focus-summary-head">Review</div>
      <ul className="focus-receipts">
        {steps.map((step) => {
          const status = stepStatus(step, interactions, packInteractive);
          const answer = stepAnswer(step, interactions);
          return (
            <li key={step.id} className={`focus-receipt${status ? ` ${status}` : ''}`}>
              <span className="focus-receipt-title">{stepTitle(step)}</span>
              {status === 'undecided' ? (
                <button type="button" className="link-btn" onClick={() => onJump(step.id)}>
                  decide
                </button>
              ) : (
                <span className="focus-receipt-state">{status ?? '—'}</span>
              )}
              {answer && <span className="focus-receipt-answer">{answer}</span>}
            </li>
          );
        })}
      </ul>
    </m.div>
  );
});
