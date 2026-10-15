import { forwardRef } from 'react';
import { m, useIsPresent } from 'motion/react';
import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import { flatten } from '../decide';
import type { ChoiceOption } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
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

export interface StepReceiptProps {
  step: FocusStep;
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  current?: boolean;
  onSelect?: () => void;
  onDecide?: () => void;
}

export function StepReceipt({ step, interactions, packInteractive, current, onSelect, onDecide }: StepReceiptProps) {
  const status = stepStatus(step, interactions, packInteractive);
  const answer = stepAnswer(step, interactions);
  return (
    <li className={`focus-receipt${status ? ` ${status}` : ''}`} aria-current={current || undefined}>
      {onSelect ? (
        <button type="button" className="focus-receipt-title focus-receipt-jump" onClick={onSelect}>
          {stepTitle(step)}
        </button>
      ) : (
        <span className="focus-receipt-title">{stepTitle(step)}</span>
      )}
      {status === 'undecided' && onDecide ? (
        <button type="button" className="link-btn" onClick={onDecide}>
          decide
        </button>
      ) : (
        <span className="focus-receipt-state">{status ?? '—'}</span>
      )}
      {answer && <span className="focus-receipt-answer">{answer}</span>}
    </li>
  );
}

export interface FocusSummaryProps {
  steps: FocusStep[];
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  onJump: (id: string) => void;
}

// FocusSummary is the deck-end receipt list; an undecided step links back while the
// round is open. It forwards popLayout's ref so the outgoing summary is measured.
export const FocusSummary = forwardRef<HTMLDivElement, FocusSummaryProps>(function FocusSummary(
  { steps, interactions, packInteractive, onJump },
  ref,
) {
  const present = useIsPresent();
  const { closed } = usePresent();
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
        {steps.map((step) => (
          <StepReceipt
            key={step.id}
            step={step}
            interactions={interactions}
            packInteractive={packInteractive}
            onDecide={closed ? undefined : () => onJump(step.id)}
          />
        ))}
      </ul>
    </m.div>
  );
});
