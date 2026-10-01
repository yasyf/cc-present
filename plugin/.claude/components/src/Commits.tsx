import { tokens } from './host/present';
import type { PackComponentProps } from './host/present';
import { Link, Meta, commitUrl, fetchedLabel, prUrl, shortSha } from './shared';

interface CommitsBlock {
  repo: string;
  number: number;
  title?: string;
  commits: { sha: string; subject: string; author?: string }[];
  total: number;
  fetched_at: string;
}

export function Commits({ block }: PackComponentProps) {
  const t = tokens();
  const b = block as unknown as CommitsBlock;
  const omitted = b.total - b.commits.length;
  return (
    <section
      style={{
        border: `1px solid ${t.border}`,
        borderRadius: t.radiusMd,
        background: t.surface,
        color: t.text,
        padding: '0.6rem 0.8rem',
        display: 'grid',
        gap: '0.4rem',
      }}
    >
      <header style={{ display: 'flex', gap: '0.6rem', alignItems: 'baseline', flexWrap: 'wrap' }}>
        <strong style={{ fontWeight: 600 }}>{b.title ?? 'Commits'}</strong>
        <Link href={`${prUrl(b.repo, b.number)}/commits`} style={{ fontFamily: t.fontMono, fontSize: '0.78rem' }}>
          {b.repo}#{b.number}
        </Link>
        <Meta>{b.total === 1 ? '1 commit' : `${b.total} commits`}</Meta>
      </header>
      <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: '0.2rem', fontSize: '0.86rem' }}>
        {omitted > 0 && <Meta>{omitted} earlier commits not shown</Meta>}
        {b.commits.map((c) => (
          <li key={c.sha} style={{ display: 'flex', gap: '0.6rem', alignItems: 'baseline' }}>
            <Link href={commitUrl(b.repo, c.sha)} style={{ fontFamily: t.fontMono, fontSize: '0.75rem', flex: 'none' }}>
              {shortSha(c.sha)}
            </Link>
            <span style={{ minWidth: 0 }}>{c.subject}</span>
            {c.author && <Meta>{c.author}</Meta>}
          </li>
        ))}
      </ol>
      <Meta>{fetchedLabel(b.fetched_at)}</Meta>
    </section>
  );
}
