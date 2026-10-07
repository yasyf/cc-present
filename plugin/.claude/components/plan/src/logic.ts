export type CallMark = '+' | '-' | '~' | '?' | ' ';

export interface CallRow {
  id?: string;
  call: string;
  mark: CallMark;
  new?: boolean;
  at?: string;
  note?: string;
  calls?: CallRow[];
}

export interface FlatCall {
  key: string;
  row: CallRow;
  depth: number;
  glyph: string;
  root: number;
  parent: string | null;
}

export function flattenCalls(roots: CallRow[]): FlatCall[] {
  const out: FlatCall[] = [];
  const walk = (rows: CallRow[], path: string, depth: number, guides: string, root: number, parent: string | null) => {
    rows.forEach((row, i) => {
      const last = i === rows.length - 1;
      const key = row.id ?? (path ? `${path}.${i}` : `${i}`);
      const glyph = depth === 0 ? '' : guides + (last ? '└ ' : '├ ');
      out.push({ key, row, depth, glyph, root: depth === 0 ? i : root, parent });
      if (row.calls) {
        const nextGuides = depth === 0 ? '' : guides + (last ? '  ' : '│ ');
        walk(row.calls, path ? `${path}.${i}` : `${i}`, depth + 1, nextGuides, depth === 0 ? i : root, key);
      }
    });
  };
  walk(roots, '', 0, '', 0, null);
  return out;
}

export function callsProblems(roots: CallRow[]): string[] {
  const seen = new Set<string>();
  const problems: string[] = [];
  for (const { key } of flattenCalls(roots)) {
    if (seen.has(key)) problems.push(`duplicate row id "${key}"`);
    seen.add(key);
  }
  return problems;
}

export interface CallCounts {
  added: number;
  removed: number;
  changed: number;
  proposed: number;
}

export function callCounts(roots: CallRow[]): CallCounts {
  const counts: CallCounts = { added: 0, removed: 0, changed: 0, proposed: 0 };
  for (const { row } of flattenCalls(roots)) {
    if (row.mark === '+') counts.added++;
    else if (row.mark === '-') counts.removed++;
    else if (row.mark === '~') counts.changed++;
    else if (row.mark === '?') counts.proposed++;
  }
  return counts;
}

export function struckBy(flat: FlatCall[], struck: ReadonlySet<string>): Map<string, string> {
  const byKey = new Map(flat.map((f) => [f.key, f]));
  const out = new Map<string, string>();
  for (const f of flat) {
    for (let p = f.parent; p !== null; p = byKey.get(p)!.parent) {
      if (struck.has(p)) out.set(f.key, p);
    }
  }
  return out;
}

export function toggleStruck(struck: readonly string[], key: string): string[] {
  return struck.includes(key) ? struck.filter((k) => k !== key) : [...struck, key];
}

export type StateMark = '+' | '-' | '~';
export type TransitionMark = '+' | '-' | '~' | '?';

export interface MockSpec {
  html: string;
  css?: string;
  w?: number;
  h?: number;
  frame?: 'none' | 'browser' | 'phone' | 'desktop' | 'terminal';
  url?: string;
  title?: string;
}

export interface MachineState {
  id: string;
  label?: string;
  note?: string;
  final?: boolean;
  mark?: StateMark;
  screen?: { md: string } | { mock: MockSpec };
}

export interface Transition {
  from: string;
  event: string;
  to: string;
  label?: string;
  mark?: TransitionMark;
}

export interface MachineSpec {
  initial?: string;
  states: MachineState[];
  grid?: (string | null)[][];
  transitions: Transition[];
}

export function initialState(m: MachineSpec): string {
  return m.initial ?? m.states[0]!.id;
}

