// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SubjectProvider } from '../api';
import type { PR as PRBlock } from '../schema';
import type { PRLive } from '../pr';
import { PRView } from './PRView';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

let container: HTMLDivElement;
let root: Root;

const live: PRLive = {
  repo: 'yasyf/cc-present',
  number: 47,
  title: 'web: add the pr block',
  url: 'https://github.com/yasyf/cc-present/pull/47',
  author: 'yasyf',
  state: 'open',
  base: 'main',
  head: 'pr-builtin-card',
  additions: 12,
  deletions: 3,
  changedFiles: 3,
  checks: { state: 'failure', total: 5, passed: 3, failed: 1, pending: 1, skipped: 0, failing: ['web'] },
  reviewDecision: 'review_required',
  reviews: [{ author: 'rev', state: 'changes_requested' }],
  labels: ['web'],
  files: [
    { path: 'web/src/pr.ts', status: 'added', additions: 10, deletions: 0, patch: '@@ -0,0 +1,2 @@\n+export const a = 1;\n+export const b = 2;' },
    { path: 'docs/logo.png', status: 'modified', additions: 0, deletions: 0 },
  ],
  fetchedAt: new Date().toISOString(),
};

function respond(body: unknown, status = 200): Response {
  return new Response(typeof body === 'string' ? body : JSON.stringify(body), { status });
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

async function renderPR(block: PRBlock): Promise<void> {
  const client = new QueryClient();
  await act(async () => {
    root.render(
      <QueryClientProvider client={client}>
        <SubjectProvider value={{ subject: 'board--abcd', scope: undefined }}>
          <PRView block={block} />
        </SubjectProvider>
      </QueryClientProvider>,
    );
  });
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

const block: PRBlock = { id: 'pr1', type: 'pr', repo: 'yasyf/cc-present', number: 47, summary: 'Ships **today**.' };

describe('PRView', () => {
  it('fetches the block through the daemon and renders the live card', async () => {
    const fetchMock = vi.fn(async () => respond(live));
    vi.stubGlobal('fetch', fetchMock);
    await renderPR(block);

    expect(fetchMock).toHaveBeenCalledWith('/api/pr?subject=board--abcd&block=pr1');
    expect(container.querySelector('.pr-block')?.classList.contains('pr-open')).toBe(true);
    expect(container.querySelector('.pr-title')?.textContent).toBe('web: add the pr block');
    expect(container.querySelector('.pr-ref')?.getAttribute('href')).toBe(live.url);
    expect(container.querySelector('.pr-meta')?.textContent).toContain('main ← pr-builtin-card');
    expect(container.querySelector('.pr-summary strong')?.textContent).toBe('today');
    const facts = [...container.querySelectorAll('.pr-fact dt')].map((dt) => dt.textContent);
    expect(facts).toEqual(['CI', 'Reviews', 'Labels']);
    expect(container.querySelector('.pr-failing')?.textContent).toBe('web');
    expect(container.querySelector('.pr-fetched')?.textContent).toBe('fetched just now');
  });

  it('expands a file to its diff and leaves a patchless file closed', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => respond(live)));
    await renderPR(block);

    const heads = [...container.querySelectorAll<HTMLButtonElement>('.pr-file-head')];
    expect(heads).toHaveLength(2);
    expect(container.querySelector('.diff-block')).toBeNull();
    await act(async () => heads[0]!.click());
    expect(heads[0]!.getAttribute('aria-expanded')).toBe('true');
    const added = [...container.querySelectorAll('.diff-add .diff-text')].map((el) => el.textContent);
    expect(added).toEqual(['export const a = 1;', 'export const b = 2;']);
    expect(heads[1]!.disabled).toBe(true);
    expect(heads[1]!.textContent).toContain('no text diff');
  });

  it('prefers the block title and reports a filtered file count', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => respond({ ...live, files: live.files.slice(0, 1) })));
    await renderPR({ ...block, title: 'The pr block', files: ['web/src'] });

    expect(container.querySelector('.pr-title')?.textContent).toBe('The pr block');
    expect(container.querySelector('.pr-files-head')?.textContent).toBe('1 of 3 files');
  });

  it('refetches on refresh', async () => {
    const fetchMock = vi.fn(async () => respond(live));
    vi.stubGlobal('fetch', fetchMock);
    await renderPR(block);

    vi.stubGlobal('fetch', fetchMock.mockImplementation(async () => respond({ ...live, state: 'merged', mergedSha: 'c3fa5679538ba87b9ca6' })));
    await act(async () => container.querySelector<HTMLButtonElement>('.pr-refresh')!.click());
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(container.querySelector('.pr-block')?.classList.contains('pr-merged')).toBe(true);
    expect(container.querySelector('.pr-sha')?.textContent).toBe('c3fa567953');
  });

  it('shows the daemon error and keeps the link to GitHub', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => respond('gh api graphql: exit status 1: HTTP 401\n', 502)));
    await renderPR(block);

    expect(container.querySelector('.pr-error')?.textContent).toBe('gh api graphql: exit status 1: HTTP 401');
    expect(container.querySelector('.pr-title')?.textContent).toBe('Pull request #47');
    expect(container.querySelector('.pr-open')?.getAttribute('href')).toBe('https://github.com/yasyf/cc-present/pull/47');
  });
});
