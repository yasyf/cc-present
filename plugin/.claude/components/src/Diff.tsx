import { DiffView, renderMarkdown, tokens, usePackState } from './host/present';
import type { PackComponentProps } from './host/present';
import { Link, Meta, fetchedLabel, prUrl } from './shared';

interface DiffBlock {
  repo?: string;
  number?: number;
  title?: string;
  note?: string;
  max_lines?: number;
  patch: string;
  fetched_at?: string;
}

interface Section {
  path: string | undefined;
  lines: string[];
}

const DIFF_GIT = /^diff --git "?a\/.*"? "?b\/(.*?)"?$/;
const SECTION_META = /^(diff --git |index |--- |\+\+\+ )/;
const NEW_FILE = /^\+\+\+ b\/(.*)$/;

export function splitSections(patch: string): Section[] {
  const out: Section[] = [];
  let cur: Section | null = null;
  for (const line of patch.split('\n')) {
    const git = DIFF_GIT.exec(line);
    if (git || !cur) {
      cur = { path: git?.[1], lines: [] };
      out.push(cur);
    }
    if (!cur.path) cur.path = NEW_FILE.exec(line)?.[1];
    cur.lines.push(line);
  }
  return out;
}

function splitHunks(s: Section): { head: string[]; body: string[] } {
  const firstHunk = s.lines.findIndex((l) => l.startsWith('@@'));
  if (firstHunk === -1) return { head: s.lines, body: [] };
  return { head: s.lines.slice(0, firstHunk), body: s.lines.slice(firstHunk) };
}

// clip spends the line budget on hunk lines; file headers ride along free, and a
// file without hunks (a rename, mode change, or binary) costs one line.
export function clip(sections: Section[], budget: number): { shown: Section[]; hidden: number } {
  const shown: Section[] = [];
  let left = budget;
  let hidden = 0;
  for (const s of sections) {
    const { head, body } = splitHunks(s);
    const cost = Math.max(body.length, 1);
    if (left <= 0) {
      hidden += cost;
      continue;
    }
    shown.push({ path: s.path, lines: [...head, ...body.slice(0, left)] });
    hidden += Math.max(0, cost - left);
    left -= cost;
  }
  return { shown, hidden };
}

function churn(sections: Section[]): { add: number; del: number } {
  let add = 0;
  let del = 0;
  for (const s of sections) {
    for (const line of splitHunks(s).body) {
      if (line.startsWith('+')) add += 1;
      else if (line.startsWith('-')) del += 1;
    }
  }
  return { add, del };
}

function FileMeta({ section }: { section: Section }) {
  const t = tokens();
  const meta = section.lines.filter((l) => l !== '' && !SECTION_META.test(l));
  return (
    <figure style={{ margin: 0 }}>
      {section.path && <figcaption style={{ fontFamily: t.fontMono, fontSize: '0.78rem' }}>{section.path}</figcaption>}
      <pre style={{ margin: 0, color: t.dim, fontFamily: t.fontMono, fontSize: '0.75rem', whiteSpace: 'pre-wrap' }}>
        {meta.length > 0 ? meta.join('\n') : 'No textual change.'}
      </pre>
    </figure>
  );
}

export function Diff({ block }: PackComponentProps) {
  const t = tokens();
  const b = block as unknown as DiffBlock;
  const [open, setOpen] = usePackState('open', true);
  const [expanded, setExpanded] = usePackState('expanded', false);
  const sections = splitSections(b.patch);
  const { shown, hidden } = expanded ? { shown: sections, hidden: 0 } : clip(sections, b.max_lines ?? 60);
  const { add, del } = churn(sections);
  const files = sections.filter((s) => s.path).length;
  const button = {
    background: 'none',
    border: 'none',
    padding: 0,
    color: t.accent,
    cursor: 'pointer',
    fontFamily: t.fontMono,
    fontSize: '0.75rem',
  } as const;

  return (
    <section
      style={{
        border: `1px solid ${t.border}`,
        borderRadius: t.radiusMd,
        background: t.surface,
        color: t.text,
        padding: '0.6rem 0.8rem',
        display: 'grid',
        gap: '0.45rem',
      }}
    >
      <header style={{ display: 'flex', gap: '0.6rem', alignItems: 'baseline', flexWrap: 'wrap' }}>
        <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} style={{ ...button, color: t.dim }}>
          {open ? '▾' : '▸'}
        </button>
        <strong style={{ fontWeight: 600 }}>{b.title ?? 'Diff excerpt'}</strong>
        {b.repo && b.number !== undefined && (
          <Link href={`${prUrl(b.repo, b.number)}/files`} style={{ fontFamily: t.fontMono, fontSize: '0.78rem' }}>
            {b.repo}#{b.number}
          </Link>
        )}
        <Meta>
          {files > 0 && `${files} ${files === 1 ? 'file' : 'files'} · `}
          <span style={{ color: t.ok }}>+{add}</span> <span style={{ color: t.danger }}>−{del}</span>
        </Meta>
      </header>
      {open && (
        <>
          {b.note && <div style={{ fontSize: '0.9rem' }} dangerouslySetInnerHTML={{ __html: renderMarkdown(b.note) }} />}
          {shown.map((s, i) =>
            s.lines.some((l) => l.startsWith('@@')) ? (
              <DiffView key={`${s.path ?? ''}-${i}`} diff={s.lines.join('\n')} {...(s.path ? { title: s.path } : {})} />
            ) : (
              <FileMeta key={`${s.path ?? ''}-${i}`} section={s} />
            ),
          )}
          {(hidden > 0 || expanded) && (
            <button type="button" onClick={() => setExpanded(!expanded)} style={{ ...button, justifySelf: 'start' }}>
              {expanded ? 'Show less' : `Show ${hidden} more ${hidden === 1 ? 'line' : 'lines'}`}
            </button>
          )}
          {b.fetched_at && <Meta>{fetchedLabel(b.fetched_at)}</Meta>}
        </>
      )}
    </section>
  );
}
