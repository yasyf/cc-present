import { useEffect } from 'react';
import type { ThreadEntry } from './threadFeed';

type ThreadOpener = (entry: ThreadEntry) => void;

let sink: ThreadOpener | null = null;

export function setThreadOpener(fn: ThreadOpener | null): void {
  sink = fn;
}

export function hasThreadOpener(): boolean {
  return sink !== null;
}

export function openThread(entry: ThreadEntry): void {
  if (!sink) throw new Error('thread opened before the shell mounted');
  sink(entry);
}

export function useThreadOpener(fn: ThreadOpener): void {
  setThreadOpener(fn);
  useEffect(() => () => setThreadOpener(null), []);
}
