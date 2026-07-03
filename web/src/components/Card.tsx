import type { Card as CardBlock } from '../schema';
import type { Interactions } from '../events';
import { BlockRenderer } from './BlockRenderer';

export function Card({ block, interactions }: { block: CardBlock; interactions: Interactions }) {
  return (
    <div className={`card${block.flagged ? ' flagged' : ''}`}>
      <div className="card-head">
        {block.title && <span className="card-title">{block.title}</span>}
        {block.status && <span className={`status status-${block.status}`}>{block.status}</span>}
        {block.chips && block.chips.length > 0 && (
          <span className="chips">
            {block.chips.map((chip, i) => (
              <span key={i} className={`chip chip-${chip.tone ?? 'default'}`}>
                {chip.label}
              </span>
            ))}
          </span>
        )}
      </div>
      {block.flagged && <div className="flag-callout">Flagged for review</div>}
      <div className="card-body">
        {block.children.map((child) => (
          <BlockRenderer key={child.id} block={child} interactions={interactions} />
        ))}
      </div>
    </div>
  );
}
