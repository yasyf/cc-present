import { beforeEach, describe, expect, it } from 'vitest';
import { VIEWED_KEY_PREFIX, viewedStore } from './viewed';

class MemoryStorage {
  readonly items = new Map<string, string>();
  getItem(key: string): string | null {
    return this.items.get(key) ?? null;
  }
  setItem(key: string, value: string): void {
    this.items.set(key, value);
  }
}

let storage: MemoryStorage;

beforeEach(() => {
  viewedStore.reset();
  storage = new MemoryStorage();
});

describe('viewedStore', () => {
  it('records in memory only until scoped', () => {
    viewedStore.mark(['a1'], storage);
    expect(viewedStore.viewed()).toEqual(['a1']);
    expect(storage.items.size).toBe(0);
  });

  it('persists marks per subject and round, deduplicated in first-seen order', () => {
    viewedStore.scope('s1', 2, storage);
    viewedStore.mark(['c1', 'a1'], storage);
    viewedStore.mark(['a1', 'm1'], storage);
    expect(viewedStore.viewed()).toEqual(['c1', 'a1', 'm1']);
    expect(storage.getItem(`${VIEWED_KEY_PREFIX}s1`)).toBe('{"round":2,"ids":["c1","a1","m1"]}');
  });

  it('restores the same round after a reload and starts a later round fresh', () => {
    storage.setItem(`${VIEWED_KEY_PREFIX}s1`, '{"round":2,"ids":["c1"]}');
    viewedStore.scope('s1', 2, storage);
    expect(viewedStore.viewed()).toEqual(['c1']);
    viewedStore.scope('s1', 3, storage);
    expect(viewedStore.viewed()).toEqual([]);
  });

  it('keeps subjects apart', () => {
    viewedStore.scope('s1', 1, storage);
    viewedStore.mark(['a1'], storage);
    viewedStore.scope('s2', 1, storage);
    expect(viewedStore.viewed()).toEqual([]);
  });

  it('refuses a corrupt record', () => {
    storage.setItem(`${VIEWED_KEY_PREFIX}s1`, '{"round":"2","ids":[]}');
    expect(() => viewedStore.scope('s1', 2, storage)).toThrow('corrupt');
  });
});
