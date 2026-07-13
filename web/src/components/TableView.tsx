import type { CSSProperties } from 'react';
import type { Column, Table as TableBlock } from '../schema';
import { renderInlineMarkdown } from '../markdown';
import { useScrollEdges } from '../useScrollEdges';

const ROW_CAP = 12;

function cellStyle(col: Column): CSSProperties {
  return {
    textAlign: col.align ?? 'left',
    fontVariantNumeric: col.align === 'right' ? 'tabular-nums' : undefined,
  };
}

export function TableView({ block }: { block: TableBlock }) {
  const { ref, edges } = useScrollEdges<HTMLDivElement>('x');
  const cls = ['table-block'];
  if (block.rows.length > ROW_CAP) cls.push('table-capped');
  if (!edges.atStart) cls.push('fade-start');
  if (!edges.atEnd) cls.push('fade-end');
  return (
    <div ref={ref} className={cls.join(' ')}>
      <table>
        <thead>
          <tr>
            {block.columns.map((col) => (
              <th key={col.key} style={cellStyle(col)}>
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {block.rows.map((row, ri) => (
            <tr key={ri}>
              {block.columns.map((col) => (
                <td
                  key={col.key}
                  style={cellStyle(col)}
                  dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(row[col.key] ?? '') }}
                />
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
