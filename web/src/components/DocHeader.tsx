import type { Doc } from '../schema';
import type { ViewMode } from '../viewmode';
import { StatsBar } from './StatsBar';
import { ThemeToggle } from './ThemeToggle';

export interface DocHeaderProps {
  doc: Doc;
  round: number;
  connected: boolean;
  peerPresent: boolean | null;
  mode: ViewMode;
  onSetView: (mode: ViewMode) => void;
}

export function DocHeader({ doc, round, connected, peerPresent, mode, onSetView }: DocHeaderProps) {
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
