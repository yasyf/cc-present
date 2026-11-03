import { useState } from 'react';
import { draftStore } from '../drafts';
import { usePresent } from '../present';
import { uuid } from '../uuid';

export function useSendNote(blockId: string) {
  const { post } = usePresent();
  const [pending, setPending] = useState<{ id: string; text: string }[]>([]);

  async function sendNote(body: string) {
    const id = uuid();
    const epoch = draftStore.epoch;
    setPending((prev) => [...prev, { id, text: body }]);
    draftStore.clear(blockId, 'note');
    const ok = await post({ type: 'feedback.created', id, blockId, text: body });
    if (ok) return;
    setPending((prev) => prev.filter((p) => p.id !== id));
    if (draftStore.epoch !== epoch) return;
    const typed = draftStore.get<string>(blockId, 'note');
    draftStore.set(blockId, 'note', typed ? `${body}\n${typed}` : body);
  }

  return { pending, sendNote };
}
