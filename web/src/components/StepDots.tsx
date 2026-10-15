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
  const statuses = steps.map((step) => stepStatus(step, interactions, packInteractive));
  const stateClass = (i: number) => {
    const rev = revisionStore.railState(steps[i]!.id);
    return `${i === index ? ' current' : ''}${statuses[i] ? ` ${statuses[i]}` : ' tick'}${rev ? ` ${rev}` : ''}`;
  };
  if (total > RAIL_MAX) {
    const decided = statuses.filter((s) => s !== null && s !== 'undecided').length;
    return (
      <div
        className="focus-strip"
        role="img"
        aria-label={`Step ${Math.min(index + 1, total)} of ${total}, ${decided} decided`}
      >
        {steps.map((step, i) => (
          <span key={step.id} className={`focus-strip-seg${stateClass(i)}`} />
        ))}
      </div>
    );
  }
  return (
    <div className="focus-dots" role="group" aria-label="steps">
      {steps.map((step, i) => {
        const status = statuses[i];
        const rev = revisionStore.railState(step.id);
        const label = `Step ${i + 1}: ${stepTitle(step)}${status ? `, ${status}` : ''}${
          rev ? `, ${RAIL_ARIA[rev]}` : ''
        }`;
        return (
          <button
            key={step.id}
            type="button"
            className={`focus-dot${stateClass(i)}`}
            aria-label={label}
            aria-current={i === index || undefined}
            onClick={() => onJump(step.id)}
          />
        );
      })}
    </div>
  );
}
