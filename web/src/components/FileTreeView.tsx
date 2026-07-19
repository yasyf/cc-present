import type { FileTree } from '../schema';

// Placeholder FileTree renderer — Phase 1 replaces this body with the collapsible
// <details> tree and badge chips. It lists the entry paths so the block dispatches
// and typechecks.
export function FileTreeView({ block }: { block: FileTree }) {
  return (
    <figure className="filetree-block">
      {block.title ? <figcaption className="filetree-title">{block.title}</figcaption> : null}
      <ul className="filetree-list">
        {block.entries.map((entry) => (
          <li key={entry.path}>
            {entry.path}
            {entry.badge ? ` [${entry.badge}]` : ''}
          </li>
        ))}
      </ul>
    </figure>
  );
}
