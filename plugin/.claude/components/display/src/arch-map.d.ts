import type { Badge, BlockKind, Medium, PartShape, Status, SystemMap, View } from './system-map.js';

export interface ArchNode {
  id: string;
  kind: BlockKind;
  name: string;
  role: string;
  about?: string;
  claim?: string;
  shape?: PartShape;
  medium?: Medium;
  tag?: string;
  holds?: { text: string; note?: string; sealed?: boolean }[];
  status?: Status;
  group?: string;
  lane?: 'main' | 'background' | 'control';
  critical?: boolean;
  badges?: Badge[];
  nodes?: ArchNode[];
}

export interface ArchGroup {
  id: string;
  label: string;
  kind?: 'zone' | 'machine' | 'process';
  in?: string;
}

export interface ArchEdge {
  from: string;
  to: string;
  kind?: 'sync' | 'async' | 'copy' | 'control';
  label?: string;
}

export interface ArchStep {
  from: string;
  to: string;
  say: string;
  waitsFor?: number;
  mark?: 'risk' | 'stop';
}

export type ArchNote = { on: string; text: string } | { edge: { from: string; to: string }; text: string };

export interface ArchDoc {
  $schema?: string;
  arch: 1;
  name: string;
  title: string;
  summary?: string;
  nodes: ArchNode[];
  groups?: ArchGroup[];
  edges?: ArchEdge[];
  path?: { title: string; steps: ArchStep[] };
  stories?: { id: string; label: string; title?: string; tone?: 'ok' | 'fail' | 'pending'; steps: ArchStep[] }[];
  notes?: ArchNote[];
  lanes?: { main?: string; background?: string; control?: string };
  start?: { node?: string; story?: string };
}

export interface ArchView extends View {
  focus?: string;
}

export interface ArchHandle {
  readonly problems: string[];
  readonly ready: Promise<void>;
  update(doc: ArchDoc): void;
  show(view: ArchView): void;
  destroy(): void;
}

export function validate(doc: unknown): string[];
export function project(doc: ArchDoc, focus: string | null): SystemMap;
export function toMarkdown(doc: ArchDoc): string;
export function mount(host: HTMLElement, doc: ArchDoc, opts?: { state?: ArchView; onState?: (v: ArchView) => void; key?: boolean; fill?: boolean }): ArchHandle;
