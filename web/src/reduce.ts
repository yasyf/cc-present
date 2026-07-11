// The pure TypeScript reducer. It mirrors internal/state/reduce.go field for
// field: document content and human verdicts are held separately and keyed by
// block id, so an agent re-upserting a block never clobbers a human's decision.
// Replaying the log from seq 0 reconstructs a fresh tab's state. Both reducers
// are driven by the same internal/state/testdata/*.json fixtures (reduce.test.ts).

import type { Block, Doc } from './schema';
import type {
  Closed,
  Decision,
  Feedback,
  HumanEvent,
  Interaction,
  Interactions,
  PresentEvent,
  PresentState,
  RoundRecord,
  Rounds,
  Verdict,
} from './events';

const VALID_VERDICTS: ReadonlySet<Verdict> = new Set(['approved', 'rejected', 'cleared']);

export function emptyDoc(): Doc {
  return { version: 1, title: '', blocks: [] };
}

export function emptyState(): PresentState {
  return {
    doc: emptyDoc(),
    interactions: {
      decisions: {},
      choices: {},
      inputs: {},
      packs: {},
      feedback: {},
      replies: {},
      submitted: { value: false, revision: 0 },
      closed: { value: false },
    },
    rounds: { current: 1, blockRounds: {}, history: [] },
  };
}

// reduce folds the log into a State. Events are processed in ascending seq
// order; last-write-wins interactions resolve by that order. The document
// starts empty, so a block.upserted before any doc.replaced appends to it.
// present.closed is terminal: any event ordered after it is a no-op.
export function reduce(events: readonly PresentEvent[]): PresentState {
  const ordered = [...events].sort((a, b) => a.seq - b.seq);
  let state = emptyState();
  for (const ev of ordered) state = applyEvent(state, ev);
  return state;
}

// applyEvent applies a single frame. present.closed is terminal: once closed,
// every later frame is a no-op, so a human interaction that races the close
// never poisons replay. channel.changed presence frames are skipped; any other
// unknown type is an error.
export function applyEvent(state: PresentState, ev: PresentEvent): PresentState {
  if (state.interactions.closed.value) return state;
  switch (ev.type) {
    case 'doc.replaced': {
      const doc = ev.payload.doc;
      const blockRounds: Record<string, number> = {};
      for (const b of doc.blocks) blockRounds[b.id] = state.rounds.current;
      return { ...state, doc, rounds: { ...state.rounds, blockRounds } };
    }
    case 'block.upserted': {
      const blocks = upsert(state.doc.blocks, ev.payload.block, ev.payload.after);
      const blockRounds = { ...state.rounds.blockRounds, [ev.payload.block.id]: state.rounds.current };
      return { ...state, doc: { ...state.doc, blocks }, rounds: { ...state.rounds, blockRounds } };
    }
    case 'block.removed': {
      const blocks = remove(state.doc.blocks, ev.payload.id);
      const blockRounds = { ...state.rounds.blockRounds };
      delete blockRounds[ev.payload.id];
      return { ...state, doc: { ...state.doc, blocks }, rounds: { ...state.rounds, blockRounds } };
    }
    case 'reply.created': {
      const { blockId, id, md } = ev.payload;
      return withInteractions(state, { replies: append(state.interactions.replies, blockId, { id, md }) });
    }
    case 'round.started': {
      const { title } = ev.payload;
      const advanced = isDirty(state) ? closeRound(state, undefined) : state;
      const rounds = { ...advanced.rounds };
      if (title) rounds.currentTitle = title;
      else delete rounds.currentTitle;
      return { ...advanced, rounds };
    }
    case 'present.closed': {
      const closed: Closed = { value: true };
      if (ev.payload.summary) closed.summary = ev.payload.summary;
      return withInteractions(state, { closed });
    }
    case 'decision.created': {
      const { blockId, verdict, note } = ev.payload;
      if (!VALID_VERDICTS.has(verdict)) throw new Error(`invalid verdict "${verdict}"`);
      const decisions = { ...state.interactions.decisions };
      if (verdict === 'cleared') {
        delete decisions[blockId];
      } else {
        const decision: Decision = { verdict };
        if (note) decision.note = note;
        decisions[blockId] = decision;
      }
      return withInteractions(state, { decisions });
    }
    case 'choice.selected': {
      const { blockId, optionIds } = ev.payload;
      return withInteractions(state, {
        choices: { ...state.interactions.choices, [blockId]: { optionIds: optionIds ?? [] } },
      });
    }
    case 'feedback.created': {
      const { blockId, id, text } = ev.payload;
      return withInteractions(state, { feedback: append(state.interactions.feedback, blockId, { id, text }) });
    }
    case 'input.submitted': {
      const { blockId, text } = ev.payload;
      return withInteractions(state, {
        inputs: { ...state.interactions.inputs, [blockId]: { text, round: inputRound(state, blockId) } },
      });
    }
    case 'pack.interaction': {
      const { blockId, payload } = ev.payload;
      return withInteractions(state, {
        packs: { ...state.interactions.packs, [blockId]: { payload } },
      });
    }
    case 'submit': {
      const { revision } = ev.payload;
      const submitted = withInteractions(state, { submitted: { value: true, revision } });
      if (!isDirty(submitted)) return submitted;
      const closed = closeRound(submitted, revision);
      const rounds = { ...closed.rounds };
      delete rounds.currentTitle;
      return { ...closed, rounds };
    }
    case 'channel.changed':
      return state;
    default: {
      ev satisfies never;
      throw new Error(`unknown event type "${(ev as { type: string }).type}"`);
    }
  }
}

