import { Fragment, useState } from 'react';
import type { CSSProperties } from 'react';
import type { Column, Table as TableBlock, TableAction } from '../schema';
import { renderInlineMarkdown, renderMarkdown } from '../markdown';
import { usePresent } from '../present';
import { filterChoices, visibleRows } from '../tableRows';
import type { TableQuery } from '../tableRows';
import { useScrollEdges } from '../useScrollEdges';
import { uuid } from '../uuid';

function cellStyle(col: Column): CSSProperties {
  return {
    textAlign: col.align ?? 'left',
    fontVariantNumeric: col.align === 'right' ? 'tabular-nums' : undefined,
  };
}

function RowActions({ block, name, actions }: { block: TableBlock; name: string; actions: TableAction[] }) {
  const { post } = usePresent();
  return (
    <div className="table-row-actions">
      {actions.map((action) => (
        <button
          key={action.id}
          type="button"
          className="link-btn"
          onClick={() =>
            void post({ type: 'feedback.created', id: uuid(), blockId: action.target ?? block.id, text: `${action.label}: ${name}` })
          }
        >
          {action.label}
        </button>
      ))}
    </div>
  );
}

function TableTools({
  block,
  query,
  setQuery,
}: {
  block: TableBlock;
  query: TableQuery;
  setQuery: (query: TableQuery) => void;
}) {
  const filters = block.columns.filter((col) => col.filter);
  if (filters.length === 0) return null;
  return (
    <div className="table-tools">
      <input
        className="field"
        type="search"
        aria-label="Filter rows"
        placeholder="Filter"
        value={query.text}
        onChange={(e) => setQuery({ ...query, text: e.target.value })}
      />
      {filters.flatMap((col) =>
        filterChoices(block, col).map((value) => {
          const on = query.chips[col.key] === value;
          return (
            <button
              key={`${col.key}:${value}`}
              type="button"
              className={on ? 'table-chip on' : 'table-chip'}
              aria-pressed={on}
              onClick={() => {
                const chips = { ...query.chips };
                if (on) delete chips[col.key];
                else chips[col.key] = value;
                setQuery({ ...query, chips });
              }}
            >
              {value}
            </button>
          );
        }),
      )}
    </div>
  );
}

export function TableView({ block }: { block: TableBlock }) {
  const { ref, edges } = useScrollEdges<HTMLDivElement>('x');
  const [query, setQuery] = useState<TableQuery>({ text: '', chips: {} });
  const [open, setOpen] = useState<string | null>(null);
  const cls = ['table-block'];
  if (!edges.atStart) cls.push('fade-start');
  if (!edges.atEnd) cls.push('fade-end');
  const { rowKey, details, actions } = block;
  const opens = (key: string) => Boolean(details?.[key] || actions?.length);
  const span = block.columns.length;

  function pressHeader(col: Column) {
    const descending = query.sort?.key === col.key && !query.sort.descending;
    setQuery({ ...query, sort: { key: col.key, descending } });
  }

  return (
    <>
      <TableTools block={block} query={query} setQuery={setQuery} />
      <div ref={ref} className={cls.join(' ')}>
        <table>
          <thead>
            <tr>
              {block.columns.map((col) => (
                <th key={col.key} style={cellStyle(col)} aria-sort={query.sort?.key !== col.key ? undefined : query.sort.descending ? 'descending' : 'ascending'}>
                  {col.sort ? (
                    <button type="button" className="table-sort" onClick={() => pressHeader(col)}>
                      {col.label}
                    </button>
                  ) : (
                    col.label
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visibleRows(block, query).map((row, ri) => {
              const key = rowKey ? (row[rowKey] ?? '') : '';
              const expandable = rowKey !== undefined && opens(key);
              return (
                <Fragment key={rowKey ? key : ri}>
                  <tr
                    className={expandable ? 'table-row-opens' : undefined}
                    onClick={expandable ? () => setOpen(open === key ? null : key) : undefined}
                  >
                    {block.columns.map((col) => (
                      <td
                        key={col.key}
                        style={cellStyle(col)}
                        dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(row[col.key] ?? '') }}
                      />
                    ))}
                  </tr>
                  {expandable && open === key && (
                    <tr className="table-row-detail">
                      <td colSpan={span}>
                        {details?.[key] && (
                          <div className="prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(details[key]) }} />
                        )}
                        {actions && actions.length > 0 && <RowActions block={block} name={key} actions={actions} />}
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </>
  );
}
