// The append-only event log and the interaction cache it reduces to. Each
// PresentEvent envelope matches the fixture shape the Go reducer consumes
// ({ origin, type, seq, payload }); the payload types match the Go JSON exactly.
// The reducer implementation lands in Phase 2 and consumes the same
// internal/state/testdata/*.json fixtures via vitest.

import type { Block, Doc } from './schema';

export type Origin = 'agent' | 'human';

export type Verdict = 'approved' | 'rejected' | 'cleared';

// --- Agent-origin payloads ---

export interface DocReplacedPayload {
  doc: Doc;
  revision: number;
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
}

export interface FeedbackCreatedPayload {
  id: string;
  blockId: string;
  text: string;
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
  | { origin: 'agent'; type: 'doc.replaced'; seq: number; payload: DocReplacedPayload }
  | { origin: 'agent'; type: 'block.upserted'; seq: number; payload: BlockUpsertedPayload }
  | { origin: 'agent'; type: 'block.removed'; seq: number; payload: BlockRemovedPayload }
  | { origin: 'agent'; type: 'reply.created'; seq: number; payload: ReplyCreatedPayload }
  | { origin: 'agent'; type: 'round.started'; seq: number; payload: RoundStartedPayload }
  | { origin: 'system'; type: 'present.closed'; seq: number; payload: PresentClosedPayload }
  | { origin: 'human'; type: 'decision.created'; seq: number; payload: DecisionCreatedPayload }
  | { origin: 'human'; type: 'choice.selected'; seq: number; payload: ChoiceSelectedPayload }
  | { origin: 'human'; type: 'feedback.created'; seq: number; payload: FeedbackCreatedPayload }
  | { origin: 'human'; type: 'input.submitted'; seq: number; payload: InputSubmittedPayload }
  | { origin: 'human'; type: 'pack.interaction'; seq: number; payload: PackInteractionPayload }
  | { origin: 'human'; type: 'submit'; seq: number; payload: SubmitPayload }
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
  | ({ type: 'doc.replaced' } & DocReplacedPayload)
  | ({ type: 'block.upserted' } & BlockUpsertedPayload)
  | ({ type: 'block.removed' } & BlockRemovedPayload)
  | ({ type: 'reply.created' } & ReplyCreatedPayload)
  | ({ type: 'round.started' } & RoundStartedPayload)
  | ({ type: 'present.closed' } & PresentClosedPayload)
  | ({ type: 'decision.created' } & DecisionCreatedPayload)
  | ({ type: 'choice.selected' } & ChoiceSelectedPayload)
  | ({ type: 'feedback.created' } & FeedbackCreatedPayload)
  | ({ type: 'input.submitted' } & InputSubmittedPayload)
  | ({ type: 'pack.interaction' } & PackInteractionPayload)
  | ({ type: 'submit' } & SubmitPayload)
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

// The full reduction: the current document, the keyed human interactions, and
// the round partition.
export interface PresentState {
  doc: Doc;
  interactions: Interactions;
  rounds: Rounds;
}
