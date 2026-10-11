import { beforeEach, describe, expect, it, vi } from 'vitest';
import { draftStore } from './drafts';

beforeEach(() => {
  draftStore.reset();
});

describe('draftStore', () => {
  it('sets, reads, and clears a slot without touching its neighbors', () => {
    draftStore.set('a1', 'note', 'half a thought');
    draftStore.set('a1', 'other', 'elsewhere');
    draftStore.set('t1', 'i0', 'flaky');
    expect(draftStore.get<string>('a1', 'note')).toBe('half a thought');
    expect([...draftStore.all<string>('a1')]).toEqual([
      ['note', 'half a thought'],
      ['other', 'elsewhere'],
    ]);

    draftStore.clear('a1', 'note');
    expect(draftStore.get('a1', 'note')).toBeUndefined();
    expect(draftStore.get('a1', 'other')).toBe('elsewhere');
    expect(draftStore.get('t1', 'i0')).toBe('flaky');
  });

  it('hands back one shared empty map for a block with no drafts', () => {
    expect(draftStore.all('a1')).toBe(draftStore.all('b2'));
    draftStore.set('a1', 'note', 'x');
    draftStore.clear('a1', 'note');
    expect(draftStore.all('a1')).toBe(draftStore.all('b2'));
  });

  it('keeps a block snapshot stable until one of its slots changes', () => {
    draftStore.set('a1', 'note', 'x');
    const before = draftStore.all('a1');
    draftStore.set('b2', 'note', 'y');
    expect(draftStore.all('a1')).toBe(before);
    draftStore.set('a1', 'note', 'xy');
    expect(draftStore.all('a1')).not.toBe(before);
    expect(before.get('note')).toBe('x');
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
