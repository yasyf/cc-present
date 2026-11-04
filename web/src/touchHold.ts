import { request } from '@cc-interact/react';
import { withToken } from './token';

export const AFTER_ANSWER_MS = 60_000;
export const WHILE_TYPING_MS = 5 * 60_000;
const BEAT_MS = 10_000;

interface Hold {
  subject: string;
  blockId: string;
  until: number;
}

const holds = new Map<string, Hold>();
let timer: ReturnType<typeof setInterval> | undefined;

function report({ subject, blockId }: Hold) {
  void request(withToken('/api/touching'), {
    method: 'POST',
    body: JSON.stringify({ subject, blockId }),
  }).catch(() => undefined);
}

function beat() {
  const now = Date.now();
  for (const [key, hold] of holds) {
    if (now < hold.until) report(hold);
    else holds.delete(key);
  }
  if (holds.size === 0) {
    clearInterval(timer);
    timer = undefined;
  }
}

export function holdCard(subject: string, blockId: string, forMs: number) {
  const key = JSON.stringify([subject, blockId]);
  const hold = { subject, blockId, until: Date.now() + forMs };
  if (!holds.has(key)) report(hold);
  holds.set(key, hold);
  timer ??= setInterval(beat, BEAT_MS);
}

export function holdWhileTyping(subject: string, fallbackId?: string): () => void {
  const listener = (event: Event) => {
    const field = event.target;
    if (!(field instanceof HTMLTextAreaElement || field instanceof HTMLInputElement)) return;
    const id = field.closest<HTMLElement>('[data-block-id]')?.dataset.blockId ?? fallbackId;
    if (id) holdCard(subject, id, event.type === 'focusout' ? AFTER_ANSWER_MS : WHILE_TYPING_MS);
  };
  const types = ['focusin', 'input', 'focusout'];
  for (const type of types) document.addEventListener(type, listener);
  return () => {
    for (const type of types) document.removeEventListener(type, listener);
  };
}
