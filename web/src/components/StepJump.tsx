import { useEffect, useRef, useState } from 'react';
import type { FocusStep, Remaining } from '../focus';
import type { Interactions, RoundRecord } from '../events';
import { useKeyboardApi } from '../keyboard';
import { useActiveBlock } from '../activeBlock';
import { StepReceipt } from './FocusSummary';
import { HistoryDrawer } from './HistoryDrawer';

export interface StepJumpProps {
  steps: FocusStep[];
  current?: number;
  progress: Remaining;
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  history: RoundRecord[];
}

export function progressLabel({ left, total }: Remaining, steps: number): string {
  if (total > 0) return left > 0 ? `${left} of ${total} left` : 'All answered';
  return steps > 0 ? 'Steps' : 'Earlier rounds';
}

export function StepJump({ steps, current, progress, interactions, packInteractive, history }: StepJumpProps) {
  const { jumpTo } = useKeyboardApi();
  const { reveal } = useActiveBlock();
  const [open, setOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current!;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  }, [open]);

  const revealedRef = useRef(reveal?.epoch);
  useEffect(() => {
    if (!reveal || reveal.epoch === revealedRef.current) return;
    revealedRef.current = reveal.epoch;
    setHistoryOpen(true);
  }, [reveal]);

  const openHistory = () => {
    setOpen(false);
    setHistoryOpen(true);
  };

  return (
    <>
      <button
        type="button"
        className="step-jump"
        aria-haspopup="dialog"
        aria-expanded={open || historyOpen}
        onClick={() => (steps.length > 0 ? setOpen(true) : setHistoryOpen(true))}
      >
        {progressLabel(progress, steps.length)}
      </button>
      <dialog
        ref={ref}
        className="step-jump-dialog"
        aria-label="Jump to a step"
        onClose={() => setOpen(false)}
        onClick={(e) => {
          if (e.target === e.currentTarget) setOpen(false);
        }}
      >
        <ul className="focus-receipts">
          {steps.map((step, i) => (
            <StepReceipt
              key={step.id}
              step={step}
              interactions={interactions}
              packInteractive={packInteractive}
              current={i === current}
              onSelect={() => {
                setOpen(false);
                jumpTo(step.id);
              }}
            />
          ))}
        </ul>
        {history.length > 0 && (
          <button type="button" className="step-jump-history" onClick={openHistory}>
            Earlier rounds ({history.length})
          </button>
        )}
      </dialog>
      {history.length > 0 && (
        <HistoryDrawer
          open={historyOpen}
          onClose={() => setHistoryOpen(false)}
          history={history}
          interactions={interactions}
        />
      )}
    </>
  );
}
