export type Medium = 'memory' | 'disk' | 'bucket' | 'keys' | 'pages';
export type Status = 'live' | 'planned' | 'retired';
export type BlockKind = 'actor' | 'app' | 'service' | 'worker' | 'store' | 'log' | 'external' | 'artifact' | 'doc';
export type PartShape =
  | 'api'
  | 'relay'
  | 'gate'
  | 'router'
  | 'guard'
  | 'queue'
  | 'pool'
  | 'log'
  | 'recovery'
  | 'handoff'
  | 'watcher'
  | 'keys'
  | 'budget'
  | 'timer'
  | 'uploader'
  | 'codec'
  | 'compactor'
  | 'sweeper'
  | 'cache'
  | 'ui'
  | 'tool'
  | 'docs'
  | 'sdk'
  | 'module'
  | 'store';

export interface Badge {
  tone: 'ok' | 'fail' | 'warn' | 'info' | 'ask';
  text: string;
}

export interface Group {
  id: string;
  label: string;
  kind?: 'zone' | 'machine' | 'process';
  in?: string;
}

export interface Part {
  id: string;
  shape: PartShape;
  name: string;
  role: string;
  medium?: Medium;
  tag?: string;
  about?: string;
  status?: Status;
  badges?: Badge[];
}

export interface Block {
  id: string;
  kind: BlockKind;
  name: string;
  role: string;
  group?: string;
  medium?: Medium;
  tag?: string;
  parts?: Part[];
  contents?: { text: string; note?: string; sealed?: boolean }[];
  about?: string;
  critical?: boolean;
  status?: Status;
  badges?: Badge[];
  lane?: 'main' | 'background' | 'control';
}

export interface Link {
  from: string;
  to: string;
  label?: string;
  style?: 'sync' | 'async' | 'copy' | 'control';
  hidden?: boolean;
}

export interface Step {
  from: string;
  to: string;
  say: string;
  waitsFor?: number;
  mark?: 'risk' | 'stop';
}

export interface Story {
  id: string;
  label: string;
  title?: string;
  tone?: 'ok' | 'fail' | 'pending';
  steps: Step[];
}

export interface SystemMap {
  title: string;
  blocks: Block[];
  groups?: Group[];
  links?: Link[];
  path?: { title: string; steps: Step[] };
  stories?: Story[];
  notes?: { on: string; text: string }[];
  lanes?: { main?: string; background?: string; control?: string };
  start?: { story: string } | { block: string };
}

export interface View {
  story?: string;
  block?: string;
  part?: string;
}

export interface Problem {
  view: string;
  rule: string;
  text: string;
  with?: string;
}

export interface Rough {
  svg(svg: SVGSVGElement): unknown;
}

export interface MapHandle {
  readonly problems: string[];
  ready: Promise<void>;
  update(map: SystemMap): void;
  show(view: View): void;
  reset(): Promise<void>;
  audit(opts?: { all?: boolean }): Promise<Problem[]>;
  destroy(): void;
}

export function mount(host: HTMLElement, map: SystemMap, opts: { rough: Rough; font?: boolean; state?: View; onState?: (v: View) => void }): MapHandle;
export function validate(map: unknown): string[];
