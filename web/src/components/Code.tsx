import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import type { ThemedToken } from 'shiki/core';
import type { Code as CodeBlock, CodePin } from '../schema';
import { CopyButton } from './CopyButton';

// A block grounded in a file (src, start, or pins) renders numbered rows with
// highlights and pins; plain code paints a <pre> until Shiki swaps in.
export function Code({ block }: { block: CodeBlock }) {
  if (block.src || block.start || block.pins?.length) return <GroundedCode block={block} />;
  return <PlainCode block={block} />;
}

// An uncurated language stays plain, and the header tag names it.
function PlainCode({ block }: { block: CodeBlock }) {
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
        <CodeTools block={block} highlightable={highlightable} />
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

function CodeTools({ block, highlightable }: { block: CodeBlock; highlightable: boolean | null }) {
  return (
    <div className="code-tools">
      <span className={`code-lang${highlightable === false ? ' code-lang-plain' : ''}`}>
        {highlightable === false ? `${block.lang} · plain text` : block.lang}
      </span>
      <CopyButton text={block.code} />
    </div>
  );
}

function litLines(highlight: string | undefined): Set<number> {
  const lit = new Set<number>();
  for (const range of highlight?.split(',') ?? []) {
    const [a, b] = range.split('-').map(Number) as [number, number?];
    for (let n = a; n <= (b ?? a); n++) lit.add(n);
  }
  return lit;
}

function sourceLabel(block: CodeBlock): string | null {
  if (!block.src) return null;
  const lines = block.lines ? `:${block.lines}` : '';
  const sha = block.sha ? ` @ ${block.sha}` : '';
  return `${block.src}${lines}${sha}`;
}

function GroundedCode({ block }: { block: CodeBlock }) {
  const lines = useMemo(() => block.code.split('\n'), [block.code]);
  const lit = useMemo(() => litLines(block.highlight), [block.highlight]);
  const pins = useMemo(() => block.pins ?? [], [block.pins]);
  const pinsByLine = useMemo(() => {
    const m = new Map<number, number[]>();
    pins.forEach((p, i) => m.set(p.line, [...(m.get(p.line) ?? []), i + 1]));
    return m;
  }, [pins]);
  const [tokens, setTokens] = useState<{ code: string; lines: ThemedToken[][] } | null>(null);
  const [highlightable, setHighlightable] = useState<boolean | null>(null);

  useEffect(() => {
    let alive = true;
    setHighlightable(null);
    void (async () => {
      const { tokenizeLines, resolveLang } = await import('../highlight');
      const lang = resolveLang(block.lang);
      if (!alive) return;
      setHighlightable(lang != null);
      if (!lang) return;
      const out = await tokenizeLines(block.code, lang);
      if (alive) setTokens({ code: block.code, lines: out });
    })();
    return () => {
      alive = false;
    };
  }, [block.code, block.lang]);

  const rowTokens = tokens?.code === block.code ? tokens.lines : null;
  const start = block.start ?? 1;
  const label = sourceLabel(block);

  return (
    <figure className="code-block code-grounded">
      <header className="code-head">
        <span className="code-title">
          {block.title}
          {label && <span className="code-src">{label}</span>}
        </span>
        <CodeTools block={block} highlightable={highlightable} />
      </header>
      <div className="code-table">
        {lines.map((text, i) => {
          const n = start + i;
          const marks = pinsByLine.get(n);
          return (
            <div key={n} className={`code-row${lit.has(n) ? ' code-lit' : ''}`}>
              <span className="code-gutter">{n}</span>
              <code className="code-text">
                {rowTokens?.[i]
                  ? rowTokens[i].map((t, ti) => (
                      <span key={ti} className="code-tok" style={t.htmlStyle as CSSProperties}>
                        {t.content}
                      </span>
                    ))
                  : text}
              </code>
              <span className="code-marks">
                {marks?.map((no) => (
                  <span key={no} className="code-pin-mark" aria-hidden>
                    {no}
                  </span>
                ))}
              </span>
            </div>
          );
        })}
      </div>
      {pins.length > 0 && (
        <ol className="code-pins">
          {pins.map((p, i) => (
            <PinNote key={i} pin={p} no={i + 1} />
          ))}
        </ol>
      )}
    </figure>
  );
}

function PinNote({ pin, no }: { pin: CodePin; no: number }) {
  return (
    <li className={`code-pin code-pin-${pin.tone ?? 'default'}`}>
      <span className="code-pin-mark">{no}</span>
      <span className="code-pin-line">L{pin.line}</span>
      <span className="code-pin-title">{pin.title}</span>
      {pin.body && <span className="code-pin-body">{pin.body}</span>}
    </li>
  );
}
