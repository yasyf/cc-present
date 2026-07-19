import { useEffect, useRef, useState } from 'react';
import type { FocusStep } from '../focus';
import { stepStatus, stepTitle } from '../focus';
import { revisionStore, useDrafting } from '../revision';
import type { RailRevisionState } from '../revision';
import type { Interactions } from '../events';

export interface FocusProgressProps {
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

// FocusProgress is the deck header: the mono step counter, the tier label, an
// optional doc-level drafting one-liner, and a tap-to-jump dot rail. Dots fill with
// the verdict color, layer revision state, render untallied steps as muted ticks,
// and collapse to a segmented bar past RAIL_MAX.
export function FocusProgress({ steps, index, interactions, packInteractive, onJump }: FocusProgressProps) {
  const total = steps.length;
  const onSummary = index >= total;
  const shown = Math.min(index + 1, total);
  const tier = index < total ? steps[index]!.tier : undefined;

  // useDrafting also subscribes this header to the revision store, so the imperative
  // railState reads below re-render on any mark change without a per-dot hook.
  const drafting = useDrafting();
  const grewToRef = useRef(total);
  const [growth, setGrowth] = useState('');
  useEffect(() => {
    if (total > grewToRef.current) setGrowth(`Deck grew to ${total} steps`);
    grewToRef.current = total;
  }, [total]);

  return (
    <div className="focus-progress">
      <div className="focus-progress-head">
        <span className="focus-step-count">{onSummary ? 'Review' : `Step ${shown} / ${total}`}</span>
        {tier && <span className="focus-step-tier">{tier}</span>}
      </div>
      {drafting && (
        <p className={`focus-drafting${drafting.passive ? ' passive' : ''}`}>
          Claude is drafting{drafting.note ? ` — ${drafting.note}` : ''}
        </p>
      )}
      <span className="focus-deck-live sr-only" aria-live="polite" aria-atomic="true">
        {growth}
      </span>
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
    </div>
  );
}
