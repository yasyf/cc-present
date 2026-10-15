import { Fragment, useRef } from 'react';
import type { Block, Section as SectionBlock } from '../schema';
import type { Interactions } from '../events';
import { blockDecided } from '../decide';
import { useViewedOnScreen } from '../viewed';
import { BlockRenderer } from './BlockRenderer';

export interface SectionGroup {
  section: SectionBlock | null;
  blocks: Block[];
}

export function sectionGroups(blocks: Block[]): SectionGroup[] {
  const groups: SectionGroup[] = [{ section: null, blocks: [] }];
  for (const block of blocks) {
    if (block.type === 'section') groups.push({ section: block, blocks: [] });
    else groups.at(-1)!.blocks.push(block);
  }
  return groups;
}

// BoardBlocks renders the live blocks; a section and its rows share one .section-group
// so the sticky head releases at the group's end.
export function BoardBlocks({
  blocks,
  interactions,
  packInteractive,
}: {
  blocks: Block[];
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
}) {
  const row = (block: Block) => (
    <BoardRow
      key={block.id}
      block={block}
      interactions={interactions}
      decided={blockDecided(block, interactions, packInteractive)}
    />
  );
  return (
    <>
      {sectionGroups(blocks).map(({ section, blocks: rows }) =>
        section ? (
          <section key={section.id} className="section-group">
            <BlockRenderer block={section} interactions={interactions} />
            {rows.map(row)}
          </section>
        ) : (
          <Fragment key="">{rows.map(row)}</Fragment>
        ),
      )}
    </>
  );
}

function BoardRow({ block, interactions, decided }: { block: Block; interactions: Interactions; decided: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  useViewedOnScreen(ref, block);
  return (
    <div
      ref={ref}
      className="block-row"
      tabIndex={-1}
      data-block-id={block.id}
      data-flip-key={block.id}
      data-decided={decided || undefined}
    >
      <BlockRenderer block={block} interactions={interactions} note />
    </div>
  );
}
