import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import type { Interactions } from '../events';

export interface FocusSummaryProps {
  steps: FocusStep[];
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  onJump: (id: string) => void;
}

// FocusSummary is the deck-end receipt: one row per step with its verdict, and an
// undecided item as a jump-link back to its step. The SubmitBar stays mounted
// below as the single submit path.
export function FocusSummary({ steps, interactions, packInteractive, onJump }: FocusSummaryProps) {
  return (
    <div className="focus-summary" tabIndex={-1}>
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
    </div>
  );
}
