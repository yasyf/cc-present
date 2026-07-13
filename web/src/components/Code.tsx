import { useEffect, useState } from 'react';
import type { Code as CodeBlock } from '../schema';
import { CopyButton } from './CopyButton';

// First paint is a plain <pre>; the lazily-imported Shiki chunk swaps in once it
// resolves. An uncurated language stays plain, and the header tag names it.
export function Code({ block }: { block: CodeBlock }) {
  const [html, setHtml] = useState<string | null>(null);
  const [highlightable, setHighlightable] = useState<boolean | null>(null);

  useEffect(() => {
    let alive = true;
    setHtml(null);
    setHighlightable(null);
    void (async () => {
      const { resolveLang, highlight } = await import('../highlight');
      const lang = resolveLang(block.lang);
      if (!alive) return;
      setHighlightable(lang != null);
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
      <header className="code-head">
        <span className="code-title">{block.title}</span>
        <div className="code-tools">
          <span className={`code-lang${highlightable === false ? ' code-lang-plain' : ''}`}>
            {highlightable === false ? `${block.lang} · plain text` : block.lang}
          </span>
          <CopyButton text={block.code} />
        </div>
      </header>
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
