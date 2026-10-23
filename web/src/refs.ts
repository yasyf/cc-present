import { withToken } from './token';

export interface SnippetChunk {
  start: number;
  code: string;
}

// A resolved reference's hover payload, stored by the CLI at push and served
// from the content-addressed asset store.
export interface Snippet {
  kind: 'code' | 'record' | 'commit';
  label: string;
  title?: string;
  url?: string;
  path?: string;
  sha?: string;
  lang?: string;
  highlight?: string;
  chunks?: SnippetChunk[];
  body?: string;
  tags?: string[];
  author?: string;
  date?: string;
}

export function fromRef(e: { target: EventTarget | null }): boolean {
  return e.target instanceof Element && e.target.closest('a.ref') !== null;
}

const snippets = new Map<string, Promise<Snippet>>();

export function loadSnippet(sha: string): Promise<Snippet> {
  let p = snippets.get(sha);
  if (!p) {
    p = fetch(withToken(`/assets/${sha}`)).then(async (res) => {
      if (!res.ok) throw new Error(`snippet ${sha.slice(0, 7)}: ${res.status}`);
      return (await res.json()) as Snippet;
    });
    p.catch(() => snippets.delete(sha));
    snippets.set(sha, p);
  }
  return p;
}

export interface PopoverPlacement {
  top: number;
  left: number;
  above: boolean;
}

// placePopover anchors a popover of the given width under the reference, or
// above it when the viewport below is shorter than maxHeight and above is not.
export function placePopover(
  anchor: { top: number; bottom: number; left: number },
  viewport: { width: number; height: number },
  size: { width: number; maxHeight: number },
  gap = 6,
  margin = 8,
): PopoverPlacement {
  const below = viewport.height - anchor.bottom - gap - margin;
  const aboveRoom = anchor.top - gap - margin;
  const above = below < size.maxHeight && aboveRoom > below;
  const left = Math.max(margin, Math.min(anchor.left, viewport.width - size.width - margin));
  return { top: above ? anchor.top - gap : anchor.bottom + gap, left, above };
}
