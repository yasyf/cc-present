import type { Diagram } from '../schema';

// Phase 1 stub: the diagram's title over its raw mermaid source in monospace. The
// live mermaid renderer (lazy client-side, Blue Pencil theme) lands in Phase 3.
export function DiagramView({ block }: { block: Diagram }) {
  return (
    <figure className="diagram-block">
      {block.title ? <figcaption className="diagram-title">{block.title}</figcaption> : null}
      <pre className="diagram-source">
        <code>{block.source}</code>
      </pre>
    </figure>
  );
}
