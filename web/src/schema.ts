// The canonical cc-present document schema. The Go structs in internal/doc
// mirror these declarations with the same camelCase JSON field names. The
// document carries only agent-owned display state (card status, progress);
// human verdicts live in the separate interaction reduction (see events.ts).

export type BuiltinBlockType =
  | 'section'
  | 'card'
  | 'approval'
  | 'choice'
  | 'input'
  | 'markdown'
  | 'code'
  | 'diff'
  | 'diagram'
  | 'image'
  | 'table'
  | 'progress'
  | 'chart'
  | 'term'
  | 'filetree'
  | 'record';

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
  // Drill-down tradeoffs and rationale behind a "Details" affordance.
  detail?: Detail;
}

export type FactTone = 'default' | 'good' | 'warn' | 'bad';

// A scannable key/value in an option's up-front cluster; tone tints the value.
export interface Fact {
  label?: string;
  value: string;
  tone?: FactTone;
}

export type DetailMode = 'inline' | 'modal';

// An expandable drill-down. mode picks the surface: inline (default) expands in
// place and joins expand-all; modal opens an overlay.
export interface Detail {
  pros?: string[];
  cons?: string[];
  md?: string;
  mode?: DetailMode;
}

// A restricted leaf an option may carry, rendered in the option's visual stage
// rather than inside the row.
export type OptionVisual = Code | Diagram | Image | Diff | Chart | Term | FileTree | Record;

export interface ChoiceOption {
  id: string;
  label: string;
  // Single-line inline markdown shown beside the option label.
  hint?: string;
  md?: string;
  // Scannable metrics shown in the option's up-front cluster.
  facts?: Fact[];
  // Drill-down tradeoffs and rationale behind a "Details" affordance.
  detail?: Detail;
  // The author's suggested pick; at most one per single-select choice.
  recommended?: boolean;
  // A single visual leaf rendered in the option's visual stage.
  visual?: OptionVisual;
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

// A text-to-diagram block rendered client-side; kind is `mermaid`.
export interface Diagram {
  id: string;
  type: 'diagram';
  kind: 'mermaid';
  source: string;
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
  // An inline index signature rather than `Record<…>`: the `Record` block type
  // declared below shadows the `Record` utility type within this module.
  rows: { [key: string]: string }[];
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

export interface ChartSeries {
  label: string;
  // Values align index-for-index to the chart's categories.
  values: number[];
}

// A categorical bar or line chart rendered client-side into themed SVG.
export interface Chart {
  id: string;
  type: 'chart';
  kind: 'bar' | 'line';
  title?: string;
  unit?: string;
  categories: string[];
  series: ChartSeries[];
}

// A terminal panel: an optional command line above its captured (ANSI) output.
export interface Term {
  id: string;
  type: 'term';
  command?: string;
  output: string;
  title?: string;
}

export type TreeBadge = 'added' | 'modified' | 'removed';

// One file path in a FileTree; directories are implicit from the path segments.
export interface FileTreeEntry {
  path: string;
  badge?: TreeBadge;
  note?: string;
}

export interface FileTree {
  id: string;
  type: 'filetree';
  title?: string;
  entries: FileTreeEntry[];
}

// A labelled https link in a Record.
export interface RecordLink {
  label: string;
  url: string;
}

// One entity's labelled profile: tone chips, keyed facts, and https links.
export interface Record {
  id: string;
  type: 'record';
  title?: string;
  chips?: Chip[];
  facts: Fact[];
  links?: RecordLink[];
}

// --- Pack blocks (plugin-supplied leaves) ---

// A pack block's wire type is two dot-separated segments, `<pack>.<name>`.
// Built-in types never contain a dot, so a dotted type is unambiguously a pack
// block and narrowing on `type` stays sound across the whole Block union.
export type PackBlockType = `${string}.${string}`;

// A plugin-supplied leaf block. Its body is opaque to the host: the id and the
// dotted type are known, every other field is preserved verbatim and rendered by
// the pack's own bundle. It nests as a card child like any built-in leaf.
export interface PackBlock {
  id: string;
  type: PackBlockType;
  [key: string]: unknown;
}

// --- Unions ---

export type StructuralBlock = Section | Card;
export type InteractiveBlock = Approval | Choice | Input;
export type ContentBlock =
  | Markdown
  | Code
  | Diff
  | Diagram
  | Image
  | Table
  | Progress
  | Chart
  | Term
  | FileTree
  | Record;

// A card nests exactly one level of these leaf blocks; it cannot nest a section
// or another card. Pack blocks are leaves and join the child set.
export type ChildBlock = InteractiveBlock | ContentBlock | PackBlock;

// A block. A section, a card, any built-in leaf, or a pack block may appear
// directly in Doc.blocks; a card nests leaf blocks only. The document body and
// every block.upserted patch carry one of these.
export type Block = StructuralBlock | ChildBlock;

// isPackBlock narrows a block to the opaque pack leaf: a dotted type is a pack
// block, a dot-free built-in type never is.
export function isPackBlock(block: Block): block is PackBlock {
  return block.type.includes('.');
}

// packNameOf is the pack namespace of a pack block type: the segment before the
// first dot.
export function packNameOf(type: PackBlockType): string {
  return type.slice(0, type.indexOf('.'));
}

export interface Doc {
  version: 1;
  title: string;
  intro?: string;
  stats?: Stat[];
  submit?: Submit;
  // Per-push hint for the client's default view; the viewer's own toggle overrides it.
  presentation?: 'focus' | 'board';
  blocks: Block[];
}
