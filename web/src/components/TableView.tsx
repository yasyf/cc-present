import type { Table as TableBlock } from '../schema';
import { renderInlineMarkdown } from '../markdown';

export function TableView({ block }: { block: TableBlock }) {
  return (
    <div className="table-block">
      <table>
        <thead>
          <tr>
            {block.columns.map((col) => (
              <th key={col.key} style={{ textAlign: col.align ?? 'left' }}>
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
                  style={{ textAlign: col.align ?? 'left' }}
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
