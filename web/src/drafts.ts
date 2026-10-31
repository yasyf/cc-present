import { useSyncExternalStore } from 'react';

export const DRAFTS_KEY_PREFIX = 'cc-present:drafts:v1:';

const NO_DRAFTS: ReadonlyMap<string, never> = new Map<string, never>();

interface DraftStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

type Drafts = Map<string, ReadonlyMap<string, unknown>>;

class DraftStore {
  private subject: string | null = null;
  private storage: DraftStorage | null = null;
  private scopes = 0;
  private drafts: Drafts = new Map();
  private listeners = new Set<() => void>();

  get epoch(): number {
    return this.scopes;
  }

  scope(subject: string, storage: DraftStorage = localStorage): void {
    if (this.subject === subject) return;
    this.subject = subject;
    this.storage = storage;
    const raw = storage.getItem(DRAFTS_KEY_PREFIX + subject);
    this.replace(raw === null ? new Map() : decodeDrafts(raw));
  }

  get<T>(blockId: string, slot: string): T | undefined {
    return this.drafts.get(blockId)?.get(slot) as T | undefined;
  }

  set(blockId: string, slot: string, value: unknown): void {
    this.drafts.set(blockId, new Map(this.drafts.get(blockId)).set(slot, value));
    this.changed();
  }

  clear(blockId: string, slot: string): void {
    const slots = this.drafts.get(blockId);
    if (!slots?.has(slot)) return;
    const rest = new Map(slots);
    rest.delete(slot);
    if (rest.size === 0) this.drafts.delete(blockId);
    else this.drafts.set(blockId, rest);
    this.changed();
  }

  entries<T>(blockId: string, group: string): ReadonlyMap<string, T> {
    return this.get<ReadonlyMap<string, T>>(blockId, group) ?? NO_DRAFTS;
  }

  setEntry(blockId: string, group: string, id: string, value: unknown): void {
    this.set(blockId, group, new Map(this.entries(blockId, group)).set(id, value));
  }

  clearEntry(blockId: string, group: string, id: string): void {
    const rest = new Map(this.entries(blockId, group));
    if (!rest.delete(id)) return;
    if (rest.size === 0) this.clear(blockId, group);
    else this.set(blockId, group, rest);
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  reset(): void {
    this.subject = null;
    this.storage = null;
    this.replace(new Map());
  }

  private replace(drafts: Drafts): void {
    this.scopes += 1;
    this.drafts = drafts;
    this.emit();
  }

  private changed(): void {
    if (this.storage !== null) {
      const key = DRAFTS_KEY_PREFIX + this.subject!;
      if (this.drafts.size === 0) this.storage.removeItem(key);
      else this.storage.setItem(key, JSON.stringify(this.drafts, encodeMaps));
    }
    this.emit();
  }

  private emit(): void {
    for (const listener of this.listeners) listener();
  }
}

function encodeMaps(_key: string, value: unknown): unknown {
  return value instanceof Map ? { $map: [...value] } : value;
}

function decodeDrafts(raw: string): Drafts {
  const drafts: unknown = JSON.parse(raw, (_key, value: unknown) =>
    value !== null && typeof value === 'object' && '$map' in value ? new Map((value as { $map: [string, unknown][] }).$map) : value,
  );
  if (!(drafts instanceof Map)) throw new Error('cc-present draft record is corrupt');
  return drafts as Drafts;
}

export const draftStore = new DraftStore();

export function useDraft<T>(blockId: string, slot: string): T | undefined {
  return useSyncExternalStore(draftStore.subscribe, () => draftStore.get<T>(blockId, slot));
}

export function useDrafts<T>(blockId: string, group: string): ReadonlyMap<string, T> {
  return useSyncExternalStore(draftStore.subscribe, () => draftStore.entries<T>(blockId, group));
}
