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

// threadFeed splits the conversation-bearing blocks into the pinned thread (the
// one the rail addresses, always rendered so its composer stays reachable) and the
// feed of every other block that has at least one note or reply.
export function threadFeed(state: PresentState, activeId: string | null): ThreadProjection {
  const replies = state.interactions.replies;
  const live: ThreadEntry[] = [];
  for (const block of flatten(state.doc.blocks)) {
    const kind = threadKind(block);
    if (!kind) continue;
    live.push({
      blockId: block.id,
      kind,
      label: label(block, kind),
      feedback: state.interactions.feedback[block.id] ?? [],
      replies: replies[block.id] ?? [],
      // An approval that forbids feedback keeps its thread visible but shows no
      // composer, mirroring the inline FeedbackThread's locked composer.
      locked: block.type === 'approval' && block.allowFeedback === false,
    });
  }

  const seen = new Set(live.map((e) => e.blockId));
  const history: ThreadEntry[] = [];
  for (const round of state.rounds.history) {
    for (const block of flatten(round.blocks)) {
      const kind = threadKind(block);
      if (!kind || seen.has(block.id)) continue;
      seen.add(block.id);
      history.push({
        blockId: block.id,
        kind,
        label: label(block, kind),
        feedback: round.feedback[block.id] ?? [],
        replies: replies[block.id] ?? [],
        locked: true,
      });
    }
  }

  const all = [...live, ...history];
  const pinned = activeId === null ? null : all.find((e) => e.blockId === activeId) ?? null;
  const feed = all.filter((e) => e !== pinned && hasConversation(e));
  return { pinned, feed };
}
