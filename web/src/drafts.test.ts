import { beforeEach, describe, expect, it, vi } from 'vitest';
import { draftStore } from './drafts';

beforeEach(() => {
  draftStore.reset();
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

  it('drops every draft when the round or the subject changes, and not on a repeat scope', () => {
    draftStore.scope('s1', 1);
    draftStore.set('a1', 'note', 'x');
    draftStore.scope('s1', 1);
    expect(draftStore.get('a1', 'note')).toBe('x');

    draftStore.scope('s1', 2);
    expect(draftStore.get('a1', 'note')).toBeUndefined();

    draftStore.set('a1', 'note', 'y');
    draftStore.scope('s2', 2);
    expect(draftStore.get('a1', 'note')).toBeUndefined();
  });

  it('moves to a new epoch only when the scope changes', () => {
    draftStore.scope('s1', 1);
    const first = draftStore.epoch;
    draftStore.set('a1', 'note', 'x');
    draftStore.clear('a1', 'note');
    draftStore.scope('s1', 1);
    expect(draftStore.epoch).toBe(first);

    draftStore.scope('s1', 2);
    expect(draftStore.epoch).not.toBe(first);
  });

  it('notifies subscribers on every change until they unsubscribe', () => {
    draftStore.scope('s1', 1);
    const listener = vi.fn();
    const unsubscribe = draftStore.subscribe(listener);

    draftStore.set('a1', 'note', 'x');
    draftStore.clear('a1', 'note');
    draftStore.clear('a1', 'note');
    draftStore.scope('s1', 1);
    expect(listener).toHaveBeenCalledTimes(2);

    draftStore.scope('s1', 2);
    expect(listener).toHaveBeenCalledTimes(3);

    unsubscribe();
    draftStore.set('a1', 'note', 'y');
    expect(listener).toHaveBeenCalledTimes(3);
  });
});
