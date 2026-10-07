import { useEffect, useMemo, useState } from 'react';
import { renderInlineMarkdown, renderMarkdown, usePackState } from './host/present';
import type { PackComponentProps } from './host/present';
import { arrowHead, initialState, layoutMachine, pathOf, stateLabel, validateMachine } from './logic';
import type { Box, MachineSpec, MachineState, StateMark, TransitionMark } from './logic';
import { MockFrame } from './MockFrame';

interface MachineBlock extends MachineSpec {
  title?: string;
  caption?: string;
}

const stateMarkClass: Record<StateMark, string> = { '+': 'add', '-': 'del', '~': 'mod' };
const edgeMarkClass: Record<TransitionMark, string> = { '+': 'add', '-': 'del', '~': 'mod', '?': 'ask' };

const fontPx = 13;

function useMeasure(el: HTMLElement | null): (s: string) => number {
  const [fontsReady, setFontsReady] = useState(false);
  useEffect(() => {
    void document.fonts.ready.then(() => setFontsReady(true));
  }, []);
  return useMemo(() => {
    const ctx = document.createElement('canvas').getContext('2d');
    if (!ctx || !el) return (s: string) => s.length * fontPx * 0.6;
    ctx.font = `500 ${fontPx}px ${getComputedStyle(el).fontFamily}`;
    return (s: string) => ctx.measureText(s).width;
  }, [el, fontsReady]);
}

export function Machine({ block }: PackComponentProps) {
  const b = block as unknown as MachineBlock;
  const [root, setRoot] = useState<HTMLElement | null>(null);
  const measure = useMeasure(root);
  const [current, setCurrent] = usePackState<string>('state', initialState(b));
  const problems = validateMachine(b);
  const layout = useMemo(() => (problems.length > 0 ? null : layoutMachine(b, measure)), [b, measure, problems.length]);

  if (!layout) {
    return (
      <section className="ccpl-frame">
        {b.title && <header className="ccpl-title">{b.title}</header>}
        <div className="ccpl-error" role="alert">
          {problems.map((p) => (
            <div key={p}>{p}</div>
          ))}
        </div>
      </section>
    );
  }

  const byId = new Map(b.states.map((s) => [s.id, s]));
  const selected = byId.get(current) ?? byId.get(initialState(b))!;
  const hasScreens = b.states.some((s) => s.screen);

  return (
    <section className="ccpl-frame ccpl-machine" ref={setRoot}>
      <header className="ccpl-head">
        <span className="ccpl-kind">state machine</span>
        {b.title && <span className="ccpl-head-title">{b.title}</span>}
      </header>
      <div className="ccpl-canvas">
        <svg className="ccpl-mc" viewBox={`0 0 ${layout.W} ${layout.H}`} style={{ maxWidth: layout.W }} role="img" aria-label={b.title ?? 'state machine'}>
          {layout.routes.map((r) => {
            const t = b.transitions[r.index]!;
            const hot = t.from === selected.id;
            const cls = ['ccpl-edge', t.mark ? `ccpl-e-${edgeMarkClass[t.mark]}` : '', hot ? 'ccpl-hot' : ''].filter(Boolean).join(' ');
            return (
              <g key={r.index} className={cls}>
                <path className="ccpl-edge-line" d={pathOf(r.pts)} />
                <path className="ccpl-edge-head" d={arrowHead(r.pts[r.pts.length - 2]!, r.pts[r.pts.length - 1]!)} />
                <text className="ccpl-edge-label" x={r.lx} y={r.ly} textAnchor={r.anchor}>
                  {r.text}
                </text>
              </g>
            );
          })}
          {b.states.map((s) => (
            <StateNode key={s.id} state={s} box={layout.boxes[s.id]!} on={s.id === selected.id} onPick={() => setCurrent(s.id)} />
          ))}
        </svg>
      </div>
      <div className="ccpl-strip">
        <code className="ccpl-strip-id">{selected.id}</code>
        {selected.note && <span className="ccpl-strip-note" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(selected.note) }} />}
        {hasScreens && <span className="ccpl-strip-hint">Tap a state to see its screen</span>}
      </div>
      {selected.screen && (
        <div className="ccpl-state-screen">
          {'md' in selected.screen ? (
            <div className="prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(selected.screen.md) }} />
          ) : (
            <MockFrame key={selected.id} spec={selected.screen.mock} />
          )}
        </div>
      )}
      {b.caption && <div className="ccpl-caption" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(b.caption) }} />}
    </section>
  );
}

function StateNode({ state, box, on, onPick }: { state: MachineState; box: Box; on: boolean; onPick: () => void }) {
  const cls = ['ccpl-state', state.mark ? `ccpl-s-${stateMarkClass[state.mark]}` : '', state.final ? 'ccpl-final' : '', on ? 'ccpl-on' : '']
    .filter(Boolean)
    .join(' ');
  return (
    <g
      className={cls}
      role="button"
      tabIndex={0}
      aria-pressed={on}
      onClick={onPick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onPick();
        }
      }}
    >
      <rect x={box.x} y={box.y} width={box.w} height={box.h} rx={box.h / 2} />
      {state.final && <rect className="ccpl-final-ring" x={box.x + 3} y={box.y + 3} width={box.w - 6} height={box.h - 6} rx={(box.h - 6) / 2} />}
      <text x={box.cx} y={box.cy + 4.5} textAnchor="middle">
        {stateLabel(state)}
      </text>
    </g>
  );
}
