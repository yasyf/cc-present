import { useContext } from 'react';
import type { ComponentType } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import { isPackBlock, type Block, type BuiltinBlockType } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { useInteractivePackTypes } from '../packs/registry';
import { ownsThread, threadTurns } from '../threadFeed';
import { Section } from './Section';
import { Card } from './Card';
import { Approval } from './Approval';
import { Choice } from './Choice';
import { Input } from './Input';
import { DraftView } from './DraftView';
import { Triage } from './Triage';
import { Markdown } from './Markdown';
import { Code } from './Code';
import { DiffView } from './DiffView';
import { DiagramView } from './DiagramView';
import { ImageView } from './ImageView';
import { TableView } from './TableView';
import { ProgressView } from './ProgressView';
import { ChartView } from './ChartView';
import { TermView } from './TermView';
import { FileTreeView } from './FileTreeView';
import { RecordView } from './RecordView';
import { PRView } from './PRView';
import { PRGroups } from './PRGroups';
import { PackBlockView } from './PackBlockView';
import { BlockThread } from './BlockThread';
import { ReplyThread } from './ReplyThread';
import { FocusStepContext } from './focusStep';
import { useThreadHost } from './threadHost';

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
  draft: DraftView,
  triage: Triage,
  markdown: Markdown,
  code: Code,
  diff: DiffView,
  diagram: DiagramView,
  image: ImageView,
  table: TableView,
  progress: ProgressView,
  chart: ChartView,
  term: TermView,
  filetree: FileTreeView,
  record: RecordView,
  pr: PRView,
  'pr-groups': PRGroups,
};

// BlockRenderer dispatches on the block's type, then threads the agent's replies
// beneath it. A block that owns its thread renders its own replies beside the
// note channel, so it is returned bare; every other block gets the shared
// ReplyThread, after the note channel when `note` asks for one. Under the rail,
// whose composer holds the notes, each reply carries the note it answers.
export function BlockRenderer({ block, interactions, note }: BlockProps & { note?: boolean }) {
  const packInteractive = useInteractivePackTypes();
  const rail = useThreadHost() === 'rail';
  const inner = <BlockBody block={block} interactions={interactions} />;
  if (ownsThread(block, packInteractive)) return inner;
  return (
    <>
      {inner}
      {note && <BlockNote block={block} interactions={interactions} packInteractive={packInteractive} />}
      <ReplyThread
        replies={interactions.replies[block.id] ?? []}
        notes={rail ? Object.values(interactions.feedback).flat() : []}
      />
    </>
  );
}

function BlockNote({ block, interactions, packInteractive }: BlockProps & { packInteractive: ReadonlySet<string> }) {
  const { closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const rail = useThreadHost() === 'rail';
  const step = useContext(FocusStepContext) !== null;
  if (block.type === 'section') return null;
  const turns = threadTurns(block, interactions.feedback, interactions.replies, packInteractive);
  return (
    <BlockThread
      blockId={block.id}
      type={block.type}
      feedback={turns.feedback}
      replies={rail ? turns.replies : []}
      locked={closed || readOnly}
      step={step}
    />
  );
}

// BlockBody dispatches a block to its renderer without the shared ReplyThread — the
// focus-mode visual stage and option drill-downs render a block bare.
export function BlockBody({ block, interactions }: BlockProps) {
  if (isPackBlock(block)) return <PackBlockView block={block} interactions={interactions} />;
  const Renderer = BUILTIN[block.type] as ComponentType<BlockProps>;
  return <Renderer block={block} interactions={interactions} />;
}
