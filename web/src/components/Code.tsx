import { useEffect, useState } from 'react';
import type { Code as CodeBlock } from '../schema';
import { CopyButton } from './CopyButton';

// First paint is a plain <pre>; the lazily-imported Shiki chunk swaps in once it
// resolves. An uncurated language stays plain, and the header tag names it.
export function Code({ block }: { block: CodeBlock }) {
  const [rendered, setRendered] = useState<{ html: string; code: string; lang: string } | null>(null);
  const [highlightable, setHighlightable] = useState<boolean | null>(null);

  useEffect(() => {
    let alive = true;
    setHighlightable(null);
    void (async () => {
      const { resolveLang, highlight } = await import('../highlight');
      const lang = resolveLang(block.lang);
      if (!alive) return;
      setHighlightable(lang != null);
      if (!lang) return;
      const out = await highlight(block.code, lang);
      if (alive) setRendered({ html: out, code: block.code, lang: block.lang });
    })();
    return () => {
      alive = false;
    };
  }, [block.code, block.lang]);

  // Show the highlighted HTML only when it was computed from the current code, so a prop
  // change never flashes the previous highlight under the new source.
  const html = rendered?.code === block.code && rendered.lang === block.lang ? rendered.html : null;

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
