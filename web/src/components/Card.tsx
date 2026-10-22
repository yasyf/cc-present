import { useContext } from 'react';
import type { Card as CardBlock, CardStatus } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { renderInlineMarkdown } from '../markdown';
import { BlockRenderer } from './BlockRenderer';
import { FocusStepContext } from './focusStep';

export function Card({ block, interactions }: { block: CardBlock; interactions: Interactions }) {
  // In focus mode FocusCard hoists title/status/chips into the step meta row, so
  // the in-body head would duplicate them; board mode (null context) keeps it.
  const focus = useContext(FocusStepContext);
  return (
    <div className={`card${block.flagged ? ' flagged' : ''}`}>
      {!focus && (
        <div className="card-head">
          {block.title && <span className="card-title">{block.title}</span>}
          {block.status && <StatusPill status={block.status} />}
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
      )}
      {!focus && block.summary && (
        <div
          className="card-summary"
          dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(block.summary) }}
        />
      )}
      {block.flagged && <div className="flag-callout">Flagged for review</div>}
      <div className="card-body">
        {block.children
          .filter((child) => !focus?.hoisted.has(child.id))
          .map((child) => (
            <BlockRenderer key={child.id} block={child} interactions={interactions} />
          ))}
      </div>
    </div>
  );
}

export function StatusPill({ status }: { status: CardStatus }) {
  const { closed } = usePresent();
  const shown = closed && status === 'open' ? 'closed' : status;
  return <span className={`status status-${shown}`}>{shown}</span>;
}
