import type { FileTreeEntry } from './schema';

// A node in a built file tree. A file leaf carries its source entry; a directory
// carries children and no entry.
export interface TreeNode {
  name: string;
  path: string;
  children: TreeNode[];
  entry?: FileTreeEntry;
}

// buildTree folds slash-separated path entries into a nested tree, one directory
// node per intermediate segment. Siblings sort directories before files,
// lexicographically by name within each group.
export function buildTree(entries: FileTreeEntry[]): TreeNode[] {
  const roots: TreeNode[] = [];
  for (const entry of entries) {
    const segments = entry.path.split('/');
    let level = roots;
    let prefix = '';
    for (const [i, name] of segments.entries()) {
      prefix = prefix ? `${prefix}/${name}` : name;
      if (i === segments.length - 1) {
        level.push({ name, path: prefix, children: [], entry });
        break;
      }
      let dir = level.find((node) => node.name === name && node.entry === undefined);
      if (!dir) {
        dir = { name, path: prefix, children: [] };
        level.push(dir);
      }
      level = dir.children;
    }
  }
  sortLevel(roots);
  return roots;
}

function sortLevel(nodes: TreeNode[]): void {
  nodes.sort((a, b) => {
    const aDir = a.entry === undefined;
    const bDir = b.entry === undefined;
    if (aDir !== bDir) return aDir ? -1 : 1;
    return a.name < b.name ? -1 : a.name > b.name ? 1 : 0;
  });
  for (const node of nodes) {
    if (node.children.length > 0) sortLevel(node.children);
  }
}
