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

const DIFF_GIT = /^diff --git a\/.* b\/(.*)$/;
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

// clip spends the line budget on hunk lines only; file headers ride along free.
export function clip(sections: Section[], budget: number): { shown: Section[]; hidden: number } {
  const shown: Section[] = [];
  let left = budget;
  let hidden = 0;
  for (const s of sections) {
    const firstHunk = s.lines.findIndex((l) => l.startsWith('@@'));
    const head = firstHunk === -1 ? s.lines : s.lines.slice(0, firstHunk);
    const body = firstHunk === -1 ? [] : s.lines.slice(firstHunk);
    if (left <= 0) {
      hidden += body.length;
      continue;
    }
    shown.push({ path: s.path, lines: [...head, ...body.slice(0, left)] });
    hidden += Math.max(0, body.length - left);
    left -= body.length;
  }
  return { shown, hidden };
}

function churn(patch: string): { add: number; del: number } {
  let add = 0;
  let del = 0;
  for (const line of patch.split('\n')) {
    if (line.startsWith('+') && !line.startsWith('+++')) add += 1;
    else if (line.startsWith('-') && !line.startsWith('---')) del += 1;
  }
  return { add, del };
}

export function Diff({ block }: PackComponentProps) {
  const t = tokens();
  const b = block as unknown as DiffBlock;
  const [open, setOpen] = usePackState('open', true);
  const [expanded, setExpanded] = usePackState('expanded', false);
  const sections = splitSections(b.patch);
  const { shown, hidden } = expanded ? { shown: sections, hidden: 0 } : clip(sections, b.max_lines ?? 60);
  const { add, del } = churn(b.patch);
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
          {shown.map((s, i) => (
            <DiffView key={`${s.path ?? ''}-${i}`} diff={s.lines.join('\n')} {...(s.path ? { title: s.path } : {})} />
          ))}
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
