// The append-only event log and the interaction cache it reduces to. Each
// PresentEvent envelope matches the fixture shape the Go reducer consumes
// ({ origin, type, seq, payload }); the payload types match the Go JSON exactly.
// The reducer implementation lives in reduce.ts and is fixture-tested against
// the same internal/state/testdata/*.json fixtures via vitest.

import type { Block, Doc } from './schema';

export type Origin = 'agent' | 'human';

export type Verdict = 'approved' | 'rejected' | 'cleared';

export const EVENT_SCHEMA_VERSION = 1 as const;

type PersistedPayload<T extends string, P> = P & { schemaVersion: typeof EVENT_SCHEMA_VERSION; type: T };

// --- Agent-origin payloads ---

export interface DocReplacedPayload {
  doc: Doc;
  revision: number;
  // Ids whose prior round stamp is restored after every top-level block is
  // stamped into the current round; an absent key stamps every block.
  retained?: string[];
}

export interface BlockUpsertedPayload {
  block: Block;
  // Position a newly inserted block after this id; ignored when replacing an
  // existing block, and an absent or unknown id appends.
  after?: string;
}

export interface BlockRemovedPayload {
  id: string;
}

export interface ReplyCreatedPayload {
  id: string;
  blockId: string;
  md: string;
}

export interface PresentClosedPayload {
  summary?: string;
}

export interface RoundStartedPayload {
  title?: string;
}

// --- Human-origin payloads ---

export interface DecisionCreatedPayload {
  blockId: string;
  verdict: Verdict;
  note?: string;
}

export interface ChoiceSelectedPayload {
  blockId: string;
  optionIds: string[];
  // A free-text write-in outside the authored option set; may stand alone
  // (single-select) or coexist with optionIds (multi-select).
  other?: string;
}

export interface FeedbackCreatedPayload {
  id: string;
  blockId: string;
  text: string;
}

export interface AnnotationCreatedPayload {
  id: string;
  blockId: string;
  // A content anchor (ccx `A-B#hash` form) the reducer stores verbatim. The
  // daemon normalizes it to the resolution before echoing.
  anchor: string;
  text: string;
  // The anchored lines at creation. Client-sent but always server-rewritten, so
  // it never round-trips client-authored.
  quote: string;
}

export interface AnnotationRemovedPayload {
  id: string;
  blockId: string;
}

// triage.decided is one partial-map merge, per-item last-write-wins. Each entry
// is a Decision (verdict + optional note); a `cleared` verdict deletes the item.
export interface TriageDecidedPayload {
  blockId: string;
  verdicts: Record<string, Decision>;
}

export interface InputSubmittedPayload {
  blockId: string;
  text: string;
}

// pack.interaction is the one generic pack human event. The payload is opaque to
// the host — the REST edge validated it against the pack-declared schema — so the
// reducer stores it verbatim and never inspects its shape.
export interface PackInteractionPayload {
  blockId: string;
  payload: unknown;
}

export interface SubmitPayload {
  revision: number;
}

// revising.changed replaces the agent's declared revising working set wholesale.
export interface RevisingChangedPayload {
  blockIds: string[];
  note?: string;
}

// --- System lifecycle (recorded with a `system` origin) ---

// channel.changed is the cc-interact Connectivity presence frame. The framework
// appends it into the same subject log with a `system` origin; the reducer skips
// it regardless of origin, so it never touches reduced state. Its payload embeds
// its own `type`, the self-describing wire shape every frame now takes.
export interface ChannelChangedPayload {
  type: 'channel.changed';
  connected: boolean;
}

// --- Event envelope (discriminated union on `type`) ---

