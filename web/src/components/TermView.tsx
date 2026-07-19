import type { Term } from '../schema';

// Placeholder Term renderer — Phase 1 replaces this body with the ANSI-highlighted panel
// (stripAnsi first paint, lazy shiki swap-in). It renders the raw output so the block
// dispatches and typechecks.
export function TermView({ block }: { block: Term }) {
  return (
    <figure className="term-block">
      {block.title ? <figcaption className="term-title">{block.title}</figcaption> : null}
      {block.command ? <div className="term-command">{block.command}</div> : null}
      <pre className="term-output">
        <code>{block.output}</code>
      </pre>
    </figure>
  );
}
