import { useEffect, useRef } from 'react';
import type { Doc } from '../schema';
import type { ViewMode } from '../viewmode';
import { useKeyboardApi } from '../keyboard';
import { Button } from './Button';
import { StatsBar } from './StatsBar';
import { StepJump } from './StepJump';
import type { StepJumpProps } from './StepJump';
import { ThemeToggle } from './ThemeToggle';

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
  const { openHelp } = useKeyboardApi();
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
        <h1 className="doc-title" title={compact ? doc.title : undefined}>
          {doc.title}
        </h1>
        {compact && step && <StepJump {...step} />}
        <span className="doc-conn">
          {peerPresent !== null && (
            <span className={`peer peer-${peerPresent ? 'on' : 'off'}`}>
              {peerPresent ? 'agent on the line' : 'agent away'}
            </span>
          )}
          <WireLamp state={closed ? 'closed' : connected ? 'live' : 'down'} />
          {onOpenComments && (
            <Button
              variant="ghost"
              size="sm"
              className="rail-toggle"
              aria-label={`Comments${commentCount ? ` (${commentCount})` : ''}`}
              onClick={onOpenComments}
            >
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" aria-hidden focusable="false">
                <path d="M4 5h16v11H8l-4 4V5z" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
              </svg>
              {commentCount ? commentCount : 'Notes'}
            </Button>
          )}
          <ViewToggle mode={mode} onSetView={onSetView} />
          <Button variant="ghost" size="sm" className="help-toggle" aria-keyshortcuts="?" onClick={openHelp}>
            Shortcuts <kbd aria-hidden>?</kbd>
          </Button>
          <ThemeToggle />
        </span>
      </div>
      {!compact && doc.intro && <p className="doc-intro">{doc.intro}</p>}
      {!compact && doc.stats && doc.stats.length > 0 && <StatsBar stats={doc.stats} />}
    </header>
  );
}

type WireState = 'live' | 'down' | 'closed';

const WIRE: Record<WireState, { text: string; label: string }> = {
  live: { text: 'LIVE', label: 'Connected to the session' },
  down: { text: 'RECONNECTING', label: 'Reconnecting to the session' },
  closed: { text: 'CLOSED', label: 'This board is closed' },
};

function WireLamp({ state }: { state: WireState }) {
  return (
    <span className={`wire-lamp wire-${state}`} role="status" aria-label={WIRE[state].label}>
      <span className="wire-dot" aria-hidden />
      {WIRE[state].text}
    </span>
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
