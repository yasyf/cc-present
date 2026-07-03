import { useMemo } from 'react';
import type { Diff as DiffBlock } from '../schema';
import { parseDiff } from '../diff';

const MARK: Record<string, string> = { add: '+', del: '-', context: ' ', meta: '' };

export function DiffView({ block }: { block: DiffBlock }) {
  const hunks = useMemo(() => parseDiff(block.diff), [block.diff]);

  return (
    <figure className="diff-block">
      {block.title && <figcaption className="diff-title">{block.title}</figcaption>}
      <div className="diff-table">
        {hunks.map((hunk, hi) => (
          <div key={hi} className="diff-hunk">
            <div className="diff-row diff-hunk-head">
              <span className="diff-gutter" />
              <span className="diff-gutter" />
              <span className="diff-mark" />
              <code className="diff-text">{hunk.heading ? `@@ ${hunk.heading}` : '@@'}</code>
            </div>
            {hunk.rows.map((row, ri) => (
              <div key={ri} className={`diff-row diff-${row.kind}`}>
                <span className="diff-gutter">{row.oldNo ?? ''}</span>
                <span className="diff-gutter">{row.newNo ?? ''}</span>
                <span className="diff-mark">{MARK[row.kind]}</span>
                <code className="diff-text">{row.text}</code>
              </div>
            ))}
          </div>
        ))}
      </div>
    </figure>
  );
}
