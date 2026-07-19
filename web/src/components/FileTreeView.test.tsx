// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { FileTreeView } from './FileTreeView';
import type { FileTree, FileTreeEntry } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const tree = (entries: FileTreeEntry[], title?: string): FileTree => ({
  id: 'f1',
  type: 'filetree',
  title,
  entries,
});

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

function render(block: FileTree): void {
  act(() => root.render(<FileTreeView block={block} />));
}

describe('FileTreeView', () => {
  it('renders a <details>/<summary> disclosure per directory, open by default', () => {
    render(tree([{ path: 'src/app/main.ts' }]));
    const details = container.querySelectorAll('details.filetree-dir');
    expect(details).toHaveLength(2);
    for (const el of details) expect((el as HTMLDetailsElement).open).toBe(true);
    expect([...container.querySelectorAll('summary.filetree-summary')].map((s) => s.textContent)).toEqual([
      'src',
      'app',
    ]);
  });

  it('shows the basename on the leaf row, not the full path', () => {
    render(tree([{ path: 'src/app/main.ts' }]));
    expect(container.querySelector('.filetree-name')?.textContent).toBe('main.ts');
  });

  it('renders the optional title in a figcaption', () => {
    render(tree([{ path: 'a.ts' }], 'Changed files'));
    expect(container.querySelector('.filetree-title')?.textContent).toBe('Changed files');
  });

  it('omits the figcaption when no title is set', () => {
    render(tree([{ path: 'a.ts' }]));
    expect(container.querySelector('.filetree-title')).toBeNull();
  });

  it('tones badges by change kind: added→good, modified→warn, removed→bad', () => {
    render(
      tree([
        { path: 'added.ts', badge: 'added' },
        { path: 'changed.ts', badge: 'modified' },
        { path: 'gone.ts', badge: 'removed' },
      ]),
    );
    const badges = [...container.querySelectorAll('.filetree-badge')];
    expect(badges.map((b) => b.textContent)).toEqual(['added', 'modified', 'removed']);
    expect(badges.map((b) => b.className)).toEqual([
      'filetree-badge filetree-badge-good',
      'filetree-badge filetree-badge-warn',
      'filetree-badge filetree-badge-bad',
    ]);
  });

  it('strikes through a removed entry name, and only that one', () => {
    render(
      tree([
        { path: 'gone.ts', badge: 'removed' },
        { path: 'new.ts', badge: 'added' },
      ]),
    );
    const struck = [...container.querySelectorAll('.filetree-name')].filter((n) =>
      n.classList.contains('filetree-struck'),
    );
    expect(struck.map((n) => n.textContent)).toEqual(['gone.ts']);
  });

  it('renders an inline note when present and omits it otherwise', () => {
    render(
      tree([
        { path: 'noted.ts', note: 'entry point' },
        { path: 'plain.ts' },
      ]),
    );
    const notes = [...container.querySelectorAll('.filetree-note')];
    expect(notes.map((n) => n.textContent)).toEqual(['entry point']);
  });

  it('omits the badge span for an unbadged entry', () => {
    render(tree([{ path: 'plain.ts' }]));
    expect(container.querySelector('.filetree-badge')).toBeNull();
  });
});
