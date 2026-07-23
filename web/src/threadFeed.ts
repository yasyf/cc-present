// The margin-rail projection: the pinned thread plus a document-ordered feed of
// the other conversation-bearing blocks — live doc order, then recorded rounds.

import { flatten } from './decide';
import type { Block } from './schema';
import type { Feedback, PresentState, Reply } from './events';

export type ThreadKind = 'approval' | 'choice';

// A ThreadEntry is one block's conversation: its feedback (the human's notes) and
// the agent's replies. locked marks a frozen history block whose composer is gone.
export interface ThreadEntry {
  blockId: string;
  kind: ThreadKind;
  label: string;
  feedback: Feedback[];
  replies: Reply[];
  locked: boolean;
  // The thread's newest turn as plain text, for the feed row's one-line excerpt;
  // null only for a pinned entry that carries no conversation yet.
  lastComment: string | null;
}

export interface ThreadProjection {
  pinned: ThreadEntry | null;
  feed: ThreadEntry[];
}

function threadKind(block: Block): ThreadKind | null {
  return block.type === 'approval' || block.type === 'choice' ? block.type : null;
}

function label(block: Block, kind: ThreadKind): string {
  const prompt = (block as { prompt?: string }).prompt;
  return prompt && prompt.trim() !== '' ? prompt : kind === 'approval' ? 'Approval' : 'Choice';
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
export function threadFeed(state: PresentState, activeId: string | null): ThreadProjection {
  const replies = state.interactions.replies;
  const live: ThreadEntry[] = [];
  for (const block of flatten(state.doc.blocks)) {
    const kind = threadKind(block);
    if (!kind) continue;
    const feedback = state.interactions.feedback[block.id] ?? [];
    const blockReplies = replies[block.id] ?? [];
    live.push({
      blockId: block.id,
      kind,
      label: label(block, kind),
      feedback,
      replies: blockReplies,
      // An approval that forbids feedback keeps its thread visible but shows no
      // composer, mirroring the inline FeedbackThread's locked composer.
      locked: block.type === 'approval' && block.allowFeedback === false,
      lastComment: lastComment(feedback, blockReplies),
    });
  }

  const seen = new Set(live.map((e) => e.blockId));
  const history: ThreadEntry[] = [];
  for (const round of state.rounds.history) {
    for (const block of flatten(round.blocks)) {
      const kind = threadKind(block);
      if (!kind || seen.has(block.id)) continue;
      seen.add(block.id);
      const feedback = round.feedback[block.id] ?? [];
      const blockReplies = replies[block.id] ?? [];
      history.push({
        blockId: block.id,
        kind,
        label: label(block, kind),
        feedback,
        replies: blockReplies,
        locked: true,
        lastComment: lastComment(feedback, blockReplies),
      });
    }
  }

  const all = [...live, ...history];
  const pinned = activeId === null ? null : all.find((e) => e.blockId === activeId) ?? null;
  const feed = all.filter((e) => e !== pinned && hasConversation(e));
  return { pinned, feed };
}
