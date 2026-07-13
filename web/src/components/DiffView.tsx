import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import type { ThemedToken } from 'shiki/core';
import type { Diff as DiffBlock } from '../schema';
import { parseDiff } from '../diff';
import { useScrollEdges } from '../useScrollEdges';

const MARK: Record<string, string> = { add: '+', del: '-', context: ' ', meta: '' };

// Tokens per hunk (null = that hunk's language wasn't inferable) → tokens per
// row (aligned to the joined hunk source) → tokens per row segment.
type HunkTokens = (ThemedToken[][] | null)[];

export function DiffView({ block }: { block: DiffBlock }) {
  const hunks = useMemo(() => parseDiff(block.diff), [block.diff]);
  const [tokens, setTokens] = useState<HunkTokens | null>(null);
  const { ref, edges } = useScrollEdges<HTMLDivElement>('x');

  useEffect(() => {
    let alive = true;
    setTokens(null);
    void (async () => {
      const { tokenizeLines, langFromPath } = await import('../highlight');
      const titleLang = block.title ? langFromPath(block.title) : null;
      const out = await Promise.all(
        hunks.map((hunk) => {
          const lang = (hunk.path ? langFromPath(hunk.path) : null) ?? titleLang;
          if (!lang) return Promise.resolve(null);
          return tokenizeLines(hunk.rows.map((r) => r.text).join('\n'), lang);
        }),
      );
      if (alive) setTokens(out);
    })();
    return () => {
      alive = false;
    };
  }, [hunks, block.title]);

  const cls = ['diff-table'];
  if (!edges.atStart) cls.push('fade-start');
  if (!edges.atEnd) cls.push('fade-end');

  return (
    <figure className="diff-block">
      {block.title && <figcaption className="diff-title">{block.title}</figcaption>}
      <div ref={ref} className={cls.join(' ')}>
        {hunks.map((hunk, hi) => (
          <div key={hi} className="diff-hunk">
            <div className="diff-row diff-hunk-head">
              <span className="diff-gutter" />
              <span className="diff-gutter" />
              <span className="diff-mark" />
              <code className="diff-text">{hunk.heading ? `@@ ${hunk.heading}` : '@@'}</code>
            </div>
            {hunk.rows.map((row, ri) => {
              const rowTokens = row.kind === 'meta' ? null : tokens?.[hi]?.[ri];
              return (
                <div key={ri} className={`diff-row diff-${row.kind}`}>
                  <span className="diff-gutter">{row.oldNo ?? ''}</span>
                  <span className="diff-gutter">{row.newNo ?? ''}</span>
                  <span className="diff-mark">{MARK[row.kind]}</span>
                  <code className="diff-text">
                    {rowTokens
                      ? rowTokens.map((t, ti) => (
                          <span key={ti} className="diff-tok" style={t.htmlStyle as CSSProperties}>
                            {t.content}
                          </span>
                        ))
                      : row.text}
                  </code>
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </figure>
  );
}
