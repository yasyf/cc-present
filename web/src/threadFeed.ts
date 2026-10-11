// The margin-rail projection: the pinned thread plus a document-ordered feed of
// the other conversation-bearing blocks — live doc order, then recorded rounds.

import { flatten } from './decide';
import { topLevelRound } from './reduce';
import { isPackBlock } from './schema';
import type { Block } from './schema';
import type { Feedback, PresentState, Reply } from './events';

// A ThreadEntry is one block's conversation: its feedback (the human's notes) and
// the agent's replies. locked marks a thread with no composer: closed, stale, or history.
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

export function noteCopy(type: string): NoteCopy {
  return type === 'approval' ? FEEDBACK_COPY : NOTE_COPY;
}

export function ownsThread(block: Block, packInteractive: ReadonlySet<string>): boolean {
  if (isPackBlock(block)) return packInteractive.has(block.type);
  return block.type === 'approval' || block.type === 'choice';
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
  if (reply) return reply.md;
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
  const admit = ({ block, top, owns }: Member, feedback: Feedback[], locked: boolean, isLive: boolean) => {
    if ((!owns && block !== top) || seen.has(block.id)) return;
    seen.add(block.id);
    const blockReplies = replies[block.id] ?? [];
    all.push({
      blockId: block.id,
      type: block.type,
      label: label(block),
      feedback,
      replies: blockReplies,
      locked,
      live: isLive,
      lastComment: lastComment(feedback, blockReplies),
    });
  };
  for (const member of live) {
    const stale = topLevelRound(state, member.top.id) !== state.rounds.current;
    admit(member, state.interactions.feedback[member.block.id] ?? [], closed || stale, true);
  }
  for (const { round, members: frozen } of past) {
    for (const member of frozen) admit(member, round.feedback[member.block.id] ?? [], true, false);
  }

  const holder = [...live, ...past.flatMap((p) => p.members)].find((m) => m.block.id === activeId);
  const owner = holder && (holder.owns ? holder.block.id : holder.top.id);
  const pinned = all.find((e) => e.blockId === owner) ?? null;
  const feed = all.filter((e) => e !== pinned && hasConversation(e));
  return { pinned, feed };
}
