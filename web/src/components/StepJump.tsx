import { useEffect, useRef, useState } from 'react';
import type { FocusStep } from '../focus';
import type { Interactions } from '../events';
import { useKeyboardApi } from '../keyboard';
import { StepReceipt } from './FocusSummary';

export interface StepJumpProps {
  steps: FocusStep[];
  index: number;
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
}

export function StepJump({ steps, index, interactions, packInteractive }: StepJumpProps) {
  const { jumpTo } = useKeyboardApi();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current!;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  }, [open]);

  const total = steps.length;
  return (
    <>
      <button
        type="button"
        className="step-jump"
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
      >
        {index >= total ? 'Review' : `Step ${index + 1} / ${total}`}
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
              current={i === index}
              onSelect={() => {
                setOpen(false);
                jumpTo(step.id);
              }}
            />
          ))}
        </ul>
      </dialog>
    </>
  );
}
