import type { Block } from '../schema';
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

export interface BlockProps {
  block: Block;
  interactions: Interactions;
}

// BlockRenderer dispatches on the block's type. The default arm is never-checked,
// so a new block type is a compile error until it is handled here.
export function BlockRenderer({ block, interactions }: BlockProps) {
  switch (block.type) {
    case 'section':
      return <Section block={block} />;
    case 'card':
      return <Card block={block} interactions={interactions} />;
    case 'approval':
      return <Approval block={block} interactions={interactions} />;
    case 'choice':
      return <Choice block={block} interactions={interactions} />;
    case 'input':
      return <Input block={block} interactions={interactions} />;
    case 'markdown':
      return <Markdown block={block} />;
    case 'code':
      return <Code block={block} />;
    case 'diff':
      return <DiffView block={block} />;
    case 'image':
      return <ImageView block={block} />;
    case 'table':
      return <TableView block={block} />;
    case 'progress':
      return <ProgressView block={block} />;
    default:
      return block satisfies never;
  }
}