export type PresentEvent =
  | { origin: 'agent'; type: 'doc.replaced'; seq: number; payload: PersistedPayload<'doc.replaced', DocReplacedPayload> }
  | { origin: 'agent'; type: 'block.upserted'; seq: number; payload: PersistedPayload<'block.upserted', BlockUpsertedPayload> }
  | { origin: 'agent'; type: 'block.removed'; seq: number; payload: PersistedPayload<'block.removed', BlockRemovedPayload> }
  | { origin: 'agent'; type: 'reply.created'; seq: number; payload: PersistedPayload<'reply.created', ReplyCreatedPayload> }
  | { origin: 'agent'; type: 'round.started'; seq: number; payload: PersistedPayload<'round.started', RoundStartedPayload> }
  | { origin: 'agent'; type: 'revising.changed'; seq: number; payload: PersistedPayload<'revising.changed', RevisingChangedPayload> }
  | { origin: 'system'; type: 'present.closed'; seq: number; payload: PersistedPayload<'present.closed', PresentClosedPayload> }
  | { origin: 'human'; type: 'decision.created'; seq: number; payload: PersistedPayload<'decision.created', DecisionCreatedPayload> }
  | { origin: 'human'; type: 'choice.selected'; seq: number; payload: PersistedPayload<'choice.selected', ChoiceSelectedPayload> }
  | { origin: 'human'; type: 'feedback.created'; seq: number; payload: PersistedPayload<'feedback.created', FeedbackCreatedPayload> }
  | { origin: 'human'; type: 'annotation.created'; seq: number; payload: PersistedPayload<'annotation.created', AnnotationCreatedPayload> }
  | { origin: 'human'; type: 'annotation.removed'; seq: number; payload: PersistedPayload<'annotation.removed', AnnotationRemovedPayload> }
  | { origin: 'human'; type: 'triage.decided'; seq: number; payload: PersistedPayload<'triage.decided', TriageDecidedPayload> }
  | { origin: 'human'; type: 'input.submitted'; seq: number; payload: PersistedPayload<'input.submitted', InputSubmittedPayload> }
  | { origin: 'human'; type: 'pack.interaction'; seq: number; payload: PersistedPayload<'pack.interaction', PackInteractionPayload> }
  | { origin: 'human'; type: 'submit'; seq: number; payload: PersistedPayload<'submit', SubmitPayload> }
  | { origin: 'system'; type: 'channel.changed'; seq: number; payload: ChannelChangedPayload };

export type PresentEventType = PresentEvent['type'];

export type AgentEvent = Extract<PresentEvent, { origin: 'agent' }>;
export type HumanEvent = Extract<PresentEvent, { origin: 'human' }>;

// --- SSE wire frame (what the /events plane delivers as each `data:`) ---

// The self-describing payload the SSE plane writes to each SSE `data:` frame: the
// event's payload with its `type` spliced in by the daemon. sse.go transmits only
// `id:` (the seq) and `data:` (the payload); the origin is a database column,
// never on the wire, so the browser discriminates purely on the embedded `type`.
// Unlike PresentEvent it is flat — the payload fields sit alongside `type`, with
// no `origin`/`seq`/nested `payload` envelope. stream.ts lifts it into a
// PresentEvent for the reducer.
export type WireFrame =
  | PersistedPayload<'doc.replaced', DocReplacedPayload>
  | PersistedPayload<'block.upserted', BlockUpsertedPayload>
  | PersistedPayload<'block.removed', BlockRemovedPayload>
  | PersistedPayload<'reply.created', ReplyCreatedPayload>
  | PersistedPayload<'round.started', RoundStartedPayload>
  | PersistedPayload<'revising.changed', RevisingChangedPayload>
  | PersistedPayload<'present.closed', PresentClosedPayload>
  | PersistedPayload<'decision.created', DecisionCreatedPayload>
  | PersistedPayload<'choice.selected', ChoiceSelectedPayload>
  | PersistedPayload<'feedback.created', FeedbackCreatedPayload>
  | PersistedPayload<'annotation.created', AnnotationCreatedPayload>
  | PersistedPayload<'annotation.removed', AnnotationRemovedPayload>
  | PersistedPayload<'triage.decided', TriageDecidedPayload>
  | PersistedPayload<'input.submitted', InputSubmittedPayload>
  | PersistedPayload<'pack.interaction', PackInteractionPayload>
  | PersistedPayload<'submit', SubmitPayload>
  | ChannelChangedPayload;