export function validateMachine(m: MachineSpec): string[] {
  const problems: string[] = [];
  const ids = new Set<string>();
  for (const s of m.states) {
    if (ids.has(s.id)) problems.push(`state "${s.id}" is declared twice`);
    ids.add(s.id);
  }
  const initial = initialState(m);
  if (!ids.has(initial)) problems.push(`initial state "${initial}" is not declared`);
  m.transitions.forEach((t, i) => {
    for (const end of [t.from, t.to]) {
      if (!ids.has(end)) problems.push(`transition ${i + 1} (${t.from} -${t.event}-> ${t.to}): unknown state "${end}"`);
    }
  });
  if (m.grid) {
    const placed = new Set<string>();
    for (const id of m.grid.flat()) {
      if (id === null) continue;
      if (!ids.has(id)) problems.push(`grid: unknown state "${id}"`);
      else if (placed.has(id)) problems.push(`grid: state "${id}" appears twice`);
      placed.add(id);
    }
    for (const s of m.states) if (!placed.has(s.id)) problems.push(`grid: state "${s.id}" has no cell`);
  }
  if (problems.length > 0) return problems;

  const live = m.transitions.filter((t) => t.mark !== '-');
  const reached = new Set([initial]);
  for (let grew = true; grew; ) {
    grew = false;
    for (const t of live) {
      if (reached.has(t.from) && !reached.has(t.to)) {
        reached.add(t.to);
        grew = true;
      }
    }
  }
  for (const s of m.states) {
    if (s.mark === '-') continue;
    if (!reached.has(s.id)) problems.push(`state "${s.id}" is unreachable from "${initial}"`);
    if (!s.final && !live.some((t) => t.from === s.id)) problems.push(`state "${s.id}" has no way out and is not final`);
  }
  return problems;
}

export interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
  cx: number;
  cy: number;
}

export type Point = [number, number];

export interface Route {
  index: number;
  pts: Point[];
  text: string;
  lx: number;
  ly: number;
  anchor: 'middle' | 'start' | 'end';
}

export interface MachineLayout {
  W: number;
  H: number;
  boxes: Record<string, Box>;
  routes: Route[];
}

const NODE_H = 36;
const GAP_Y = 64;
const PAD = 24;

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n));

export function stateLabel(s: MachineState): string {
  return s.label ?? s.id;
}

export function edgeText(t: Transition): string {
  return t.label ?? t.event;
}

function cells(m: MachineSpec): Map<string, [number, number]> {
  const out = new Map<string, [number, number]>();
  if (m.grid) {
    m.grid.forEach((row, r) => row.forEach((id, c) => id !== null && out.set(id, [c, r])));
    return out;
  }
  const initial = initialState(m);
  const depth = new Map([[initial, 0]]);
  const queue = [initial];
  while (queue.length > 0) {
    const u = queue.shift()!;
    for (const t of m.transitions) {
      if (t.from === u && !depth.has(t.to)) {
        depth.set(t.to, depth.get(u)! + 1);
        queue.push(t.to);
      }
    }
  }
  const rows: number[] = [];
  for (const s of m.states) {
    const c = depth.get(s.id) ?? 0;
    rows[c] = (rows[c] ?? -1) + 1;
    out.set(s.id, [c, rows[c]!]);
  }
  return out;
}

