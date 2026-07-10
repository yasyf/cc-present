import type { RoundRecord } from '../events';

// WaitingPanel fills the gap between rounds; the subline recalls how the last
// round closed, or notes a fresh board.
export function WaitingPanel({ round, lastRound }: { round: number; lastRound: RoundRecord | undefined }) {
  const subline = !lastRound
    ? 'Waiting for the agent to add content'
    : lastRound.submittedRevision != null
      ? `Round ${lastRound.number} submitted · rev ${lastRound.submittedRevision}`
      : `Round ${lastRound.number} wrapped up`;

  return (
    <div className="waiting-panel">
      <div className="waiting-dots" aria-hidden>
        <span />
        <span />
        <span />
      </div>
      <div className="waiting-title">Waiting for round {round}</div>
      <div className="waiting-sub">{subline}</div>
    </div>
  );
}
