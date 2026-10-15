import { useEffect, useRef, useState } from 'react';
import type { FocusStep } from '../focus';
import { useDrafting } from '../revision';

export interface FocusProgressProps {
  steps: FocusStep[];
  index: number;
}

// FocusProgress is the deck header: the tier label, an optional drafting line, and a
// polite live region announcing deck growth. The step counter lives in DocHeader.
export function FocusProgress({ steps, index }: FocusProgressProps) {
  const total = steps.length;
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
      {tier && <span className="focus-step-tier">{tier}</span>}
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
