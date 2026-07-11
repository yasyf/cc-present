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
import { ImageView } from './ImageView';
import { TableView } from './TableView';
import { ProgressView } from './ProgressView';
import { PackBlockView } from './PackBlockView';

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
  image: ImageView,
  table: TableView,
  progress: ProgressView,
};

// BlockRenderer dispatches on the block's type. Pack blocks route to the pack
// registry; every built-in resolves through the exhaustive BUILTIN table.
export function BlockRenderer({ block, interactions }: BlockProps) {
  if (isPackBlock(block)) return <PackBlockView block={block} interactions={interactions} />;
  const Renderer = BUILTIN[block.type] as ComponentType<BlockProps>;
  return <Renderer block={block} interactions={interactions} />;
}
