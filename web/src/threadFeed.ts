// The margin-rail projection: the pinned thread plus a document-ordered feed of
// the other conversation-bearing blocks — live doc order, then recorded rounds.

import { flatten } from './decide';
import { markdownText } from './markdown';
import { topLevelRound } from './reduce';
import { isPackBlock } from './schema';
import type { Block } from './schema';
import type { Feedback, PresentState, Reply } from './events';

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
  packInteractive: ReadonlySet<string>,
): { feedback: Feedback[]; replies: Reply[] } {
  const folded = block.type === 'card' ? block.children.filter((child) => !ownsThread(child, packInteractive)) : [];
  const ids = [block, ...folded].map((b) => b.id);
  return { feedback: ids.flatMap((id) => feedback[id] ?? []), replies: ids.flatMap((id) => replies[id] ?? []) };
}

interface Member {
  block: Block;
  top: Block;
  owns: boolean;
}

function members(blocks: Block[], packInteractive: ReadonlySet<string>): Member[] {
  return blocks.flatMap((top) =>
    flatten([top]).map((block) => ({ block, top, owns: ownsThread(block, packInteractive) })),
  );
}

function label(block: Block): string {
  const fields = block as { prompt?: unknown; question?: unknown; title?: unknown; label?: unknown };
  const heading = [fields.prompt, fields.question, fields.title, fields.label].find(
    (v): v is string => typeof v === 'string' && v.trim() !== '',
  );
  return heading ?? (isPackBlock(block) ? block.type : block.type.charAt(0).toUpperCase() + block.type.slice(1));
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
export function threadFeed(
  state: PresentState,
  activeId: string | null,
  packInteractive: ReadonlySet<string>,
): ThreadProjection {
  const replies = state.interactions.replies;
  const closed = state.interactions.closed.value;
  const live = members(state.doc.blocks, packInteractive);
  const past = state.rounds.history.map((round) => ({ round, members: members(round.blocks, packInteractive) }));

  const all: ThreadEntry[] = [];
  const seen = new Set<string>();
  const admit = (
    { block, top, owns }: Member,
    feedback: Record<string, Feedback[]>,
    locked: boolean,
    isLive: boolean,
  ) => {
    if ((!owns && block !== top) || seen.has(block.id)) return;
    seen.add(block.id);
    const turns = threadTurns(block, feedback, replies, packInteractive);
    all.push({
      blockId: block.id,
      type: block.type,
      label: label(block),
      ...turns,
      locked,
      live: isLive,
      lastComment: lastComment(turns.feedback, turns.replies),
    });
  };
  for (const member of live) {
    const stale = topLevelRound(state, member.top.id) !== state.rounds.current;
    admit(member, state.interactions.feedback, closed || stale, !stale);
  }
  for (const { round, members: frozen } of past) {
    for (const member of frozen) admit(member, round.feedback, true, false);
  }

  const holder = [...live, ...past.flatMap((p) => p.members)].find((m) => m.block.id === activeId);
  const owner = holder && (holder.owns ? holder.block.id : holder.top.id);
  const pinned = all.find((e) => e.blockId === owner) ?? null;
  const feed = all.filter((e) => e !== pinned && hasConversation(e));
  return { pinned, feed };
}
