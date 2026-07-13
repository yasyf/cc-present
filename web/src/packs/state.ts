// usePackState: ephemeral per-tab draft state for pack components, scoped by a
// block's id and type. Survives board↔focus toggles and agent re-upserts, dies on reload.

import { createContext, useCallback, useContext, useSyncExternalStore } from 'react';

// PackBlockScope identifies the enclosing pack block; keying on id and type keeps a
// re-typed upsert from reading the old type's stale draft.
export interface PackBlockScope {
  id: string;
  type: string;
}

// PackBlockScopeContext carries the enclosing pack block's scope, set by PackBlockView;
// null outside a pack block, which usePackState treats as a programmer error.
export const PackBlockScopeContext = createContext<PackBlockScope | null>(null);

// store and listeners nest scope → key so a block scope and a state key never share a delimiter.
const store = new Map<string, Map<string, unknown>>();
const listeners = new Map<string, Map<string, Set<() => void>>>();

function scopeId(scope: PackBlockScope): string {
  return `${scope.id}\0${scope.type}`;
}

function keyMap<V>(m: Map<string, Map<string, V>>, sid: string): Map<string, V> {
  let byKey = m.get(sid);
  if (!byKey) {
    byKey = new Map();
    m.set(sid, byKey);
  }
  return byKey;
}

function emit(sid: string, key: string): void {
  const set = listeners.get(sid)?.get(key);
  if (set) for (const listener of set) listener();
}

export function resetPackStateForTest(): void {
  store.clear();
  listeners.clear();
}

// packStateListenerScopesForTest counts scopes still holding listeners, so a test can
// prove unsubscribe prunes the listener map instead of leaking sets.
export function packStateListenerScopesForTest(): number {
  return listeners.size;
}

// usePackState is ui.usePackState. The lazy seed is load-bearing: seeding without
// emitting keeps getSnapshot stable so a per-render `initial` cannot loop React.
export function usePackState<T>(key: string, initial: T): [T, (next: T) => void] {
  const scope = useContext(PackBlockScopeContext);
  if (scope === null) throw new Error('usePackState must be called inside a pack block');
  const sid = scopeId(scope);
  if (store.get(sid)?.has(key) !== true) keyMap(store, sid).set(key, initial);

  const subscribe = useCallback(
    (listener: () => void) => {
      const byKey = keyMap(listeners, sid);
      let set = byKey.get(key);
      if (!set) {
        set = new Set();
        byKey.set(key, set);
      }
      set.add(listener);
      return () => {
        set.delete(listener);
        if (set.size === 0) {
          byKey.delete(key);
          if (byKey.size === 0) listeners.delete(sid);
        }
      };
    },
    [sid, key],
  );
  const value = useSyncExternalStore(subscribe, () => store.get(sid)?.get(key) as T);
  const setValue = useCallback(
    (next: T) => {
      keyMap(store, sid).set(key, next);
      emit(sid, key);
    },
    [sid, key],
  );
  return [value, setValue];
}
