import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import type { Interactions } from '../events';

export interface FocusProgressProps {
  steps: FocusStep[];
  index: number;
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  onJump: (id: string) => void;
}

const RAIL_MAX = 10;

// FocusProgress is the deck header: the mono step counter, the tier label, and a
// tap-to-jump dot rail that fills decided dots with the verdict color, collapsing
// to a segmented bar past RAIL_MAX steps.
export function FocusProgress({ steps, index, interactions, packInteractive, onJump }: FocusProgressProps) {
  const total = steps.length;
  const shown = Math.min(index + 1, total);
  const tier = index < total ? steps[index]!.tier : undefined;

  return (
    <div className="focus-progress">
      <div className="focus-progress-head">
        <span className="focus-step-count">
          Step {shown} / {total}
        </span>
        {tier && <span className="focus-step-tier">{tier}</span>}
      </div>
      {total <= RAIL_MAX ? (
        <div className="focus-dots" role="group" aria-label="steps">
          {steps.map((step, i) => {
            const status = stepStatus(step, interactions, packInteractive);
            const label = `Step ${i + 1}: ${stepTitle(step)}${status ? `, ${status}` : ''}`;
            return (
              <button
                key={step.id}
                type="button"
                className={`focus-dot${i === index ? ' current' : ''}${status ? ` ${status}` : ''}`}
                aria-label={label}
                aria-current={i === index || undefined}
                onClick={() => onJump(step.id)}
              />
            );
          })}
        </div>
      ) : (
        <div className="focus-bar" role="progressbar" aria-valuenow={shown} aria-valuemin={0} aria-valuemax={total}>
          <div className="focus-bar-fill" style={{ width: `${(shown / total) * 100}%` }} />
        </div>
      )}
    </div>
  );
}