export function layoutMachine(m: MachineSpec, measure: (s: string) => number): MachineLayout {
  const nodeW = clamp(Math.ceil(Math.max(...m.states.map((s) => measure(stateLabel(s)))) + 34), 88, 200);
  const gapX = clamp(Math.ceil(Math.max(0, ...m.transitions.map((t) => measure(edgeText(t)))) + 40), 72, 220);
  const boxes: Record<string, Box> = {};
  let W = 0;
  let H = 0;
  for (const [id, [c, r]] of cells(m)) {
    const x = PAD + c * (nodeW + gapX);
    const y = PAD + r * (NODE_H + GAP_Y);
    boxes[id] = { x, y, w: nodeW, h: NODE_H, cx: x + nodeW / 2, cy: y + NODE_H / 2 };
    W = Math.max(W, x + nodeW + PAD);
    H = Math.max(H, y + NODE_H + PAD);
  }

  const blocked = (x: number, y: number, skip: Box[]) =>
    Object.values(boxes).some((b) => !skip.includes(b) && x > b.x - 4 && x < b.x + b.w + 4 && y > b.y - 4 && y < b.y + b.h + 4);
  const clear = (pts: Point[], skip: Box[]) =>
    pts.slice(1).every(([x2, y2], i) => {
      const [x1, y1] = pts[i]!;
      for (let k = 1; k < 24; k++) if (blocked(x1 + ((x2 - x1) * k) / 24, y1 + ((y2 - y1) * k) / 24, skip)) return false;
      return true;
    });

  const pairs = new Set(m.transitions.map((t) => `${t.from}\u0000${t.to}`));
  const routes = m.transitions.map((t, index): Route => {
    const a = boxes[t.from]!;
    const b = boxes[t.to]!;
    const text = edgeText(t);
    if (a === b) {
      const pts: Point[] = [
        [a.cx + 12, a.y],
        [a.cx + 12, a.y - 18],
        [a.cx - 12, a.y - 18],
        [a.cx - 12, a.y],
      ];
      return { index, pts, text, lx: a.cx, ly: a.y - 24, anchor: 'middle' };
    }
    const o = pairs.has(`${t.to}\u0000${t.from}`) ? (t.from < t.to ? -7 : 7) : 0;
    const dx = b.cx - a.cx;
    const dy = b.cy - a.cy;
    const skip = [a, b];
    let pts: Point[];
    if (Math.abs(dy) < 1) {
      const straight: Point[] = [
        [dx > 0 ? a.x + a.w : a.x, a.cy + o],
        [dx > 0 ? b.x : b.x + b.w, b.cy + o],
      ];
      const gy = a.y - GAP_Y / 2 + o;
      pts = clear(straight, skip)
        ? straight
        : [
            [a.cx + o, a.y],
            [a.cx + o, gy],
            [b.cx + o, gy],
            [b.cx + o, b.y],
          ];
    } else if (Math.abs(dx) < 1) {
      const straight: Point[] = [
        [a.cx + o, dy > 0 ? a.y + a.h : a.y],
        [b.cx + o, dy > 0 ? b.y : b.y + b.h],
      ];
      const gx = a.x + a.w + gapX / 2 + o;
      pts = clear(straight, skip)
        ? straight
        : [
            [a.x + a.w, a.cy + o],
            [gx, a.cy + o],
            [gx, b.cy + o],
            [b.x + b.w, b.cy + o],
          ];
    } else {
      const hFirst: Point[] = [
        [dx > 0 ? a.x + a.w : a.x, a.cy + o],
        [b.cx + o, a.cy + o],
        [b.cx + o, dy > 0 ? b.y : b.y + b.h],
      ];
      const vFirst: Point[] = [
        [a.cx + o, dy > 0 ? a.y + a.h : a.y],
        [a.cx + o, b.cy + o],
        [dx > 0 ? b.x : b.x + b.w, b.cy + o],
      ];
      pts = clear(hFirst, skip) || !clear(vFirst, skip) ? hFirst : vFirst;
    }
    for (const [x, y] of pts) {
      W = Math.max(W, x + PAD);
      H = Math.max(H, y + PAD);
    }
    return { index, pts, text, ...labelAt(pts, o) };
  });
  return { W, H, boxes, routes };
}

function labelAt(pts: Point[], offset: number): Pick<Route, 'lx' | 'ly' | 'anchor'> {
  let best = 0;
  let len = -1;
  pts.slice(1).forEach(([x2, y2], i) => {
    const [x1, y1] = pts[i]!;
    const l = Math.abs(x2 - x1) + Math.abs(y2 - y1);
    if (l > len) {
      len = l;
      best = i;
    }
  });
  const [x1, y1] = pts[best]!;
  const [x2, y2] = pts[best + 1]!;
  if (Math.abs(y2 - y1) < 1) return { lx: (x1 + x2) / 2, ly: offset > 0 ? y1 + 15 : y1 - 7, anchor: 'middle' };
  return offset < 0 ? { lx: x1 - 7, ly: (y1 + y2) / 2 + 4, anchor: 'end' } : { lx: x1 + 7, ly: (y1 + y2) / 2 + 4, anchor: 'start' };
}

