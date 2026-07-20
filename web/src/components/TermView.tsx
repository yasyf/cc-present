import { useEffect, useState } from 'react';
import type { Term } from '../schema';
import { stripAnsi } from '../ansi';
import { CopyButton } from './CopyButton';

// First paint is a plain <pre> of the stripped output; the lazy Shiki chunk swaps in the
// ANSI-colored HTML. The command row skips Shiki; copy always yields the stripped output.
export function TermView({ block }: { block: Term }) {
  const [rendered, setRendered] = useState<{ html: string; output: string } | null>(null);
  const stripped = stripAnsi(block.output);

  useEffect(() => {
    let alive = true;
    void (async () => {
      const { highlightAnsi } = await import('../highlight');
      const out = await highlightAnsi(block.output);
      if (alive) setRendered({ html: out, output: block.output });
    })();
    return () => {
      alive = false;
    };
  }, [block.output]);

  // Show the highlighted HTML only when it was computed from the current output, so an
  // output change never flashes the previous highlight under the new metadata.
  const html = rendered?.output === block.output ? rendered.html : null;

  return (
    <figure className="term-block">
      <header className="term-head">
        {block.title ? <span className="term-title">{block.title}</span> : null}
        <div className="term-tools">
          <CopyButton text={stripped} />
        </div>
      </header>
      {block.command ? (
        <div className="term-command">
          <span className="term-prompt" aria-hidden="true">
            ❯
          </span>
          <span className="term-command-text">{block.command}</span>
        </div>
      ) : null}
      {html ? (
        <div className="shiki-wrap" dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <pre className="term-plain">
          <code>{stripped}</code>
        </pre>
      )}
    </figure>
  );
}
