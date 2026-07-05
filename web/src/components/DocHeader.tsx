import { ConnectionFrame } from '@cc-interact/react';
import type { Doc } from '../schema';
import { StatsBar } from './StatsBar';
import { ThemeToggle } from './ThemeToggle';

export interface DocHeaderProps {
  doc: Doc;
  connected: boolean;
  peerPresent: boolean | null;
}

export function DocHeader({ doc, connected, peerPresent }: DocHeaderProps) {
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
          <ThemeToggle />
        </span>
      </div>
      {doc.intro && <p className="doc-intro">{doc.intro}</p>}
      {doc.stats && doc.stats.length > 0 && <StatsBar stats={doc.stats} />}
    </header>
  );
}