// --- Browser interaction (the POST /api/interactions body's `interaction`) ---

// One human interaction the browser submits. It is a discriminated union over
// the human event payloads, tagged with the event `type` so the daemon can
// validate it against the reduced document and append the matching event. The
// browser generates the feedback `id` (like the request nonce) so a retry is
// idempotent.
export type Interaction =
  | ({ type: 'decision.created' } & DecisionCreatedPayload)
  | ({ type: 'choice.selected' } & ChoiceSelectedPayload)
  | ({ type: 'feedback.created' } & FeedbackCreatedPayload)
  | ({ type: 'annotation.created' } & AnnotationCreatedPayload)
  | ({ type: 'annotation.removed' } & AnnotationRemovedPayload)
  | ({ type: 'triage.decided' } & TriageDecidedPayload)
  | ({ type: 'input.submitted' } & InputSubmittedPayload)
  | ({ type: 'pack.interaction' } & PackInteractionPayload)
  | ({ type: 'submit' } & SubmitPayload);

// --- Interaction cache (mirrors internal/state.State) ---

export interface Decision {
  verdict: Verdict;
  note?: string;
}

export interface Selection {
  optionIds: string[];
  // A free-text write-in outside the authored option set.
  other?: string;
}

export interface InputValue {
  text: string;
  // The round its enclosing top-level block was in when the entry was committed,
  // stamped by the reducer.
  round: number;
}

// PackValue is a human's last-write-wins interaction on a pack block: the payload
// exactly as the REST edge validated it. The reducer stays pack-blind — it never
// inspects a pack payload's shape.
export interface PackValue {
  payload: unknown;
}

export interface Feedback {
  id: string;
  text: string;
}

// Annotation is one anchored mark a human placed on a draft block. anchor is an
// opaque content-anchor string the reducer never parses; quote is the server-
// stamped text of the anchored lines.
export interface Annotation {
  id: string;
  anchor: string;
  text: string;
  quote: string;
}

export interface Reply {
  id: string;
  md: string;
}

export interface Submitted {
  value: boolean;
  revision: number;
}

export interface Closed {
  value: boolean;
  summary?: string;
}

export interface Interactions {
  decisions: Record<string, Decision>;
  choices: Record<string, Selection>;
  inputs: Record<string, InputValue>;
  packs: Record<string, PackValue>;
  feedback: Record<string, Feedback[]>;
  replies: Record<string, Reply[]>;
  // An ordered per-block annotation list, upserted in place by annotation id.
  annotations: Record<string, Annotation[]>;
  // Per-block, per-item verdicts; a `cleared` verdict deletes the item entry.
  triage: Record<string, Record<string, Decision>>;
  submitted: Submitted;
  closed: Closed;
}

// RoundRecord is a closed round: the top-level blocks live at close (frozen
// copies) plus the interaction values snapshotted to those blocks' ids.
// submittedRevision is present only when the round closed on a submit.
export interface RoundRecord {
  number: number;
  title?: string;
  blocks: Block[];
  decisions: Record<string, Decision>;
  choices: Record<string, Selection>;
  inputs: Record<string, InputValue>;
  packs: Record<string, PackValue>;
  feedback: Record<string, Feedback[]>;
  annotations: Record<string, Annotation[]>;
  triage: Record<string, Record<string, Decision>>;
  submittedRevision?: number;
}

// Rounds tracks the round partition. current is 1-based; blockRounds maps a
// top-level block id to the round of its last agent touch; history holds the
// closed rounds in ascending order.
export interface Rounds {
  current: number;
  currentTitle?: string;
  blockRounds: Record<string, number>;
  history: RoundRecord[];
}

// The agent's declared revising working set. Mirrors internal/state.Revising.
export interface Revising {
  blockIds: string[];
  note?: string;
}

// The full reduction: the current document, the keyed human interactions, the
// round partition, and the agent's declared revising working set.
export interface PresentState {
  schemaVersion: typeof EVENT_SCHEMA_VERSION;
  doc: Doc;
  interactions: Interactions;
  rounds: Rounds;
  revising: Revising;
}