// applyInteraction applies a human interaction optimistically, before the SSE
// echo arrives, using the same reduction the log replay uses. Feedback is
// append-only, so applying it here would double-count once the echo lands; and a
// submit optimistically advances the round, which a failed POST could never walk
// back — stranding the UI in waiting with no live blocks and no way to retry.
// Both are deferred to the echo. Every last-write-wins interaction is idempotent
// under a re-apply, so the echo reconciles it cleanly.
export function applyInteraction(state: PresentState, interaction: Interaction): PresentState {
  if (interaction.type === 'feedback.created' || interaction.type === 'submit') return state;
  return applyEvent(state, interactionEvent(interaction));
}

function interactionEvent(interaction: Interaction): HumanEvent {
  const seq = Number.MAX_SAFE_INTEGER;
  switch (interaction.type) {
    case 'decision.created':
      return {
        origin: 'human',
        type: 'decision.created',
        seq,
        payload: { blockId: interaction.blockId, verdict: interaction.verdict, note: interaction.note },
      };
    case 'choice.selected':
      return {
        origin: 'human',
        type: 'choice.selected',
        seq,
        payload: { blockId: interaction.blockId, optionIds: interaction.optionIds },
      };
    case 'feedback.created':
      return {
        origin: 'human',
        type: 'feedback.created',
        seq,
        payload: { id: interaction.id, blockId: interaction.blockId, text: interaction.text },
      };
    case 'input.submitted':
      return {
        origin: 'human',
        type: 'input.submitted',
        seq,
        payload: { blockId: interaction.blockId, text: interaction.text },
      };
    case 'pack.interaction':
      return {
        origin: 'human',
        type: 'pack.interaction',
        seq,
        payload: { blockId: interaction.blockId, payload: interaction.payload },
      };
    case 'submit':
      return { origin: 'human', type: 'submit', seq, payload: { revision: interaction.revision } };
  }
}

function withInteractions(state: PresentState, patch: Partial<Interactions>): PresentState {
  return { ...state, interactions: { ...state.interactions, ...patch } };
}

