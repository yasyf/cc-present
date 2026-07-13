import type { Block } from '../schema';
import type { Interactions } from '../events';
import { blockDecided } from '../decide';
import { BlockRenderer } from './BlockRenderer';

// BoardBlocks renders the current round's live blocks. A section renders as a
// bare flex child of .blocks (no .block-row/data-flip-key) so the stylesheet can
// pin it as a sticky tier header, outside the FLIP set. Every other block is a
// flip-tracked row: one whose every decidable is decided carries data-decided so
// the stylesheet can recede it to a receipt; a row with nothing to decide never
// gets the attribute.
export function BoardBlocks({
  blocks,
  interactions,
  packInteractive,
}: {
  blocks: Block[];
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
}) {
  return (
    <>
      {blocks.map((block) =>
        block.type === 'section' ? (
          <BlockRenderer key={block.id} block={block} interactions={interactions} />
        ) : (
          <div
            className="block-row"
            key={block.id}
            data-flip-key={block.id}
            data-decided={blockDecided(block, interactions, packInteractive) || undefined}
          >
            <BlockRenderer block={block} interactions={interactions} />
          </div>
        ),
      )}
    </>
  );
}
