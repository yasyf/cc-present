import type { Doc } from '../schema';
import type { ViewMode } from '../viewmode';
import { Button } from './Button';
import { StatsBar } from './StatsBar';
import { ThemeToggle } from './ThemeToggle';

export interface DocHeaderProps {
  doc: Doc;
  round: number;
  connected: boolean;
  peerPresent: boolean | null;
  mode: ViewMode;
  onSetView: (mode: ViewMode) => void;
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
  mode,
  onSetView,
  commentCount,
  onOpenComments,
}: DocHeaderProps) {
  return (
    <header className="doc-header">
      <p className="doc-eyebrow">CC-PRESENT · ROUND {round}</p>
      <div className="doc-header-top">
        <h1 className="doc-title">{doc.title}</h1>
        <span className="doc-conn">
          {peerPresent !== null && (
            <span className={`peer peer-${peerPresent ? 'on' : 'off'}`}>
              {peerPresent ? 'agent on the line' : 'agent away'}
            </span>
          )}
          <WireLamp connected={connected} />
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
          <ThemeToggle />
        </span>
      </div>
      {doc.intro && <p className="doc-intro">{doc.intro}</p>}
      {doc.stats && doc.stats.length > 0 && <StatsBar stats={doc.stats} />}
    </header>
  );
}

function WireLamp({ connected }: { connected: boolean }) {
  const label = connected ? 'Connected to the session' : 'Reconnecting to the session';
  return (
    <span className={`wire-lamp wire-${connected ? 'live' : 'down'}`} role="status" aria-label={label}>
      <span className="wire-dot" aria-hidden />
      {connected ? 'LIVE' : 'RECONNECTING'}
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
