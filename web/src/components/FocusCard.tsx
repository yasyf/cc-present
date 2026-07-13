import type { FocusStep } from '../focus';
import type { Interactions } from '../events';
import { BlockRenderer } from './BlockRenderer';

// FocusCard renders the live step body: its lead-in context then the focal block,
// each through BlockRenderer so the decidables register with the keyboard. It
// never wraps a block in .block-row/data-flip-key — FLIP is board-only.
export function FocusCard({ step, interactions }: { step: FocusStep; interactions: Interactions }) {
  return (
    <div className="focus-card" tabIndex={-1}>
      {step.tier && <div className="focus-tier">{step.tier}</div>}
      <div className="focus-card-body">
        {step.context.map((block) => (
          <BlockRenderer key={block.id} block={block} interactions={interactions} />
        ))}
        <BlockRenderer key={step.block.id} block={step.block} interactions={interactions} />
      </div>
    </div>
  );
}
