import { useId, useMemo } from 'react';
import type { Chart } from '../schema';
import { formatValue, niceScale, seriesColors } from '../chart';
import { resolveColor } from '../cssColor';
import { useResolvedTheme } from '../theme';

const W = 560;
const H = 300;
const PAD_L = 44;
const PAD_R = 12;
const PAD_T = 12;
const PAD_B = 44;
const PLOT_W = W - PAD_L - PAD_R;
const PLOT_H = H - PAD_T - PAD_B;
const BAR_GAP = 2;
const MAX_BAR = 24;

function resolveAccent(): string {
  const probe = document.createElement('span');
  probe.style.cssText = 'position:absolute;width:0;height:0;overflow:hidden;visibility:hidden';
  document.body.appendChild(probe);
  const accent = resolveColor(probe, '--accent', '#3d56c5');
  probe.remove();
  return accent;
}

function barPath(x: number, y: number, w: number, h: number, roundTop: boolean): string {
  const r = Math.max(0, Math.min(4, w / 2, h));
  if (roundTop) {
    return `M${x},${y + h}L${x},${y + r}Q${x},${y} ${x + r},${y}L${x + w - r},${y}Q${x + w},${y} ${x + w},${y + r}L${x + w},${y + h}Z`;
  }
  return `M${x},${y}L${x},${y + h - r}Q${x},${y + h} ${x + r},${y + h}L${x + w - r},${y + h}Q${x + w},${y + h} ${x + w},${y + h - r}L${x + w},${y}Z`;
}

// ChartView renders a chart block into themed SVG: grouped bars or category-anchored
// lines over a baseline-0 scale, with a legend for two-plus series and per-mark
// tooltips. Series colors re-derive off --accent when the resolved theme flips; axis
// and label text always wears --text/--dim, never a series color.
export function ChartView({ block }: { block: Chart }) {
  const theme = useResolvedTheme();
  const ids = useId();
  const titleId = `${ids}-t`;
  const descId = `${ids}-d`;

  const colors = useMemo(
    () => seriesColors(resolveAccent(), theme, block.series.length),
    [theme, block.series.length],
  );

  const scale = useMemo(() => {
    const values = block.series.flatMap((s) => s.values);
    return niceScale(Math.min(...values), Math.max(...values));
  }, [block.series]);

  const yScale = (v: number) => PAD_T + PLOT_H * (1 - (v - scale.min) / (scale.max - scale.min));
  const baselineY = yScale(0);
  const bandW = PLOT_W / block.categories.length;
  const xCenter = (c: number) => PAD_L + bandW * c + bandW / 2;

  const n = block.series.length;
  const groupW = Math.min(bandW * 0.8, n * MAX_BAR + (n - 1) * BAR_GAP);
  const barW = (groupW - (n - 1) * BAR_GAP) / n;

  const desc = `${block.kind} chart, ${block.categories.length} categories, ${n} series`;
  const markTitle = (category: string, label: string, value: number) =>
    `${category} · ${label}: ${formatValue(value, block.unit)}`;

  return (
    <figure className="chart-block">
      {block.title ? <figcaption className="chart-title">{block.title}</figcaption> : null}
      <div className="chart-plot">
        <svg className="chart-svg" viewBox={`0 0 ${W} ${H}`} role="img" aria-labelledby={`${titleId} ${descId}`}>
          <title id={titleId}>{block.title ?? 'Chart'}</title>
          <desc id={descId}>{desc}</desc>
          {scale.ticks.map((t) => {
            const y = yScale(t);
            return (
              <g key={`tick-${t}`}>
                <line
                  className={t === 0 ? 'chart-baseline' : 'chart-grid'}
                  x1={PAD_L}
                  y1={y}
                  x2={PAD_L + PLOT_W}
                  y2={y}
                />
                <text className="chart-tick" x={PAD_L - 6} y={y} textAnchor="end" dominantBaseline="middle">
                  {formatValue(t)}
                </text>
              </g>
            );
          })}
          {block.categories.map((cat, c) => (
            <text key={`cat-${cat}`} className="chart-cat" x={xCenter(c)} y={H - PAD_B + 16} textAnchor="middle">
              {cat}
            </text>
          ))}
          {block.kind === 'bar'
            ? block.series.map((s, si) =>
                s.values.map((v, c) => {
                  const x = PAD_L + bandW * c + (bandW - groupW) / 2 + si * (barW + BAR_GAP);
                  const y1 = yScale(v);
                  const top = Math.min(baselineY, y1);
                  const h = Math.abs(y1 - baselineY);
                  return (
                    <path
                      key={`bar-${s.label}-${c}`}
                      className="chart-bar"
                      d={barPath(x, top, barW, h, v >= 0)}
                      fill={colors[si]}
                    >
                      <title>{markTitle(block.categories[c]!, s.label, v)}</title>
                    </path>
                  );
                }),
              )
            : block.series.map((s, si) => (
                <g key={`line-${s.label}`}>
                  <polyline
                    className="chart-line"
                    points={s.values.map((v, c) => `${xCenter(c)},${yScale(v)}`).join(' ')}
                    fill="none"
                    stroke={colors[si]}
                  />
                  {s.values.map((v, c) => (
                    <circle
                      key={`dot-${s.label}-${c}`}
                      className="chart-dot"
                      cx={xCenter(c)}
                      cy={yScale(v)}
                      r={3.5}
                      fill={colors[si]}
                    >
                      <title>{markTitle(block.categories[c]!, s.label, v)}</title>
                    </circle>
                  ))}
                </g>
              ))}
        </svg>
      </div>
      {n >= 2 ? (
        <ul className="chart-legend">
          {block.series.map((s, si) => (
            <li key={s.label} className="chart-legend-item">
              <span className="chart-swatch" style={{ background: colors[si] }} aria-hidden />
              <span className="chart-legend-label">{s.label}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </figure>
  );
}