function append<T>(map: Record<string, T[]>, key: string, item: T): Record<string, T[]> {
  return { ...map, [key]: [...(map[key] ?? []), item] };
}

function isDirty(state: PresentState): boolean {
  return state.doc.blocks.some((b) => state.rounds.blockRounds[b.id] === state.rounds.current);
}

// closeRound appends a frozen snapshot of the current round and advances current.
// The caller owns currentTitle: submit clears it, round.started sets the next.
function closeRound(state: PresentState, revision: number | undefined): PresentState {
  const cur = state.rounds.current;
  const blocks = state.doc.blocks.filter((b) => state.rounds.blockRounds[b.id] === cur);
  const ids = idsOf(blocks);
  const record: RoundRecord = {
    number: cur,
    blocks: [...blocks],
    decisions: filterMap(state.interactions.decisions, ids),
    choices: filterMap(state.interactions.choices, ids),
    inputs: filterMap(state.interactions.inputs, ids),
    packs: filterMap(state.interactions.packs, ids),
    feedback: filterFeedback(state.interactions.feedback, ids),
  };
  if (state.rounds.currentTitle) record.title = state.rounds.currentTitle;
  if (revision !== undefined) record.submittedRevision = revision;
  const rounds: Rounds = {
    ...state.rounds,
    current: cur + 1,
    history: [...state.rounds.history, record],
  };
  return { ...state, rounds };
}

// topLevelRound resolves the round of the top-level block enclosing `id`: the
// block itself when top-level, else the card one level up that contains it,
// mirroring idsOf's one-level child resolution. undefined when no block in the
// doc owns the id.
export function topLevelRound(state: PresentState, id: string): number | undefined {
  const { blockRounds, current } = state.rounds;
  for (const b of state.doc.blocks) {
    if (b.id === id) return blockRounds[id] ?? current;
    if (b.type === 'card') {
      for (const child of b.children) {
        if (child.id === id) return blockRounds[b.id] ?? current;
      }
    }
  }
  return undefined;
}

// inputRound is the round an input value belongs to; an orphaned interaction with
// no block in the doc falls back to the current round so the reduction stays total.
function inputRound(state: PresentState, id: string): number {
  return topLevelRound(state, id) ?? state.rounds.current;
}

// idsOf collects the ids of a block slice plus one level of card children,
// mirroring where interactive blocks may nest (see SubmitBar's flatten).
function idsOf(blocks: readonly Block[]): Set<string> {
  const ids = new Set<string>();
  for (const b of blocks) {
    ids.add(b.id);
    if (b.type === 'card') for (const child of b.children) ids.add(child.id);
  }
  return ids;
}

function filterMap<T>(map: Record<string, T>, ids: Set<string>): Record<string, T> {
  const out: Record<string, T> = {};
  for (const [id, v] of Object.entries(map)) if (ids.has(id)) out[id] = v;
  return out;
}

function filterFeedback(map: Record<string, Feedback[]>, ids: Set<string>): Record<string, Feedback[]> {
  const out: Record<string, Feedback[]> = {};
  for (const [id, v] of Object.entries(map)) if (ids.has(id)) out[id] = [...v];
  return out;
}

function upsert(blocks: readonly Block[], block: Block, after: string | undefined): Block[] {
  const idx = blocks.findIndex((b) => b.id === block.id);
  if (idx !== -1) {
    const next = blocks.slice();
    next[idx] = block;
    return next;
  }
  if (after !== undefined) {
    const afterIdx = blocks.findIndex((b) => b.id === after);
    if (afterIdx !== -1) {
      const next = blocks.slice();
      next.splice(afterIdx + 1, 0, block);
      return next;
    }
  }
  return [...blocks, block];
}

function remove(blocks: readonly Block[], id: string): Block[] {
  const idx = blocks.findIndex((b) => b.id === id);
  if (idx === -1) return blocks.slice();
  const next = blocks.slice();
  next.splice(idx, 1);
  return next;
}
