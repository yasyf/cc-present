import { useRef } from 'react';
import { renderInlineMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';
import { panelProblems, stepCount } from './logic';
import type { Panel, Step, Tone } from './logic';
import { Controls, usePlayer } from './player';

interface SequenceBlock {
  title?: string;
  autoplay?: boolean;
  intervalMs?: number;
  panels: Panel[];
}

const colW = 150;
const headH = 34;
const rowH = 44;
const top = headH + 16;
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

function SequencePanel({ panel, shown }: { panel: Panel; shown: number }) {
  const x = new Map(panel.actors.map((a, i) => [a.id, colW * (i + 0.5)]));
  const width = colW * panel.actors.length;
  const height = top + rowH * panel.steps.length + 8;
  const current = panel.steps[shown - 1];
  return (
    <figure className="ccpd-panel">
      {panel.title && <figcaption className="ccpd-panel-title">{panel.title}</figcaption>}
      <svg className="ccpd-seq" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={panel.title ?? 'sequence'}>
        <defs>
          {tones.map((tone) => (
            <marker key={tone} id={`ccpd-arrow-${tone}`} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">
              <path d="M0,0 L10,5 L0,10 z" className={`ccpd-fill-${tone}`} />
            </marker>
          ))}
        </defs>
        {panel.actors.map((a) => {
          const cx = x.get(a.id) ?? 0;
          return (
            <g key={a.id}>
              <line className="ccpd-lifeline" x1={cx} y1={headH} x2={cx} y2={height} />
              <rect className="ccpd-actor" x={cx - colW / 2 + 10} y={2} width={colW - 20} height={headH - 4} rx={6} />
              <text className="ccpd-actor-label" x={cx} y={headH / 2 + 4} textAnchor="middle">
                {a.label}
              </text>
            </g>
          );
        })}
        {panel.steps.slice(0, shown).map((s, i) => (
          <StepMark key={i} step={s} y={top + rowH * i} x={x} active={i === shown - 1} />
        ))}
      </svg>
      <div className="ccpd-caption" aria-live="polite">
        {current && <span dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(current.note ?? current.label) }} />}
      </div>
    </figure>
  );
}

function StepMark({ step, y, x, active }: { step: Step; y: number; x: Map<string, number>; active: boolean }) {
  const tone = step.tone ?? 'default';
  const cls = `ccpd-step ccpd-tone-${tone}${active ? ' ccpd-active' : ''}`;
  const fx = x.get(step.from) ?? 0;
  if (step.to === undefined || step.to === step.from) {
    const w = colW - 24;
    return (
      <g className={cls}>
        <rect className="ccpd-note" x={fx - w / 2} y={y + 6} width={w} height={rowH - 14} rx={5} />
        <text className="ccpd-step-label" x={fx} y={y + rowH / 2 + 3} textAnchor="middle">
          {step.label}
        </text>
      </g>
    );
  }
  const tx = x.get(step.to) ?? 0;
  const dir = tx > fx ? 1 : -1;
  const ly = y + rowH - 12;
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
      <text className="ccpd-step-label" x={(fx + tx) / 2} y={ly - 7} textAnchor="middle">
        {step.label}
      </text>
    </g>
  );
}
