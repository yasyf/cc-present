export type Segment = { kind: 'md'; text: string } | { kind: 'mermaid'; source: string };

const mermaidFence = /^```mermaid[^\n]*\n([\s\S]*?)^```[ \t]*$/gm;

export function splitMermaid(md: string): Segment[] {
  const out: Segment[] = [];
  let last = 0;
  for (const m of md.matchAll(mermaidFence)) {
    const before = md.slice(last, m.index);
    if (before.trim()) out.push({ kind: 'md', text: before });
    out.push({ kind: 'mermaid', source: (m[1] ?? '').trimEnd() });
    last = m.index + m[0].length;
  }
  const rest = md.slice(last);
  if (rest.trim()) out.push({ kind: 'md', text: rest });
  return out;
}

export interface Actor {
  id: string;
  label: string;
}

export type Tone = 'default' | 'ok' | 'warn' | 'danger';

export interface Step {
  from: string;
  to?: string;
  label: string;
  note?: string;
  tone?: Tone;
  dashed?: boolean;
}

export interface Panel {
  title?: string;
  actors: Actor[];
  steps: Step[];
}

export function panelProblems(panel: Panel, index: number): string[] {
  const ids = new Set<string>();
  const problems: string[] = [];
  for (const a of panel.actors) {
    if (ids.has(a.id)) problems.push(`panel ${index + 1}: duplicate actor "${a.id}"`);
    ids.add(a.id);
  }
  panel.steps.forEach((s, i) => {
    for (const end of [s.from, s.to]) {
      if (end !== undefined && !ids.has(end)) problems.push(`panel ${index + 1} step ${i + 1}: unknown actor "${end}"`);
    }
  });
  return problems;
}

export function stepCount(panels: Panel[]): number {
  return Math.max(...panels.map((p) => p.steps.length));
}

export function artifactDocument(kind: 'html' | 'svg', source: string, frameId: string): string {
  const reporter =
    `<script>(()=>{const post=()=>parent.postMessage({ccPresentArtifact:${JSON.stringify(frameId)},` +
    `height:Math.ceil(document.documentElement.getBoundingClientRect().height)},'*');` +
    `new ResizeObserver(post).observe(document.documentElement);addEventListener('load',post);post();})()</script>`;
  if (kind === 'svg') {
    return (
      '<!doctype html><html><head><meta charset="utf-8"><style>html,body{margin:0}' +
      'svg{display:block;max-width:100%;height:auto;margin:0 auto}</style></head><body>' +
      source +
      reporter +
      '</body></html>'
    );
  }
  return source + reporter;
}

export interface Wrapped {
  lines: string[];
  clipped: boolean;
}

function breakWord(word: string, maxWidth: number, measure: (s: string) => number): string[] {
  const parts: string[] = [];
  let rest = word;
  while (measure(rest) > maxWidth) {
    let n = rest.length - 1;
    while (n >= 3 && measure(`${rest.slice(0, n)}-`) > maxWidth) n--;
    if (n < 3 || rest.length - n < 2) break;
    parts.push(`${rest.slice(0, n)}-`);
    rest = rest.slice(n);
  }
  return [...parts, rest];
}

export function wrapLabel(text: string, maxWidth: number, measure: (s: string) => number, maxLines = 3): Wrapped {
  const lines: string[] = [];
  for (const word of text.split(/\s+/).filter(Boolean)) {
    const [head = word, ...tail] = breakWord(word, maxWidth, measure);
    const last = lines[lines.length - 1];
    if (last !== undefined && measure(`${last} ${head}`) <= maxWidth) {
      lines[lines.length - 1] = `${last} ${head}`;
    } else {
      lines.push(head);
    }
    lines.push(...tail);
  }
  const kept = lines.slice(0, maxLines);
  let clipped = lines.length > maxLines;
  if (clipped) kept[maxLines - 1] = `${kept[maxLines - 1]} ${lines.slice(maxLines).join(' ')}`;
  return {
    lines: kept.map((line) => {
      if (measure(line) <= maxWidth) return line;
      clipped = true;
      let cut = line;
      while (cut.length > 1 && measure(`${cut}…`) > maxWidth) cut = cut.slice(0, -1);
      return `${cut.trimEnd()}…`;
    }),
    clipped,
  };
}
