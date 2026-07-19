import { useEffect, useState } from 'react';
import type { Term } from '../schema';
import { CopyButton } from './CopyButton';

// The ansi-regex pattern (chalk/ansi-regex), built from a string so the ESC/CSI/BEL
// bytes stay as \u escapes rather than literal control characters in source.
const ANSI_ESCAPE = new RegExp(
  '[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-ntqry=><~]))',
  'g',
);

// stripAnsi drops ANSI escapes; first paint and the copy payload need it synchronously.
export function stripAnsi(text: string): string {
  return text.replace(ANSI_ESCAPE, '');
}

// First paint is a plain <pre> of the stripped output; the lazy Shiki chunk swaps in the
// ANSI-colored HTML. The command row skips Shiki; copy always yields the stripped output.
export function TermView({ block }: { block: Term }) {
  const [html, setHtml] = useState<string | null>(null);
  const stripped = stripAnsi(block.output);

  useEffect(() => {
    let alive = true;
    setHtml(null);
    void (async () => {
      const { highlightAnsi } = await import('../highlight');
      const out = await highlightAnsi(block.output);
      if (alive) setHtml(out);
    })();
    return () => {
      alive = false;
    };
  }, [block.output]);

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
