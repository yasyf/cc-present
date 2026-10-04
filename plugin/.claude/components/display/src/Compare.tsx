import { renderInlineMarkdown, renderMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';

type Cell = string | { md: string; tone?: 'good' | 'bad' | 'warn' | 'neutral' };

interface CompareBlock {
  title?: string;
  caption?: string;
  columns: { label: string; sub?: string; highlight?: boolean }[];
  rows: { label: string; hint?: string; cells: Cell[] }[];
}

const marks = { good: '✓', bad: '✗', warn: '!', neutral: '–' };

export function Compare({ block }: PackComponentProps) {
  const b = block as unknown as CompareBlock;
  const hl = (i: number) => (b.columns[i]?.highlight ? ' ccpd-hl' : '');
  return (
    <section className="ccpd-frame">
      {b.title && <header className="ccpd-title">{b.title}</header>}
      {b.caption && <div className="prose ccpd-cmp-caption" dangerouslySetInnerHTML={{ __html: renderMarkdown(b.caption) }} />}
      <div className="ccpd-cmp-scroll">
        <table className="ccpd-cmp">
          <thead>
            <tr>
              <th />
              {b.columns.map((c, i) => (
                <th key={i} className={`ccpd-cmp-col${hl(i)}`}>
                  <div>{c.label}</div>
                  {c.sub && <div className="ccpd-cmp-sub">{c.sub}</div>}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {b.rows.map((r, ri) => (
              <tr key={ri} className="ccpd-cmp-row" style={{ animationDelay: `${ri * 60}ms` }}>
                <th scope="row" className="ccpd-cmp-label">
                  <div>{r.label}</div>
                  {r.hint && <div className="ccpd-cmp-sub">{r.hint}</div>}
                </th>
                {b.columns.map((_, ci) => (
                  <CompareCell key={ci} cell={r.cells[ci]} extra={hl(ci)} />
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function CompareCell({ cell, extra }: { cell: Cell | undefined; extra: string }) {
  if (cell === undefined) return <td className={`ccpd-cmp-cell${extra}`} />;
  const c = typeof cell === 'string' ? { md: cell } : cell;
  return (
    <td className={`ccpd-cmp-cell${extra}${c.tone ? ` ccpd-cell-${c.tone}` : ''}`}>
      {c.tone && <span className="ccpd-mark">{marks[c.tone]}</span>}
      <span dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(c.md) }} />
    </td>
  );
}
