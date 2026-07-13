import { forwardRef } from 'react';
import { m, useIsPresent } from 'motion/react';
import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import type { Interactions } from '../events';
import { cardVariants } from './focusMotion';

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
            </li>
          );
        })}
      </ul>
    </m.div>
  );
});
