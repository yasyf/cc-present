import { describe, expect, it } from 'vitest';
import type { FileTreeEntry } from './schema';
import { buildTree, type TreeNode } from './filetree';

const entries = (...paths: string[]): FileTreeEntry[] => paths.map((path) => ({ path }));

// Flatten to a `<depth><dir '/' | file>name` shape for terse structural assertions.
function shape(nodes: TreeNode[], depth = 0): string[] {
  const rows: string[] = [];
  for (const node of nodes) {
    const kind = node.entry === undefined ? '/' : '';
    rows.push(`${'  '.repeat(depth)}${node.name}${kind}`);
    rows.push(...shape(node.children, depth + 1));
  }
  return rows;
}

describe('buildTree', () => {
  it('nests a deep path into one directory node per intermediate segment', () => {
    const tree = buildTree(entries('src/app/main.ts'));
    expect(shape(tree)).toEqual(['src/', '  app/', '    main.ts']);
    // The leaf carries its source entry; directories carry none.
    expect(tree[0]?.entry).toBeUndefined();
    expect(tree[0]?.path).toBe('src');
    expect(tree[0]?.children[0]?.children[0]?.entry).toEqual({ path: 'src/app/main.ts' });
  });

  it('collapses shared prefixes into one directory with implicit dir nodes', () => {
    const tree = buildTree(entries('src/a.ts', 'src/b.ts', 'src/deep/c.ts'));
    expect(shape(tree)).toEqual(['src/', '  deep/', '    c.ts', '  a.ts', '  b.ts']);
  });

  it('orders directories before files, then lexicographically within each group', () => {
    const tree = buildTree(entries('z.ts', 'beta/x.ts', 'm.ts', 'alpha/y.ts'));
    expect(shape(tree)).toEqual(['alpha/', '  y.ts', 'beta/', '  x.ts', 'm.ts', 'z.ts']);
  });

  it('sorts nested siblings the same way at every level', () => {
    const tree = buildTree(entries('pkg/z.ts', 'pkg/sub/b.ts', 'pkg/a.ts'));
    expect(shape(tree)).toEqual(['pkg/', '  sub/', '    b.ts', '  a.ts', '  z.ts']);
  });

  it('preserves each leaf entry (badge and note) on the file node', () => {
    const tree = buildTree([
      { path: 'src/gone.ts', badge: 'removed', note: 'superseded' },
      { path: 'src/new.ts', badge: 'added' },
    ]);
    const dir = tree[0]!;
    expect(dir.children.map((c) => c.entry)).toEqual([
      { path: 'src/gone.ts', badge: 'removed', note: 'superseded' },
      { path: 'src/new.ts', badge: 'added' },
    ]);
  });

  it('returns an empty forest for no entries', () => {
    expect(buildTree([])).toEqual([]);
  });

  it('keeps a removed file and a same-named added directory as distinct nodes', () => {
    const built = buildTree([
      { path: 'a', badge: 'removed' },
      { path: 'a/b', badge: 'added' },
    ]);
    // Directories sort before files, so the implicit "a/" precedes the removed "a" file.
    expect(shape(built)).toEqual(['a/', '  b', 'a']);
    const [dir, file] = built;
    expect(dir?.entry).toBeUndefined();
    expect(file?.entry).toEqual({ path: 'a', badge: 'removed' });
    // Same path, distinct kind — the render key must fold in kind to stay unique.
    expect(dir?.path).toBe(file?.path);
  });
});
