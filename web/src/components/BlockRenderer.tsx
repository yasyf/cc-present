import type { ComponentType } from 'react';
import { isPackBlock, type Block, type BuiltinBlockType } from '../schema';
import type { Interactions } from '../events';
import { Section } from './Section';
import { Card } from './Card';
import { Approval } from './Approval';
import { Choice } from './Choice';
import { Input } from './Input';
import { Markdown } from './Markdown';
import { Code } from './Code';
import { DiffView } from './DiffView';
import { DiagramView } from './DiagramView';
import { ImageView } from './ImageView';
import { TableView } from './TableView';
import { ProgressView } from './ProgressView';
import { PackBlockView } from './PackBlockView';
import { ReplyThread } from './ReplyThread';

export interface BlockProps {
  block: Block;
  interactions: Interactions;
}

type BuiltinRenderer<T extends BuiltinBlockType> = ComponentType<{
  block: Extract<Block, { type: T }>;
  interactions: Interactions;
}>;

// The built-in dispatch table. The mapped type is exhaustive: a new built-in
// block type is a compile error until it is added here, and an unknown key is
// rejected. Components that ignore `interactions` still satisfy the entry.
const BUILTIN: { [T in BuiltinBlockType]: BuiltinRenderer<T> } = {
  section: Section,
  card: Card,
  approval: Approval,
  choice: Choice,
  input: Input,
  markdown: Markdown,
  code: Code,
  diff: DiffView,
  diagram: DiagramView,
  image: ImageView,
  table: TableView,
  progress: ProgressView,
};

// BlockRenderer dispatches on the block's type, then threads the agent's replies
// beneath it. Approval renders its replies inline, so it is returned bare; every
// other block — built-in or pack — gets the shared ReplyThread.
export function BlockRenderer({ block, interactions }: BlockProps) {
  const inner = renderInner(block, interactions);
  if (block.type === 'approval') return inner;
  return (
    <>
      {inner}
      <ReplyThread replies={interactions.replies[block.id] ?? []} />
    </>
  );
}

function renderInner(block: Block, interactions: Interactions) {
  if (isPackBlock(block)) return <PackBlockView block={block} interactions={interactions} />;
  const Renderer = BUILTIN[block.type] as ComponentType<BlockProps>;
  return <Renderer block={block} interactions={interactions} />;
}
