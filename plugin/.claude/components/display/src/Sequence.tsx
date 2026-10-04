import { useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';
import { renderInlineMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';
import { panelProblems, stepCount, wrapLabel } from './logic';
import type { Panel, Step, Tone, Wrapped } from './logic';
import { Controls, usePlayer } from './player';

interface SequenceBlock {
  title?: string;
  autoplay?: boolean;
  intervalMs?: number;
  panels: Panel[];
}

const tones: Tone[] = ['default', 'ok', 'warn', 'danger'];

export function Sequence({ block }: PackComponentProps) {
  const b = block as unknown as SequenceBlock;
  const root = useRef<HTMLElement>(null);
  const player = usePlayer(stepCount(b.panels), b.intervalMs ?? 1600, b.autoplay ?? true, root);
  const problems = b.panels.flatMap(panelProblems);
  if (problems.length > 0) {
    return (
      <section className="ccpd-frame">
        <div className="ccpd-error" role="alert">
          {problems.map((p) => (
            <div key={p}>{p}</div>
          ))}
        </div>
      </section>
    );
  }
  return (
    <section className="ccpd-frame" ref={root}>
      {b.title && <header className="ccpd-title">{b.title}</header>}
      <div className="ccpd-panels">
        {b.panels.map((panel, i) => (
          <SequencePanel key={i} panel={panel} shown={Math.min(player.shown, panel.steps.length)} />
        ))}
      </div>
      <Controls player={player} unit="Step" />
    </section>
  );
}

const fontPx = 12;
const lineH = 15;

function useWidth(el: RefObject<HTMLElement | null>): number {
  const [width, setWidth] = useState(0);
  useEffect(() => {
    if (!el.current) return;
    const ro = new ResizeObserver((entries) => setWidth(entries[0]?.contentRect.width ?? 0));
    ro.observe(el.current);
    return () => ro.disconnect();
  }, [el]);
  return width;
}

function measurer(el: HTMLElement | null, weight: number): (s: string) => number {
  const ctx = document.createElement('canvas').getContext('2d');
  if (!ctx || !el) return (s) => s.length * fontPx * 0.6;
  ctx.font = `${weight} ${fontPx}px ${getComputedStyle(el).fontFamily}`;
  return (s) => ctx.measureText(s).width;
}

function Label({ x, y, wrapped, full, className }: { x: number; y: number; wrapped: Wrapped; full: string; className: string }) {
  return (
    <text className={className} x={x} y={y} textAnchor="middle">
      {wrapped.clipped && <title>{full}</title>}
      {wrapped.lines.map((line, i) => (
        <tspan key={i} x={x} dy={i === 0 ? 0 : lineH}>
          {line}
        </tspan>
      ))}
    </text>
  );
}

function SequencePanel({ panel, shown }: { panel: Panel; shown: number }) {
  const fig = useRef<HTMLElement>(null);
  const width = useWidth(fig) || 600;
  const colW = width / panel.actors.length;
  const x = new Map(panel.actors.map((a, i) => [a.id, colW * (i + 0.5)]));
  const actorText = measurer(fig.current, 600);
  const stepText = measurer(fig.current, 400);
  const actors = panel.actors.map((a) => wrapLabel(a.label, colW - 10, actorText));
  const headH = 12 + lineH * Math.max(...actors.map((w) => w.lines.length));
  const rows = panel.steps.map((s) => {
    const note = s.to === undefined || s.to === s.from;
    const span = note ? colW - 18 : Math.abs((x.get(s.to as string) ?? 0) - (x.get(s.from) ?? 0));
    const wrapped = wrapLabel(s.label, note ? Math.min(width - 24, Math.max(span, colW * 2.2)) : Math.min(width - 8, Math.max(span - 8, colW * 2.4)), stepText);
    const textW = Math.max(...wrapped.lines.map(stepText));
    return { step: s, wrapped, textW, h: 22 + lineH * wrapped.lines.length };
  });
  const tops = rows.map((_, i) => headH + 14 + rows.slice(0, i).reduce((sum, r) => sum + r.h, 0));
  const height = headH + 22 + rows.reduce((sum, r) => sum + r.h, 0);
  const current = panel.steps[shown - 1];
  return (
    <figure className="ccpd-panel" ref={fig}>
      {panel.title && <figcaption className="ccpd-panel-title">{panel.title}</figcaption>}
      <svg className="ccpd-seq" width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label={panel.title ?? 'sequence'}>
        <defs>
          {tones.map((tone) => (
            <marker key={tone} id={`ccpd-arrow-${tone}`} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
              <path d="M0,0 L10,5 L0,10 z" className={`ccpd-fill-${tone}`} />
            </marker>
          ))}
        </defs>
        {panel.actors.map((a, i) => {
          const cx = x.get(a.id) ?? 0;
          const wrapped = actors[i] as Wrapped;
          return (
            <g key={a.id}>
              <line className="ccpd-lifeline" x1={cx} y1={headH} x2={cx} y2={height} />
              <rect className="ccpd-actor" x={cx - colW / 2 + 3} y={2} width={colW - 6} height={headH - 4} rx={6} />
              <Label
                className="ccpd-actor-label"
                x={cx}
                y={headH / 2 + 4 - ((wrapped.lines.length - 1) * lineH) / 2}
                wrapped={wrapped}
                full={a.label}
              />
            </g>
          );
        })}
        {rows.slice(0, shown).map((r, i) => (
          <StepMark key={i} row={r} y={tops[i] ?? 0} x={x} colW={colW} width={width} active={i === shown - 1} />
        ))}
      </svg>
      <div className="ccpd-caption" aria-live="polite">
        {current && <span dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(current.note ?? current.label) }} />}
      </div>
    </figure>
  );
}

function StepMark({
  row,
  y,
  x,
  colW,
  width,
  active,
}: {
  row: { step: Step; wrapped: Wrapped; textW: number; h: number };
  y: number;
  x: Map<string, number>;
  colW: number;
  width: number;
  active: boolean;
}) {
  const { step, wrapped, textW, h } = row;
  const tone = step.tone ?? 'default';
  const cls = `ccpd-step ccpd-tone-${tone}${active ? ' ccpd-active' : ''}`;
  const fx = x.get(step.from) ?? 0;
  const n = wrapped.lines.length;
  if (step.to === undefined || step.to === step.from) {
    const w = Math.max(colW - 6, textW + 16);
    const cx = Math.min(Math.max(fx, w / 2 + 2), width - w / 2 - 2);
    return (
      <g className={cls}>
        <rect className="ccpd-note" x={cx - w / 2} y={y + 4} width={w} height={h - 8} rx={5} />
        <Label className="ccpd-step-label" x={cx} y={y + h / 2 + 4 - ((n - 1) * lineH) / 2} wrapped={wrapped} full={step.label} />
      </g>
    );
  }
  const tx = x.get(step.to) ?? 0;
  const dir = tx > fx ? 1 : -1;
  const ly = y + h - 8;
  return (
    <g className={cls}>
      <line
        className={step.dashed ? 'ccpd-msg ccpd-dashed' : 'ccpd-msg'}
        x1={fx}
        y1={ly}
        x2={tx - dir * 2}
        y2={ly}
        pathLength={step.dashed ? undefined : 1}
        markerEnd={`url(#ccpd-arrow-${tone})`}
      />
      <Label
        className="ccpd-step-label"
        x={Math.min(Math.max((fx + tx) / 2, textW / 2 + 4), width - textW / 2 - 4)}
        y={ly - 7 - (n - 1) * lineH}
        wrapped={wrapped}
        full={step.label}
      />
    </g>
  );
}
