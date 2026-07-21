import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import type { Interactions } from '../events';
import { revisionStore } from '../revision';
import type { RailRevisionState } from '../revision';

export interface StepDotsProps {
  steps: FocusStep[];
  index: number;
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  onJump: (id: string) => void;
}

const RAIL_MAX = 10;

const RAIL_ARIA: Record<RailRevisionState, string> = {
  revising: 'being revised',
  added: 'new step',
  changed: 'updated since you saw it',
};

export function StepDots({ steps, index, interactions, packInteractive, onJump }: StepDotsProps) {
  const total = steps.length;
  return (
    <div className={total <= RAIL_MAX ? 'focus-dots' : 'focus-strip'} role="group" aria-label="steps">
      {steps.map((step, i) => {
        const status = stepStatus(step, interactions, packInteractive);
        const rev = revisionStore.railState(step.id);
        const label = `Step ${i + 1}: ${stepTitle(step)}${status ? `, ${status}` : ''}${
          rev ? `, ${RAIL_ARIA[rev]}` : ''
        }`;
        const base = total <= RAIL_MAX ? 'focus-dot' : 'focus-strip-seg';
        return (
          <button
            key={step.id}
            type="button"
            className={`${base}${i === index ? ' current' : ''}${status ? ` ${status}` : ' tick'}${
              rev ? ` ${rev}` : ''
            }`}
            aria-label={label}
            aria-current={i === index || undefined}
            onClick={() => onJump(step.id)}
          />
        );
      })}
    </div>
  );
}
