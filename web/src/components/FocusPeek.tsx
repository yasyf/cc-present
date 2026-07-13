import type { FocusStep } from '../focus';
import { stepTitle } from '../focus';

// FocusPeek is the next card showing behind the current one — a facade of chrome
// and title only. It must never mount BlockRenderer: a real next card would
// register its decidables in the ring and double-instantiate pack bundles.
export function FocusPeek({ step }: { step: FocusStep }) {
  return (
    <div className="focus-peek" aria-hidden>
      {step.tier && <div className="focus-tier">{step.tier}</div>}
      <div className="focus-peek-title">{stepTitle(step)}</div>
    </div>
  );
}
