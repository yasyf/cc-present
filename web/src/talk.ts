import type { Feedback, Reply } from './events';
import type { Block } from './schema';

export type TalkEntry =
  | { kind: 'note'; note: Feedback }
  | { kind: 'reply'; reply: Reply }
  | { kind: 'block'; block: Block };

export function talkEntries(blocks: Block[], talkId: string, feedback: Feedback[], replies: Reply[]): TalkEntry[] {
  const noteIds = new Set(feedback.map((f) => f.id));
  const rest = blocks.filter((b) => b.id !== talkId);
  const anchor = (b: Block) => (b.type === 'card' && b.re && noteIds.has(b.re) ? b.re : undefined);
  const answers = (re: string | undefined): TalkEntry[] => [
    ...replies
      .filter((r) => (re === undefined ? !r.re || !noteIds.has(r.re) : r.re === re))
      .map((reply): TalkEntry => ({ kind: 'reply', reply })),
    ...rest.filter((b) => anchor(b) === re).map((block): TalkEntry => ({ kind: 'block', block })),
  ];
  return [
    ...answers(undefined),
    ...feedback.flatMap((note): TalkEntry[] => [{ kind: 'note', note }, ...answers(note.id)]),
  ];
}
