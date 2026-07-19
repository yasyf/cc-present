import type { FileTree, FileTreeEntry, TreeBadge } from '../schema';
import { buildTree, type TreeNode } from '../filetree';

const BADGE_TONE: Record<TreeBadge, string> = {
  added: 'good',
  modified: 'warn',
  removed: 'bad',
};

// FileTreeView renders path entries as a nested tree of native <details>/<summary>
// disclosures, one per implicit directory. Leaf rows show the basename, an optional
// change badge, and an optional dim note; a removed entry's name is struck through.
export function FileTreeView({ block }: { block: FileTree }) {
  return (
    <figure className="filetree-block">
      {block.title ? <figcaption className="filetree-title">{block.title}</figcaption> : null}
      <div className="filetree-root">
        <TreeLevel nodes={buildTree(block.entries)} />
      </div>
    </figure>
  );
}

function TreeLevel({ nodes }: { nodes: TreeNode[] }) {
  return (
    <ul className="filetree-list">
      {nodes.map((node) => (
        <li key={node.path} className="filetree-item">
          {node.entry ? <FileRow name={node.name} entry={node.entry} /> : <DirNode node={node} />}
        </li>
      ))}
    </ul>
  );
}

function DirNode({ node }: { node: TreeNode }) {
  return (
    <details className="filetree-dir" open>
      <summary className="filetree-summary">{node.name}</summary>
      <TreeLevel nodes={node.children} />
    </details>
  );
}

function FileRow({ name, entry }: { name: string; entry: FileTreeEntry }) {
  return (
    <div className="filetree-file">
      <span className={`filetree-name${entry.badge === 'removed' ? ' filetree-struck' : ''}`}>{name}</span>
      {entry.badge ? (
        <span className={`filetree-badge filetree-badge-${BADGE_TONE[entry.badge]}`}>{entry.badge}</span>
      ) : null}
      {entry.note ? <span className="filetree-note">{entry.note}</span> : null}
    </div>
  );
}