export function arrowHead([x1, y1]: Point, [x2, y2]: Point, size = 7): string {
  const len = Math.hypot(x2 - x1, y2 - y1) || 1;
  const ux = (x2 - x1) / len;
  const uy = (y2 - y1) / len;
  const bx = x2 - ux * size;
  const by = y2 - uy * size;
  const px = -uy * (size / 2);
  const py = ux * (size / 2);
  return `M${x2},${y2}L${bx + px},${by + py}L${bx - px},${by - py}Z`;
}

export function pathOf(pts: Point[]): string {
  return pts.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x},${y}`).join('');
}

const mockBase =
  'html,body{margin:0}html{overflow:hidden}' +
  'body{font:14px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif;color:#1f2328;background:#fff}';

const terminalBase =
  'body{background:#16181d;color:#d7dae0;font:13px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace;' +
  'padding:12px 14px;white-space:pre-wrap}b{color:#fff}.dim{color:#7d8590}.g{color:#4ac26b}.r{color:#ff7b72}' +
  '.y{color:#e3b341}.b{color:#79c0ff}.m{color:#d2a8ff}.o{color:#ffa657}.inv{background:#d7dae0;color:#16181d}' +
  '.box{display:inline-block;border:1px solid #3d444d;padding:2px 8px}';

export function mockCsp(nonce: string): string {
  return `default-src 'none'; style-src 'unsafe-inline'; img-src data:; script-src 'nonce-${nonce}'; base-uri 'none'; form-action 'none'`;
}

export function neutralize(root: ParentNode): void {
  const navigating = new Set(['href', 'target', 'action', 'formaction', 'ping', 'srcdoc']);
  for (const el of root.querySelectorAll('meta,base,link,iframe,frame,frameset,object,embed,portal')) el.remove();
  for (const el of root.querySelectorAll('animate,set')) {
    if ((el.getAttribute('attributeName') ?? '').endsWith('href')) el.remove();
  }
  for (const el of root.querySelectorAll('*')) {
    for (const attr of [...el.attributes]) if (navigating.has(attr.localName)) el.removeAttributeNode(attr);
  }
}

export function mockDocument(spec: MockSpec, nonce: string, token: string): string {
  const payload = JSON.stringify({ html: spec.html, css: spec.css ?? '', token }).replace(/</g, '\\u003c');
  const host =
    `(()=>{const p=${payload};const d=new DOMParser().parseFromString(p.html,'text/html');(${String(neutralize)})(d);` +
    `const s=document.createElement('style');s.textContent=p.css;document.head.append(s,...d.head.querySelectorAll('style'));` +
    `document.body.append(...d.body.childNodes);` +
    `const post=()=>{const rects={};for(const el of document.querySelectorAll('[data-ref]')){` +
    `const r=el.getBoundingClientRect();rects[el.getAttribute('data-ref')]=[r.left,r.top,r.width,r.height];}` +
    `parent.postMessage({ccPresentMock:p.token,height:Math.ceil(document.documentElement.getBoundingClientRect().height),rects},'*');};` +
    `new ResizeObserver(post).observe(document.documentElement);addEventListener('load',post);post();})()`;
  return (
    '<!doctype html><html><head><meta charset="utf-8">' +
    `<meta http-equiv="Content-Security-Policy" content="${mockCsp(nonce)}">` +
    `<style>${mockBase}${spec.frame === 'terminal' ? terminalBase : ''}</style>` +
    `</head><body><script nonce="${nonce}">${host}</script></body></html>`
  );
}

export type Rect = [number, number, number, number];

export interface PinSpot {
  ref: string;
  n: number;
  x: number;
  y: number;
}

export function pinSpots(refs: string[], rects: Record<string, Rect>, scale: number, width: number, height: number): PinSpot[] {
  const out: PinSpot[] = [];
  refs.forEach((ref, i) => {
    const r = rects[ref];
    if (!r) return;
    const y = r[1] * scale;
    if (y > height) return;
    out.push({ ref, n: i + 1, x: clamp((r[0] + r[2]) * scale, 11, width - 11), y: clamp(y, 11, height - 11) });
  });
  return out;
}

export function setComment(comments: Readonly<Record<string, string>>, ref: string, text: string): Record<string, string> {
  const next = { ...comments };
  if (text.trim()) next[ref] = text.trim();
  else delete next[ref];
  return next;
}
