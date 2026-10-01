import type { ReactNode } from 'react';
import { renderMarkdown, tokens } from './host/present';
import type { PackComponentProps, ThemeTokens } from './host/present';
import { Link, Meta, Pill, commitUrl, fetchedLabel, shortSha } from './shared';

type State = 'open' | 'draft' | 'merged' | 'landed' | 'closed';

interface Checks {
  state: string;
  total: number;
  passed: number;
  failed: number;
  pending: number;
  skipped: number;
  failing?: string[];
}

interface CardBlock {
  repo: string;
  number: number;
  note?: string;
  title: string;
  url: string;
  author: string;
  state: State;
  base?: string;
  head?: string;
  merged_sha?: string;
  additions?: number;
  deletions?: number;
  changed_files?: number;
  checks?: Checks;
  review_decision?: string;
  reviews?: { author: string; state: string }[];
  labels?: string[];
  fetched_at: string;
}

function stateColor(t: ThemeTokens, state: State): string {
  return { open: t.accent, draft: t.dim, merged: t.ok, landed: t.ok, closed: t.danger }[state];
}

function verdictColor(t: ThemeTokens, verdict: string): string {
  if (verdict === 'approved' || verdict === 'success') return t.ok;
  if (verdict === 'changes_requested' || verdict === 'failure' || verdict === 'error') return t.danger;
  if (verdict === 'pending' || verdict === 'expected' || verdict === 'review_required') return t.warn;
  return t.dim;
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  const t = tokens();
  return (
    <div style={{ display: 'contents' }}>
      <dt
        style={{
          color: t.dim,
          fontFamily: t.fontMono,
          fontSize: '0.68rem',
          textTransform: 'uppercase',
          letterSpacing: t.trackCaps,
          paddingTop: '0.15rem',
        }}
      >
        {label}
      </dt>
      <dd style={{ margin: 0, display: 'flex', flexWrap: 'wrap', gap: '0.3rem', alignItems: 'center' }}>{children}</dd>
    </div>
  );
}

function checksText(c: Checks): string {
  const parts = [`${c.passed}/${c.total} passed`];
  if (c.failed > 0) parts.push(`${c.failed} failed`);
  if (c.pending > 0) parts.push(`${c.pending} pending`);
  if (c.skipped > 0) parts.push(`${c.skipped} skipped`);
  return parts.join(' · ');
}

export function Card({ block }: PackComponentProps) {
  const t = tokens();
  const b = block as unknown as CardBlock;
  return (
    <article
      style={{
        border: `1px solid ${t.border}`,
        borderLeft: `3px solid ${stateColor(t, b.state)}`,
        borderRadius: t.radiusMd,
        background: t.surface,
        color: t.text,
        padding: '0.7rem 0.9rem',
        display: 'grid',
        gap: '0.5rem',
      }}
    >
      <header style={{ display: 'grid', gap: '0.25rem' }}>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
          <Pill color={stateColor(t, b.state)}>{b.state}</Pill>
          <Link href={b.url} style={{ fontFamily: t.fontMono, fontSize: '0.8rem' }}>
            {b.repo}#{b.number}
          </Link>
          {b.merged_sha && (
            <Link href={commitUrl(b.repo, b.merged_sha)} style={{ fontFamily: t.fontMono, fontSize: '0.75rem' }}>
              {shortSha(b.merged_sha)}
            </Link>
          )}
        </div>
        <Link href={b.url} style={{ color: t.text, fontWeight: 600, lineHeight: 1.35 }}>
          {b.title}
        </Link>
        <Meta>
          {b.author}
          {b.base && b.head && ` · ${b.base} ← ${b.head}`}
          {b.additions !== undefined && b.deletions !== undefined && (
            <>
              {' · '}
              <span style={{ color: t.ok }}>+{b.additions}</span> <span style={{ color: t.danger }}>−{b.deletions}</span>
            </>
          )}
          {b.changed_files !== undefined && ` · ${b.changed_files} files`}
        </Meta>
      </header>
      {b.note && <div style={{ fontSize: '0.9rem' }} dangerouslySetInnerHTML={{ __html: renderMarkdown(b.note) }} />}
      <dl style={{ display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '0.3rem 0.8rem', margin: 0, fontSize: '0.85rem' }}>
        {b.checks && b.checks.total > 0 && (
          <Fact label="CI">
            <Pill color={verdictColor(t, b.checks.state)}>{b.checks.state}</Pill>
            <span>{checksText(b.checks)}</span>
            {b.checks.failing?.map((name) => (
              <code key={name} style={{ color: t.danger, fontSize: '0.75rem' }}>
                {name}
              </code>
            ))}
          </Fact>
        )}
        {((b.reviews && b.reviews.length > 0) || (b.review_decision && b.review_decision !== 'none')) && (
          <Fact label="Reviews">
            {b.review_decision && b.review_decision !== 'none' && (
              <Pill color={verdictColor(t, b.review_decision)}>{b.review_decision.replaceAll('_', ' ')}</Pill>
            )}
            {b.reviews?.map((r) => (
              <span key={r.author} style={{ color: verdictColor(t, r.state), fontSize: '0.8rem' }}>
                {r.author} {r.state === 'approved' ? '✓' : r.state === 'changes_requested' ? '✗' : '·'}
              </span>
            ))}
          </Fact>
        )}
        {b.labels && b.labels.length > 0 && (
          <Fact label="Labels">
            {b.labels.map((l) => (
              <Pill key={l} color={t.dim}>
                {l}
              </Pill>
            ))}
          </Fact>
        )}
      </dl>
      <Meta>{fetchedLabel(b.fetched_at)}</Meta>
    </article>
  );
}
