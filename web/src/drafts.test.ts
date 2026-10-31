import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DRAFTS_KEY_PREFIX, draftStore } from './drafts';

function memoryStorage() {
  const items = new Map<string, string>();
  return {
    items,
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => void items.set(key, value),
    removeItem: (key: string) => void items.delete(key),
  };
}

let storage = memoryStorage();

beforeEach(() => {
  draftStore.reset();
  storage = memoryStorage();
});

describe('draftStore', () => {
  it('sets, reads, and clears a slot without touching its neighbors', () => {
    draftStore.set('a1', 'note', 'half a thought');
    draftStore.set('a1', 'other', 'elsewhere');
    draftStore.set('t1', 'note', 'flaky');
    expect(draftStore.get<string>('a1', 'note')).toBe('half a thought');

    draftStore.clear('a1', 'note');
    expect(draftStore.get('a1', 'note')).toBeUndefined();
    expect(draftStore.get('a1', 'other')).toBe('elsewhere');
    expect(draftStore.get('t1', 'note')).toBe('flaky');
  });

  it('keeps a group of entries apart from the block slots beside it', () => {
    draftStore.set('d1', 'note', 'a general note');
    draftStore.set('d1', 'input', 'typed');
    draftStore.setEntry('d1', 'annotations', 'n1', 'on line two');
    draftStore.setEntry('d1', 'annotations', 'note', 'an entry that shares a slot name');

    expect([...draftStore.entries<string>('d1', 'annotations')]).toEqual([
      ['n1', 'on line two'],
      ['note', 'an entry that shares a slot name'],
    ]);
    expect(draftStore.get('d1', 'note')).toBe('a general note');

    draftStore.clearEntry('d1', 'annotations', 'note');
    expect([...draftStore.entries<string>('d1', 'annotations')]).toEqual([['n1', 'on line two']]);
    expect(draftStore.get('d1', 'note')).toBe('a general note');
  });

  it('hands back one shared empty map for a group with no entries', () => {
    expect(draftStore.entries('a1', 'annotations')).toBe(draftStore.entries('b2', 'notes'));
    draftStore.setEntry('a1', 'annotations', 'n1', 'x');
    draftStore.clearEntry('a1', 'annotations', 'n1');
    expect(draftStore.entries('a1', 'annotations')).toBe(draftStore.entries('b2', 'notes'));
    expect(draftStore.get('a1', 'annotations')).toBeUndefined();
  });

  it('keeps a group snapshot stable until one of its entries changes', () => {
    draftStore.setEntry('a1', 'annotations', 'n1', 'x');
    const before = draftStore.entries<string>('a1', 'annotations');
    draftStore.setEntry('b2', 'annotations', 'n1', 'y');
    draftStore.set('a1', 'note', 'beside it');
    expect(draftStore.entries('a1', 'annotations')).toBe(before);
    draftStore.setEntry('a1', 'annotations', 'n1', 'xy');
    expect(draftStore.entries('a1', 'annotations')).not.toBe(before);
    expect(before.get('n1')).toBe('x');
  });

  it('restores the unsent drafts of a board in a fresh page, entries included', () => {
    draftStore.scope('s1', storage);
    draftStore.set('a1', 'note', 'half a thought');
    draftStore.setEntry('d1', 'annotations', 'n1', { anchor: 2, focus: 3, text: 'on line two' });

    draftStore.reset();
    draftStore.scope('s1', storage);

    expect(draftStore.get('a1', 'note')).toBe('half a thought');
    expect([...draftStore.entries('d1', 'annotations')]).toEqual([['n1', { anchor: 2, focus: 3, text: 'on line two' }]]);
  });

  it('forgets the stored record once its last draft is sent', () => {
    draftStore.scope('s1', storage);
    draftStore.set('a1', 'note', 'x');
    expect(storage.items.has(DRAFTS_KEY_PREFIX + 's1')).toBe(true);

    draftStore.clear('a1', 'note');
    expect(storage.items.has(DRAFTS_KEY_PREFIX + 's1')).toBe(false);
  });

  it('keeps the drafts of each board apart, and a repeat scope keeps what is typed', () => {
    draftStore.scope('s1', storage);
    draftStore.set('a1', 'note', 'x');
    draftStore.scope('s1', storage);
    expect(draftStore.get('a1', 'note')).toBe('x');

    draftStore.scope('s2', storage);
    expect(draftStore.get('a1', 'note')).toBeUndefined();

    draftStore.scope('s1', storage);
    expect(draftStore.get('a1', 'note')).toBe('x');
  });

  it('moves to a new epoch only when the board changes', () => {
    draftStore.scope('s1', storage);
    const first = draftStore.epoch;
    draftStore.set('a1', 'note', 'x');
    draftStore.clear('a1', 'note');
    draftStore.scope('s1', storage);
    expect(draftStore.epoch).toBe(first);

    draftStore.scope('s2', storage);
    expect(draftStore.epoch).not.toBe(first);
  });

  it('refuses a corrupt stored record', () => {
    storage.setItem(DRAFTS_KEY_PREFIX + 's1', '[]');
    expect(() => draftStore.scope('s1', storage)).toThrow('cc-present draft record is corrupt');
  });

  it('notifies subscribers on every change until they unsubscribe', () => {
    draftStore.scope('s1', storage);
    const listener = vi.fn();
    const unsubscribe = draftStore.subscribe(listener);

    draftStore.set('a1', 'note', 'x');
    draftStore.clear('a1', 'note');
    draftStore.clear('a1', 'note');
    draftStore.scope('s1', storage);
    expect(listener).toHaveBeenCalledTimes(2);

    draftStore.scope('s2', storage);
    expect(listener).toHaveBeenCalledTimes(3);

    unsubscribe();
    draftStore.set('a1', 'note', 'y');
    expect(listener).toHaveBeenCalledTimes(3);
  });
});
