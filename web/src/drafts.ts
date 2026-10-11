import { useSyncExternalStore } from 'react';

const NO_DRAFTS: ReadonlyMap<string, never> = new Map<string, never>();

class DraftStore {
  private subject: string | null = null;
  private round = 0;
  private scopes = 0;
  private drafts = new Map<string, ReadonlyMap<string, unknown>>();
  private listeners = new Set<() => void>();

  get epoch(): number {
    return this.scopes;
  }

  scope(subject: string, round: number): void {
    if (this.subject === subject && this.round === round) return;
    this.subject = subject;
    this.round = round;
    this.drop();
  }

  get<T>(blockId: string, slot: string): T | undefined {
    return this.drafts.get(blockId)?.get(slot) as T | undefined;
  }

  set(blockId: string, slot: string, value: unknown): void {
    this.drafts.set(blockId, new Map(this.drafts.get(blockId)).set(slot, value));
    this.emit();
  }

  clear(blockId: string, slot: string): void {
    const slots = this.drafts.get(blockId);
    if (!slots?.has(slot)) return;
    const rest = new Map(slots);
    rest.delete(slot);
    if (rest.size === 0) this.drafts.delete(blockId);
    else this.drafts.set(blockId, rest);
    this.emit();
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
    this.round = 0;
    this.drop();
  }

  private drop(): void {
    this.scopes += 1;
    this.drafts.clear();
    this.emit();
  }

  private emit(): void {
    for (const listener of this.listeners) listener();
  }
}

export const draftStore = new DraftStore();

export function useDraft<T>(blockId: string, slot: string): T | undefined {
  return useSyncExternalStore(draftStore.subscribe, () => draftStore.get<T>(blockId, slot));
}

export function useDrafts<T>(blockId: string, group: string): ReadonlyMap<string, T> {
  return useSyncExternalStore(draftStore.subscribe, () => draftStore.entries<T>(blockId, group));
}
