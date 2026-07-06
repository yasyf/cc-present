// The canonical cc-present document schema. The Go structs in internal/doc
// mirror these declarations with the same camelCase JSON field names. The
// document carries only agent-owned display state (card status, progress);
// human verdicts live in the separate interaction reduction (see events.ts).

export type BlockType =
  | 'section'
  | 'card'
  | 'approval'
  | 'choice'
  | 'input'
  | 'markdown'
  | 'code'
  | 'diff'
  | 'image'
  | 'table'
  | 'progress';

export interface Stat {
  label: string;
  value: string;
}

export interface Submit {
  label: string;
  note?: string;
}

// --- Structural blocks (top level) ---

export interface Section {
  id: string;
  type: 'section';
  title: string;
  md?: string;
}

export type ChipTone = 'default' | 'flag' | 'demo';

export interface Chip {
  label: string;
  tone?: ChipTone;
}

export type CardStatus = 'open' | 'resolved' | 'redrafted';

export interface Card {
  id: string;
  type: 'card';
  title?: string;
  // Single-line inline markdown shown under the card title.
  summary?: string;
  chips?: Chip[];
  flagged?: boolean;
  status?: CardStatus;
  children: ChildBlock[];
}

// --- Interactive child blocks ---

export interface Approval {
  id: string;
  type: 'approval';
  prompt?: string;
  // Defaults to true when omitted; the default is applied at render time.
  allowFeedback?: boolean;
}

export interface ChoiceOption {
  id: string;
  label: string;
  // Single-line inline markdown shown beside the option label.
  hint?: string;
  md?: string;
}

export interface Choice {
  id: string;
  type: 'choice';
  prompt?: string;
  multi?: boolean;
  options: ChoiceOption[];
}

export interface Input {
  id: string;
  type: 'input';
  label: string;
  placeholder?: string;
  multiline?: boolean;
}

// --- Content child blocks ---

export interface Markdown {
  id: string;
  type: 'markdown';
  md: string;
  struck?: boolean;
}

export interface Code {
  id: string;
  type: 'code';
  lang: string;
  code: string;
  title?: string;
}

export interface Diff {
  id: string;
  type: 'diff';
  diff: string;
  title?: string;
}

// An https: URL, an asset:<sha256> content-addressed reference, or a data: URI
// (at most 32 KiB).
export type ImageSrc = string;

export interface Image {
  id: string;
  type: 'image';
  src: ImageSrc;
  alt: string;
  caption?: string;
}

export type ColumnAlign = 'left' | 'right';

export interface Column {
  key: string;
  label: string;
  align?: ColumnAlign;
}

export interface Table {
  id: string;
  type: 'table';
  columns: Column[];
  rows: Record<string, string>[];
}

export type ProgressState = 'active' | 'done' | 'error';

export interface Progress {
  id: string;
  type: 'progress';
  label: string;
  value: number;
  max: number;
  state?: ProgressState;
}

// --- Unions ---

export type StructuralBlock = Section | Card;
export type InteractiveBlock = Approval | Choice | Input;
export type ContentBlock = Markdown | Code | Diff | Image | Table | Progress;

// A card nests exactly one level of these leaf blocks; it cannot nest a section
// or another card.
export type ChildBlock = InteractiveBlock | ContentBlock;

// A block. A section, a card, or any of the nine leaf blocks may appear directly
// in Doc.blocks; a card nests leaf blocks only. The document body and every
// block.upserted patch carry one of these.
export type Block = StructuralBlock | ChildBlock;

export interface Doc {
  version: 1;
  title: string;
  intro?: string;
  stats?: Stat[];
  submit?: Submit;
  blocks: Block[];
}
