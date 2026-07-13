// usePackState: ephemeral per-tab draft state for pack components, scoped by block
// id. It survives board↔focus toggles and agent re-upserts, dying only on reload.

import { createContext, useCallback, useContext, useSyncExternalStore } from 'react';

// PackBlockIdContext carries the enclosing pack block's id, set by PackBlockView;
// null outside a pack block, which usePackState treats as a programmer error.
export const PackBlockIdContext = createContext<string | null>(null);

const store = new Map<string, unknown>();
const listeners = new Map<string, Set<() => void>>();

function scopeKey(blockId: string, key: string): string {
  return `${blockId}\0${key}`;
}

function listenersFor(sk: string): Set<() => void> {
  let set = listeners.get(sk);
  if (!set) {
    set = new Set();
    listeners.set(sk, set);
  }
  return set;
}

function emit(sk: string): void {
  const set = listeners.get(sk);
  if (set) for (const listener of set) listener();
}

export function resetPackStateForTest(): void {
  store.clear();
  listeners.clear();
}

// usePackState is ui.usePackState. The lazy seed is load-bearing: seeding without
// emitting keeps getSnapshot stable so a per-render `initial` cannot loop React.
export function usePackState<T>(key: string, initial: T): [T, (next: T) => void] {
  const blockId = useContext(PackBlockIdContext);
  if (blockId === null) throw new Error('usePackState must be called inside a pack block');
  const sk = scopeKey(blockId, key);
  if (!store.has(sk)) store.set(sk, initial);

  const subscribe = useCallback(
    (listener: () => void) => {
      const set = listenersFor(sk);
      set.add(listener);
      return () => {
        set.delete(listener);
      };
    },
    [sk],
  );
  const value = useSyncExternalStore(subscribe, () => store.get(sk) as T);
  const setValue = useCallback(
    (next: T) => {
      store.set(sk, next);
      emit(sk);
    },
    [sk],
  );
  return [value, setValue];
}
