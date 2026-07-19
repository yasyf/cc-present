import type { Chart } from '../schema';

// Placeholder Chart renderer — Phase 1 replaces this body with the themed SVG bar/line
// chart. It renders the series values as plain text so the block dispatches and typechecks.
export function ChartView({ block }: { block: Chart }) {
  return (
    <figure className="chart-block">
      {block.title ? <figcaption className="chart-title">{block.title}</figcaption> : null}
      <ul className="chart-series">
        {block.series.map((series) => (
          <li key={series.label}>
            {series.label}: {series.values.join(', ')}
            {block.unit ? ` ${block.unit}` : ''}
          </li>
        ))}
      </ul>
    </figure>
  );
}
