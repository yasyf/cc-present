import { ConnectionFrame } from '@cc-interact/react';
import type { Doc } from '../schema';
import type { ViewMode } from '../viewmode';
import { StatsBar } from './StatsBar';
import { ThemeToggle } from './ThemeToggle';

export interface DocHeaderProps {
  doc: Doc;
  connected: boolean;
  peerPresent: boolean | null;
  mode: ViewMode;
  onSetView: (mode: ViewMode) => void;
}

export function DocHeader({ doc, connected, peerPresent, mode, onSetView }: DocHeaderProps) {
  return (
    <header className="doc-header">
      <div className="doc-header-top">
        <h1 className="doc-title">{doc.title}</h1>
        <span className="doc-conn">
          {peerPresent !== null && (
            <span className={`peer peer-${peerPresent ? 'on' : 'off'}`}>
              {peerPresent ? 'agent online' : 'agent offline'}
            </span>
          )}
          <ConnectionFrame connected={connected} />
          <ViewToggle mode={mode} onSetView={onSetView} />
          <ThemeToggle />
        </span>
      </div>
      {doc.intro && <p className="doc-intro">{doc.intro}</p>}
      {doc.stats && doc.stats.length > 0 && <StatsBar stats={doc.stats} />}
    </header>
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
