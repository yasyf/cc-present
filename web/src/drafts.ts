import { useSyncExternalStore } from 'react';

const NO_DRAFTS: ReadonlyMap<string, never> = new Map();

class DraftStore {
  private subject: string | null = null;
  private round = 0;
  private drafts = new Map<string, ReadonlyMap<string, unknown>>();
  private listeners = new Set<() => void>();

  scope(subject: string, round: number): void {
    if (this.subject === subject && this.round === round) return;
    this.subject = subject;
    this.round = round;
    this.drafts.clear();
    this.emit();
  }

  get<T>(blockId: string, slot: string): T | undefined {
    return this.drafts.get(blockId)?.get(slot) as T | undefined;
  }

  all<T>(blockId: string): ReadonlyMap<string, T> {
    return (this.drafts.get(blockId) ?? NO_DRAFTS) as ReadonlyMap<string, T>;
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

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  reset(): void {
    this.subject = null;
    this.round = 0;
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

export function useDrafts<T>(blockId: string): ReadonlyMap<string, T> {
  return useSyncExternalStore(draftStore.subscribe, () => draftStore.all<T>(blockId));
}
