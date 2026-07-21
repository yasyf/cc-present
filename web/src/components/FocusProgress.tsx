import { useEffect, useRef, useState } from 'react';
import type { FocusStep } from '../focus';
import { useDrafting } from '../revision';

export interface FocusProgressProps {
  steps: FocusStep[];
  index: number;
}

// FocusProgress is the deck header: the mono step counter, the tier label, an
// optional doc-level drafting one-liner, and a polite live region that announces
// deck growth. The tap-to-jump step-dot rail lives in WizardFooter.
export function FocusProgress({ steps, index }: FocusProgressProps) {
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
    </div>
  );
}
