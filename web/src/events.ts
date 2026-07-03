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

export interface SubmitPayload {
  revision: number;
}

// --- Presence (framework-appended, skipped by the reducer) ---

// channel.changed is the cc-interact Connectivity presence frame. The framework
// appends it into the same subject log with a `system` origin; the reducer skips
// it regardless of origin, so it never touches reduced state.
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
  | { origin: 'agent'; type: 'present.closed'; seq: number; payload: PresentClosedPayload }
  | { origin: 'human'; type: 'decision.created'; seq: number; payload: DecisionCreatedPayload }
  | { origin: 'human'; type: 'choice.selected'; seq: number; payload: ChoiceSelectedPayload }
  | { origin: 'human'; type: 'feedback.created'; seq: number; payload: FeedbackCreatedPayload }
  | { origin: 'human'; type: 'input.submitted'; seq: number; payload: InputSubmittedPayload }
  | { origin: 'human'; type: 'submit'; seq: number; payload: SubmitPayload }
  | { origin: 'system'; type: 'channel.changed'; seq: number; payload: ChannelChangedPayload };

export type PresentEventType = PresentEvent['type'];

export type AgentEvent = Extract<PresentEvent, { origin: 'agent' }>;
export type HumanEvent = Extract<PresentEvent, { origin: 'human' }>;

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
  feedback: Record<string, Feedback[]>;
  replies: Record<string, Reply[]>;
  submitted: Submitted;
  closed: Closed;
}

// The full reduction: the current document plus the keyed human interactions.
export interface PresentState {
  doc: Doc;
  interactions: Interactions;
}
