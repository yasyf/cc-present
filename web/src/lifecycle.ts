// The board's display phase, derived from reduced state. The reducer keeps no
// phase: between rounds the live round is empty by design, so an empty current
// round is the only "waiting" signal. closed wins and carries the still-open
// blocks so a mid-round close renders them read-only.

import type { Block } from './schema';
import type { PresentState, RoundRecord } from './events';

export type BoardPhase =
  | { kind: 'live'; blocks: Block[] }
  | { kind: 'waiting'; lastRound: RoundRecord | undefined }
  | { kind: 'closed'; blocks: Block[]; summary?: string };

// boardPhase reads the current round's live blocks off the reduced state.
export function boardPhase(state: PresentState): BoardPhase {
  const current = state.rounds.current;
  const blocks = state.doc.blocks.filter((b) => state.rounds.blockRounds[b.id] === current);
  if (state.interactions.closed.value) {
    return { kind: 'closed', blocks, summary: state.interactions.closed.summary };
  }
  if (blocks.length > 0) return { kind: 'live', blocks };
  return { kind: 'waiting', lastRound: state.rounds.history.at(-1) };
}
