import { useEffect, useRef } from 'react';
import type { Doc } from '../schema';
import type { ViewMode } from '../viewmode';
import { HeaderMenu } from './HeaderMenu';
import { StatsBar } from './StatsBar';
import { StepJump } from './StepJump';
import type { StepJumpProps } from './StepJump';

export interface DocHeaderProps {
  doc: Doc;
  round: number;
  connected: boolean;
  peerPresent: boolean | null;
  closed: boolean;
  mode: ViewMode;
  onSetView: (mode: ViewMode) => void;
  step?: StepJumpProps;
  // The margin-rail comments trigger, shown only below the rail breakpoint where
  // the rail becomes a sheet; both are provided together or not at all.
  commentCount?: number;
  onOpenComments?: () => void;
}

export function DocHeader({
  doc,
  round,
  connected,
  peerPresent,
  closed,
  mode,
  onSetView,
  step,
  commentCount,
  onOpenComments,
}: DocHeaderProps) {
  const compact = mode === 'focus';
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!compact) return;
    const header = ref.current!;
    const root = document.documentElement;
    const observer = new ResizeObserver(() => root.style.setProperty('--doc-header-h', `${header.offsetHeight}px`));
    observer.observe(header);
    return () => {
      observer.disconnect();
      root.style.removeProperty('--doc-header-h');
    };
  }, [compact]);

  return (
    <header className="doc-header" ref={ref} data-compact={compact || undefined}>
      {!compact && <p className="doc-eyebrow">CC-PRESENT · ROUND {round}</p>}
      <div className="doc-header-top">
        <div className="doc-heading">
          <h1 className="doc-title" title={doc.title}>
            {doc.title}
          </h1>
          <StatusDot state={wireState(closed, connected, peerPresent)} />
        </div>
        <span className="doc-controls">
          {step && <StepJump {...step} />}
          <ViewToggle mode={mode} onSetView={onSetView} />
          <HeaderMenu commentCount={commentCount} onOpenComments={onOpenComments} />
        </span>
      </div>
      {!compact && doc.intro && <p className="doc-intro">{doc.intro}</p>}
      {!compact && doc.stats && doc.stats.length > 0 && <StatsBar stats={doc.stats} />}
    </header>
  );
}

export type WireState = 'present' | 'away' | 'live' | 'down' | 'closed';

export function wireState(closed: boolean, connected: boolean, peerPresent: boolean | null): WireState {
  if (closed) return 'closed';
  if (!connected) return 'down';
  if (peerPresent === null) return 'live';
  return peerPresent ? 'present' : 'away';
}

const WIRE_LABEL: Record<WireState, string> = {
  present: 'Agent on the line · live',
  away: 'Agent away · live',
  live: 'Connected to the session',
  down: 'Reconnecting to the session',
  closed: 'This board is closed',
};

function StatusDot({ state }: { state: WireState }) {
  return (
    <span
      className={`wire-dot wire-${state}`}
      role="status"
      aria-label={WIRE_LABEL[state]}
      title={WIRE_LABEL[state]}
    />
  );
}

function ViewToggle({ mode, onSetView }: { mode: ViewMode; onSetView: (mode: ViewMode) => void }) {
  return (
    <span className="view-toggle" role="group" aria-label="view mode">
      <button
        type="button"
        className={`view-seg${mode === 'focus' ? ' on' : ''}`}
        aria-pressed={mode === 'focus'}
        onClick={() => onSetView('focus')}
      >
        Focus
      </button>
      <button
        type="button"
        className={`view-seg${mode === 'board' ? ' on' : ''}`}
        aria-pressed={mode === 'board'}
        onClick={() => onSetView('board')}
      >
        Board
      </button>
    </span>
  );
}
