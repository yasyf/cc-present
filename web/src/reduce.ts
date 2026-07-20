// The pure TypeScript reducer. It mirrors internal/state/reduce.go field for
// field: document content and human verdicts are held separately and keyed by
// block id, so an agent re-upserting a block never clobbers a human's decision.
// Replaying the log from seq 0 reconstructs a fresh tab's state. Both reducers
// are driven by the same internal/state/testdata/*.json fixtures (reduce.test.ts).

import type { Block, Card, ChildBlock, Choice, Doc } from './schema';
import type {
  Annotation,
  Closed,
  Decision,
  HumanEvent,
  Interaction,
  Interactions,
  PresentEvent,
  PresentState,
  Revising,
  RoundRecord,
  Rounds,
  Selection,
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
      annotations: {},
      triage: {},
      submitted: { value: false, revision: 0 },
      closed: { value: false },
    },
    rounds: { current: 1, blockRounds: {}, history: [] },
    revising: { blockIds: [] },
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
// every later frame is a no-op. Framework frames in the shared log —
// channel.changed and the agent.* lifecycle — are skipped; any other unknown
// type is an error.
export function applyEvent(state: PresentState, ev: PresentEvent): PresentState {
  if (state.interactions.closed.value) return state;
  if (ev.type.startsWith('agent.')) return state;
  switch (ev.type) {
    case 'doc.replaced': {
      const doc = ev.payload.doc;
      const blockRounds: Record<string, number> = {};
      for (const b of doc.blocks) blockRounds[b.id] = state.rounds.current;
      return { ...state, doc, rounds: { ...state.rounds, blockRounds }, revising: { blockIds: [] } };
    }
    case 'block.upserted': {
      const { block, after } = ev.payload;
      const topId = enclosingTopId(state.doc.blocks, block.id, after);
      const blocks = upsert(state.doc.blocks, block, after);
      const blockRounds = { ...state.rounds.blockRounds, [topId]: state.rounds.current };
      const revising = revisingOnUpsert(state.revising, topId);
      return { ...state, doc: { ...state.doc, blocks }, rounds: { ...state.rounds, blockRounds }, revising };
    }
    case 'block.removed': {
      const loc = locate(state.doc.blocks, ev.payload.id);
      if (!loc || loc.kind === 'option-visual') return state;
      const blocks = remove(state.doc.blocks, ev.payload.id);
      const blockRounds = { ...state.rounds.blockRounds };
      if (loc.kind === 'card-child') blockRounds[loc.topId] = state.rounds.current;
      else delete blockRounds[loc.topId];
      const revising = revisingOnRemove(state.revising, loc.topId);
      return { ...state, doc: { ...state.doc, blocks }, rounds: { ...state.rounds, blockRounds }, revising };
    }
    case 'reply.created': {
      const { blockId, id, md } = ev.payload;
      return withInteractions(state, { replies: append(state.interactions.replies, blockId, { id, md }) });
    }
    case 'round.started': {
      const { title } = ev.payload;
      const cleared = { ...state, revising: { blockIds: [] } };
      const advanced = isDirty(cleared) ? closeRound(cleared, undefined) : cleared;
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
      const { blockId, optionIds, other } = ev.payload;
      const selection: Selection = { optionIds: optionIds ?? [] };
      if (other) selection.other = other;
      return withInteractions(state, { choices: { ...state.interactions.choices, [blockId]: selection } });
    }
    case 'feedback.created': {
      const { blockId, id, text } = ev.payload;
      return withInteractions(state, { feedback: append(state.interactions.feedback, blockId, { id, text }) });
    }
    case 'annotation.created': {
      const { blockId, id, anchor, text, quote } = ev.payload;
      const ann: Annotation = { id, anchor, text, quote };
      const list = state.interactions.annotations[blockId] ?? [];
      const idx = list.findIndex((a) => a.id === id);
      const next = idx === -1 ? [...list, ann] : list.map((a, i) => (i === idx ? ann : a));
      return withInteractions(state, { annotations: { ...state.interactions.annotations, [blockId]: next } });
    }
    case 'annotation.removed': {
      const { blockId, id } = ev.payload;
      const list = state.interactions.annotations[blockId];
      if (!list || !list.some((a) => a.id === id)) return state;
      // A removal that empties the list leaves the (now empty) list under the
      // block key, mirroring the Go splice — the key is never deleted.
      return withInteractions(state, {
        annotations: { ...state.interactions.annotations, [blockId]: list.filter((a) => a.id !== id) },
      });
    }
    case 'triage.decided': {
      const { blockId, verdicts } = ev.payload;
      const block = { ...(state.interactions.triage[blockId] ?? {}) };
      for (const [itemId, entry] of Object.entries(verdicts)) {
        if (!VALID_VERDICTS.has(entry.verdict)) throw new Error(`invalid verdict "${entry.verdict}"`);
        if (entry.verdict === 'cleared') {
          delete block[itemId];
          continue;
        }
        const decision: Decision = { verdict: entry.verdict };
        if (entry.note) decision.note = entry.note;
        block[itemId] = decision;
      }
      const triage = { ...state.interactions.triage };
      // An emptied inner map deletes the block key; a non-empty one replaces it.
      if (Object.keys(block).length === 0) delete triage[blockId];
      else triage[blockId] = block;
      return withInteractions(state, { triage });
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
      const submitted = {
        ...withInteractions(state, { submitted: { value: true, revision } }),
        revising: { blockIds: [] },
      };
      if (!isDirty(submitted)) return submitted;
      const closed = closeRound(submitted, revision);
      const rounds = { ...closed.rounds };
      delete rounds.currentTitle;
      return { ...closed, rounds };
    }
    case 'revising.changed': {
      const { blockIds, note } = ev.payload;
      const revising: Revising = { blockIds: blockIds ?? [] };
      if (note) revising.note = note;
      return { ...state, revising };
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
        payload: { blockId: interaction.blockId, optionIds: interaction.optionIds, other: interaction.other },
      };
    case 'feedback.created':
      return {
        origin: 'human',
        type: 'feedback.created',
        seq,
        payload: { id: interaction.id, blockId: interaction.blockId, text: interaction.text },
      };
    case 'annotation.created':
      return {
        origin: 'human',
        type: 'annotation.created',
        seq,
        payload: {
          id: interaction.id,
          blockId: interaction.blockId,
          anchor: interaction.anchor,
          text: interaction.text,
          quote: interaction.quote,
        },
      };
    case 'annotation.removed':
      return {
        origin: 'human',
        type: 'annotation.removed',
        seq,
        payload: { id: interaction.id, blockId: interaction.blockId },
      };
    case 'triage.decided':
      return {
        origin: 'human',
        type: 'triage.decided',
        seq,
        payload: { blockId: interaction.blockId, verdicts: interaction.verdicts },
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

// revisingOnUpsert drops id as its revision lands, draining the note whenever the
// set empties — including an upsert while the set is already empty, the doc-level
// note's completion signal.
function revisingOnUpsert(revising: Revising, id: string): Revising {
  const blockIds = revising.blockIds.filter((b) => b !== id);
  if (blockIds.length === 0) return { blockIds: [] };
  return revising.note ? { blockIds, note: revising.note } : { blockIds };
}

// revisingOnRemove drops id as its block is removed, draining the note only when
// removing a tracked id empties the set; a removal while the set is already empty
// leaves a doc-level note untouched.
function revisingOnRemove(revising: Revising, id: string): Revising {
  const had = revising.blockIds.includes(id);
  const blockIds = revising.blockIds.filter((b) => b !== id);
  if (had && blockIds.length === 0) return { blockIds: [] };
  return revising.note ? { blockIds, note: revising.note } : { blockIds };
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
    feedback: filterClone(state.interactions.feedback, ids, (v) => [...v]),
    annotations: filterClone(state.interactions.annotations, ids, (v) => [...v]),
    triage: filterClone(state.interactions.triage, ids, (v) => ({ ...v })),
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

// filterClone projects a keyed interaction map to the round's block ids, cloning
// each retained value through `clone` so a later mutation of the live interaction
// state cannot reach a round's frozen snapshot (lists and triage inner maps are
// references). Generalizes filterMap for the append-only / nested cases.
function filterClone<T>(map: Record<string, T>, ids: Set<string>, clone: (v: T) => T): Record<string, T> {
  const out: Record<string, T> = {};
  for (const [id, v] of Object.entries(map)) if (ids.has(id)) out[id] = clone(v);
  return out;
}

// A block's resolved position, mirroring doc.Location: whether it sits at the top
// level, as a card child, or as a choice option's visual, plus the id of the
// enclosing top-level block whose round and revising bookkeeping the block keys.
type LocationKind = 'top-level' | 'card-child' | 'option-visual';

interface Location {
  kind: LocationKind;
  topId: string;
}

// locate mirrors doc.Locate: it finds `id` anywhere the document registers a
// block — a top-level block, a card child, or a choice option's visual — and
// reports the enclosing top-level id. undefined when no block carries `id`.
function locate(blocks: readonly Block[], id: string): Location | undefined {
  for (const b of blocks) {
    if (b.id === id) return { kind: 'top-level', topId: b.id };
    if (b.type === 'choice' && hasOptionVisual(b, id)) return { kind: 'option-visual', topId: b.id };
    if (b.type === 'card') {
      for (const child of b.children) {
        if (child.id === id) return { kind: 'card-child', topId: b.id };
        if (child.type === 'choice' && hasOptionVisual(child, id)) return { kind: 'option-visual', topId: b.id };
      }
    }
  }
  return undefined;
}

function hasOptionVisual(choice: Choice, id: string): boolean {
  return choice.options.some((o) => o.visual?.id === id);
}

// enclosingTopId resolves the top-level id a block.upserted keys its round and
// revising bookkeeping on, mirroring the reducer's Go logic: the block's own
// enclosing top when it already exists, else the card one level up when `after`
// names a card child, else the block's own id (a new top-level append/insert).
function enclosingTopId(blocks: readonly Block[], id: string, after: string | undefined): string {
  const existing = locate(blocks, id);
  if (existing) return existing.topId;
  if (after !== undefined) {
    const afterLoc = locate(blocks, after);
    if (afterLoc?.kind === 'card-child') return afterLoc.topId;
  }
  return id;
}

// upsert mirrors doc.UpsertBlocks: an existing id (top-level or card child)
// replaces the block where it lives, order preserved; a new id lands after
// `after` (top-level or as a new child of the card holding it), else appends at
// the top level.
function upsert(blocks: readonly Block[], block: Block, after: string | undefined): Block[] {
  const idx = blocks.findIndex((b) => b.id === block.id);
  if (idx !== -1) {
    const next = blocks.slice();
    next[idx] = block;
    return next;
  }
  for (const [i, b] of blocks.entries()) {
    if (b.type !== 'card') continue;
    const ci = b.children.findIndex((c) => c.id === block.id);
    if (ci !== -1) {
      const children = b.children.slice();
      children[ci] = block as ChildBlock;
      return withCardAt(blocks, i, b, children);
    }
  }
  if (after !== undefined) {
    const afterIdx = blocks.findIndex((b) => b.id === after);
    if (afterIdx !== -1) {
      const next = blocks.slice();
      next.splice(afterIdx + 1, 0, block);
      return next;
    }
    for (const [i, b] of blocks.entries()) {
      if (b.type !== 'card') continue;
      const ci = b.children.findIndex((c) => c.id === after);
      if (ci !== -1) {
        const children = b.children.slice();
        children.splice(ci + 1, 0, block as ChildBlock);
        return withCardAt(blocks, i, b, children);
      }
    }
  }
  return [...blocks, block];
}

// remove mirrors doc.RemoveBlock: it splices `id` from the top level or from the
// card that holds it, copying the card and its children rather than mutating in
// place; an id no block carries leaves the document unchanged.
function remove(blocks: readonly Block[], id: string): Block[] {
  const idx = blocks.findIndex((b) => b.id === id);
  if (idx !== -1) {
    const next = blocks.slice();
    next.splice(idx, 1);
    return next;
  }
  for (const [i, b] of blocks.entries()) {
    if (b.type !== 'card') continue;
    const ci = b.children.findIndex((c) => c.id === id);
    if (ci !== -1) {
      const children = b.children.slice();
      children.splice(ci, 1);
      return withCardAt(blocks, i, b, children);
    }
  }
  return blocks.slice();
}

// withCardAt shallow-copies the block slice, swapping the card at index `i` for a
// copy carrying the new children — the card object and its children array are
// never mutated in place, since the result feeds React state.
function withCardAt(blocks: readonly Block[], i: number, card: Card, children: ChildBlock[]): Block[] {
  const next = blocks.slice();
  next[i] = { ...card, children };
  return next;
}
