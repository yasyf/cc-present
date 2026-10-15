import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { PR as PRBlock } from '../schema';
import { useSubject } from '../api';
import { renderMarkdown } from '../markdown';
import {
  checksSummary,
  commitUrl,
  fetchPR,
  fetchedAgo,
  filePatch,
  prUrl,
  verdictTone,
  type PRFile,
  type PRLive,
} from '../pr';
import { DiffView } from './DiffView';

const FILE_STATUS: Record<string, string> = {
  added: 'A',
  removed: 'D',
  modified: 'M',
  renamed: 'R',
  copied: 'C',
  changed: 'M',
  unchanged: '=',
};

function prQueryKey(subject: string, block: PRBlock) {
  return ['pr', subject, block.id, block.repo, block.number, block.files ?? []] as const;
}

export function PRView({ block }: { block: PRBlock }) {
  const { subject } = useSubject();
  const query = useQuery<PRLive, Error>({
    queryKey: prQueryKey(subject, block),
    queryFn: () => fetchPR(subject, block.id),
    staleTime: 60_000,
    retry: false,
  });
  const pr = query.data;
  const url = pr?.url ?? prUrl(block.repo, block.number);

  return (
    <article className={`pr-block pr-${pr?.state ?? 'loading'}`}>
      <header className="pr-head">
        <div className="pr-line">
          {pr && <span className={`pr-pill pr-tone-${stateTone(pr.state)}`}>{pr.state}</span>}
          <a className="pr-ref" href={url} target="_blank" rel="noopener noreferrer">
            {block.repo}#{block.number}
          </a>
          {pr?.mergedSha && (
            <a className="pr-sha" href={commitUrl(block.repo, pr.mergedSha)} target="_blank" rel="noopener noreferrer">
              {pr.mergedSha.slice(0, 10)}
            </a>
          )}
          <span className="pr-actions">
            <button
              type="button"
              className="pr-refresh"
              onClick={() => void query.refetch()}
              disabled={query.isFetching}
            >
              {query.isFetching ? 'Refreshing…' : 'Refresh'}
            </button>
            <a className="pr-open" href={url} target="_blank" rel="noopener noreferrer">
              Open on GitHub
            </a>
          </span>
        </div>
        <h4 className="pr-title">
          <a href={url} target="_blank" rel="noopener noreferrer">
            {block.title ?? pr?.title ?? `Pull request #${block.number}`}
          </a>
        </h4>
        {pr && (
          <div className="pr-meta">
            {pr.author} · {pr.base} ← {pr.head} · <span className="pr-add">+{pr.additions}</span>{' '}
            <span className="pr-del">−{pr.deletions}</span> · {pr.changedFiles} {pr.changedFiles === 1 ? 'file' : 'files'}
          </div>
        )}
      </header>
      {block.summary && (
        <div className="prose markdown-block pr-summary" dangerouslySetInnerHTML={{ __html: renderMarkdown(block.summary) }} />
      )}
      {query.isPending && <div className="pr-status">Loading from GitHub…</div>}
      {query.isError && (
        <div className="pr-status pr-error" role="alert">
          {query.error.message}
        </div>
      )}
      {pr && <PRFacts pr={pr} />}
      {pr && <PRFiles pr={pr} filtered={(block.files?.length ?? 0) > 0} />}
      {pr && <div className="pr-fetched">fetched {fetchedAgo(pr.fetchedAt, Date.now())}</div>}
    </article>
  );
}

function stateTone(state: PRLive['state']): string {
  return { open: 'accent', draft: 'dim', merged: 'good', landed: 'good', closed: 'bad' }[state];
}

function PRFacts({ pr }: { pr: PRLive }) {
  const reviewed = pr.reviewDecision !== 'none' || pr.reviews.length > 0;
  return (
    <dl className="pr-facts">
      {pr.checks.total > 0 && (
        <div className="pr-fact">
          <dt>CI</dt>
          <dd>
            <span className={`pr-pill pr-tone-${verdictTone(pr.checks.state)}`}>{pr.checks.state}</span>
            <span>{checksSummary(pr.checks)}</span>
            {pr.checks.failing?.map((name) => (
              <code key={name} className="pr-failing">
                {name}
              </code>
            ))}
          </dd>
        </div>
      )}
      {reviewed && (
        <div className="pr-fact">
          <dt>Reviews</dt>
          <dd>
            {pr.reviewDecision !== 'none' && (
              <span className={`pr-pill pr-tone-${verdictTone(pr.reviewDecision)}`}>
                {pr.reviewDecision.replaceAll('_', ' ')}
              </span>
            )}
            {pr.reviews.map((r) => (
              <span key={r.author} className={`pr-reviewer pr-tone-${verdictTone(r.state)}`}>
                {r.author} {r.state.replaceAll('_', ' ')}
              </span>
            ))}
          </dd>
        </div>
      )}
      {pr.labels.length > 0 && (
        <div className="pr-fact">
          <dt>Labels</dt>
          <dd>
            {pr.labels.map((label) => (
              <span key={label} className="pr-pill pr-tone-dim">
                {label}
              </span>
            ))}
          </dd>
        </div>
      )}
    </dl>
  );
}

function PRFiles({ pr, filtered }: { pr: PRLive; filtered: boolean }) {
  if (pr.files.length === 0) {
    return <div className="pr-status">{filtered ? 'No changed file matches this block’s files filter.' : 'No changed files.'}</div>;
  }
  return (
    <section className="pr-files">
      <div className="pr-files-head">
        {filtered ? `${pr.files.length} of ${pr.changedFiles} files` : `${pr.files.length} files`}
      </div>
      <ul className="pr-file-list">
        {pr.files.map((file) => (
          <PRFileRow key={file.path} file={file} />
        ))}
      </ul>
    </section>
  );
}

function PRFileRow({ file }: { file: PRFile }) {
  const [open, setOpen] = useState(false);
  const hasPatch = Boolean(file.patch);
  return (
    <li className={`pr-file${open ? ' is-open' : ''}`}>
      <button
        type="button"
        className="pr-file-head"
        aria-expanded={hasPatch ? open : undefined}
        disabled={!hasPatch}
        onClick={() => setOpen(!open)}
      >
        <span className="pr-file-caret" aria-hidden="true">
          {hasPatch ? (open ? '▾' : '▸') : ''}
        </span>
        <span className={`pr-file-status pr-file-${file.status}`} title={file.status}>
          {FILE_STATUS[file.status] ?? file.status}
        </span>
        <span className="pr-file-path">{file.previousPath ? `${file.previousPath} → ${file.path}` : file.path}</span>
        {!hasPatch && <span className="pr-file-note">no text diff</span>}
        <span className="pr-file-stat">
          <span className="pr-add">+{file.additions}</span> <span className="pr-del">−{file.deletions}</span>
        </span>
      </button>
      {open && hasPatch && <DiffView block={{ id: `pr-file:${file.path}`, type: 'diff', diff: filePatch(file) }} />}
    </li>
  );
}
