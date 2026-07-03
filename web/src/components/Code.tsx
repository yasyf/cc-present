import { useEffect, useState } from 'react';
import type { Code as CodeBlock } from '../schema';

// First paint is a plain <pre>; the Shiki chunk is imported lazily and the
// highlighted HTML swaps in once it resolves. A language outside the curated set
// stays as plain text.
export function Code({ block }: { block: CodeBlock }) {
  const [html, setHtml] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    void (async () => {
      const { resolveLang, highlight } = await import('../highlight');
      const lang = resolveLang(block.lang);
      if (!lang) return;
      const out = await highlight(block.code, lang);
      if (alive) setHtml(out);
    })();
    return () => {
      alive = false;
    };
  }, [block.code, block.lang]);

  return (
    <figure className="code-block">
      {block.title && <figcaption className="code-title">{block.title}</figcaption>}
      {html ? (
        <div className="shiki-wrap" dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <pre className="code-plain">
          <code>{block.code}</code>
        </pre>
      )}
    </figure>
  );
}
