// The pure TypeScript reducer. It mirrors internal/state/reduce.go field for
// field: document content and human verdicts are held separately and keyed by
// block id, so an agent re-upserting a block never clobbers a human's decision.
// Replaying the log from seq 0 reconstructs a fresh tab's state. Both reducers
// are driven by the same internal/state/testdata/*.json fixtures (reduce.test.ts).

import type { Block, Doc } from './schema';
import type {
  Closed,
  Decision,
  HumanEvent,
  Interaction,
  Interactions,
  PresentEvent,
  PresentState,
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
      feedback: {},
      replies: {},
      submitted: { value: false, revision: 0 },
      closed: { value: false },
    },
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
    case 'doc.replaced':
      return { ...state, doc: ev.payload.doc };
    case 'block.upserted':
      return {
        ...state,
        doc: { ...state.doc, blocks: upsert(state.doc.blocks, ev.payload.block, ev.payload.after) },
      };
    case 'block.removed':
      return { ...state, doc: { ...state.doc, blocks: remove(state.doc.blocks, ev.payload.id) } };
    case 'reply.created': {
      const { blockId, id, md } = ev.payload;
      return withInteractions(state, { replies: append(state.interactions.replies, blockId, { id, md }) });
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
        inputs: { ...state.interactions.inputs, [blockId]: { text } },
      });
    }
    case 'submit':
      return withInteractions(state, { submitted: { value: true, revision: ev.payload.revision } });
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
// append-only, so applying it here would double-count once the echo lands; it
// is deferred to the echo. Every last-write-wins interaction is idempotent
// under a re-apply, so the echo reconciles it cleanly.
export function applyInteraction(state: PresentState, interaction: Interaction): PresentState {
  if (interaction.type === 'feedback.created') return state;
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
