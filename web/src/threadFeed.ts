// The margin-rail projection: the pinned thread plus a document-ordered feed of
// the other conversation-bearing blocks — live doc order, then recorded rounds.

import { flatten } from './decide';
import { markdownText } from './markdown';
import { topLevelRound } from './reduce';
import { isPackBlock } from './schema';
import type { Block } from './schema';
import { isRoundStub } from './events';
import type { Feedback, PresentState, Reply, RoundOutline } from './events';

export interface ThreadEntry {
  blockId: string;
  type: string;
  label: string;
  feedback: Feedback[];
  replies: Reply[];
  locked: boolean;
  live: boolean;
  // The thread's newest turn as plain text, for the feed row's one-line excerpt;
  // null only for a pinned entry that carries no conversation yet.
  lastComment: string | null;
}

export interface ThreadProjection {
  pinned: ThreadEntry | null;
  feed: ThreadEntry[];
}

export interface NoteCopy {
  add: string;
  placeholder: string;
}

const FEEDBACK_COPY: NoteCopy = { add: 'Add feedback', placeholder: 'Add feedback for the agent…' };
const NOTE_COPY: NoteCopy = { add: 'Add note', placeholder: 'Add a note for the agent…' };
const DECISION_NOTE_COPY: NoteCopy = { ...NOTE_COPY, add: 'Note on this decision' };
const STEP_NOTE_COPY: NoteCopy = { ...NOTE_COPY, add: 'Note on this step' };

export function noteCopy(type: string, step = false): NoteCopy {
  if (type === 'approval') return FEEDBACK_COPY;
  if (type === 'choice') return DECISION_NOTE_COPY;
  return step ? STEP_NOTE_COPY : NOTE_COPY;
}

export function ownsThread(block: Block, packInteractive: ReadonlySet<string>): boolean {
  if (isPackBlock(block)) return packInteractive.has(block.type);
  return block.type === 'approval' || block.type === 'choice';
}

export function threadTurns(
  block: Block,
  feedback: Record<string, Feedback[]>,
  replies: Record<string, Reply[]>,
): { feedback: Feedback[]; replies: Reply[] } {
  return turnsOf(
    flatten([block]).map((b) => b.id),
    feedback,
    replies,
  );
}

function turnsOf(
  ids: string[],
  feedback: Record<string, Feedback[]>,
  replies: Record<string, Reply[]>,
): { feedback: Feedback[]; replies: Reply[] } {
  return { feedback: ids.flatMap((id) => feedback[id] ?? []), replies: ids.flatMap((id) => replies[id] ?? []) };
}

function heading(block: Block): string | undefined {
  const fields = block as { prompt?: unknown; question?: unknown; title?: unknown; label?: unknown };
  return [fields.prompt, fields.question, fields.title, fields.label].find(
    (v): v is string => typeof v === 'string' && v.trim() !== '',
  );
}

function label(block: Block, top: Block): string {
  return (
    heading(block) ??
    heading(top) ??
    (isPackBlock(block) ? block.type : block.type.charAt(0).toUpperCase() + block.type.slice(1))
  );
}

export function outlineOf(blocks: Block[]): RoundOutline[] {
  return blocks.map((top) => ({
    id: top.id,
    type: top.type,
    label: label(top, top),
    children: top.type === 'card' ? top.children.map((child) => child.id) : undefined,
  }));
}

function outlineIds(top: RoundOutline): string[] {
  return [top.id, ...(top.children ?? [])];
}

export function blockLabel(blocks: Block[], id: string): string | null {
  for (const top of blocks) {
    const block = flatten([top]).find((b) => b.id === id);
    if (block) return label(block, top);
  }
  return null;
}

function hasConversation(entry: ThreadEntry): boolean {
  return entry.feedback.length > 0 || entry.replies.length > 0;
}

// lastComment excerpts a thread's newest turn — the latest reply once the agent has
// answered, else the latest note. There are no timestamps to interleave the two
// streams, so append order stands in: a reply answers the notes filed before it.
function lastComment(feedback: Feedback[], replies: Reply[]): string | null {
  const reply = replies[replies.length - 1];
  if (reply) return markdownText(reply.md);
  const note = feedback[feedback.length - 1];
  return note ? note.text : null;
}

// threadFeed splits the conversation-bearing blocks into the pinned thread (the
// one the rail addresses, always rendered so its composer stays reachable) and the
// feed of every other block that has at least one note or reply.
export function threadFeed(state: PresentState, activeId: string | null): ThreadProjection {
  const replies = state.interactions.replies;
  const closed = state.interactions.closed.value;
  const live = outlineOf(state.doc.blocks);
  const past = state.rounds.history.map((round) => ({
    feedback: round.feedback,
    outline: isRoundStub(round) ? round.outline : outlineOf(round.blocks),
  }));

  const all: ThreadEntry[] = [];
  const seen = new Set<string>();
  const admit = (top: RoundOutline, feedback: Record<string, Feedback[]>, locked: boolean, isLive: boolean) => {
    if (seen.has(top.id)) return;
    seen.add(top.id);
    const turns = turnsOf(outlineIds(top), feedback, replies);
    all.push({
      blockId: top.id,
      type: top.type,
      label: top.label,
      ...turns,
      locked,
      live: isLive,
      lastComment: lastComment(turns.feedback, turns.replies),
    });
  };
  for (const top of live) {
    const stale = topLevelRound(state, top.id) !== state.rounds.current;
    admit(top, state.interactions.feedback, closed || stale, !stale);
  }
  for (const round of past) {
    for (const top of round.outline) admit(top, round.feedback, true, false);
  }

  const holder = [...live, ...past.flatMap((p) => p.outline)].find(
    (top) => activeId !== null && outlineIds(top).includes(activeId),
  );
  const pinned = all.find((e) => e.blockId === holder?.id) ?? null;
  const feed = all.filter((e) => e !== pinned && hasConversation(e));
  return { pinned, feed };
}
