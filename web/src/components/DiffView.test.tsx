// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';

vi.mock('../highlight', () => ({
  langFromPath: (path: string) => (path.endsWith('.py') ? 'python' : null),
  tokenizeLines: (code: string) =>
    Promise.resolve(
      code
        .split('\n')
        .map((line) => [{ content: line, offset: 0, htmlStyle: { color: '#111', '--shiki-dark': '#eee' } }]),
    ),
}));

import { DiffView } from './DiffView';
import type { Diff as DiffBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// useScrollEdges constructs a ResizeObserver; jsdom ships none.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

const diffBlock = (diff: string, title?: string): DiffBlock => ({ id: 'd', type: 'diff', diff, title });

async function renderDiff(block: DiffBlock) {
  await act(async () => {
    root.render(<DiffView block={block} />);
  });
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('DiffView syntax highlighting', () => {
  it('renders token spans once the language resolves from the file path', async () => {
    await renderDiff(
      diffBlock(`--- a/greet.py\n+++ b/greet.py\n@@ -1 +1 @@\n-x = 1\n+x = 2`),
    );
    const toks = Array.from(container.querySelectorAll('.diff-tok'));
    expect(toks.length).toBeGreaterThan(0);
    expect(toks.map((t) => t.textContent)).toContain('x = 2');
    const dark = (toks[0] as HTMLElement).style.getPropertyValue('--shiki-dark');
    expect(dark).toBe('#eee');
  });

  it('falls back to the title extension when no file path is captured', async () => {
    await renderDiff(diffBlock(`@@ -1 +1 @@\n-x = 1\n+x = 2`, 'snippet.py'));
    expect(container.querySelectorAll('.diff-tok').length).toBeGreaterThan(0);
  });

  it('stays permanently plain when no language is inferable', async () => {
    await renderDiff(diffBlock(`@@ -1 +1 @@\n-a\n+b`));
    expect(container.querySelectorAll('.diff-tok').length).toBe(0);
    expect(container.textContent).toContain('a');
    expect(container.textContent).toContain('b');
  });

  it('toggles horizontal edge-fade classes from the scroll edges', async () => {
    await renderDiff(diffBlock(`--- a/x.py\n+++ b/x.py\n@@ -1 +1 @@\n-a\n+b`));
    // jsdom reports zero scroll extent, so the scroller sits at both edges (no fades).
    const table = container.querySelector('.diff-table');
    expect(table?.classList.contains('fade-start')).toBe(false);
    expect(table?.classList.contains('fade-end')).toBe(false);
  });
});
