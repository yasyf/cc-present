import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DECAY_MS, revisionStore } from './revision';
import { emptyState } from './reduce';
import type { PresentState, Revising, WireFrame } from './events';
import type { Block } from './schema';

const md = (id: string): Block => ({ id, type: 'markdown', md: id });

function state(blockIds: string[], revising: Revising = { blockIds: [] }): PresentState {
  const base = emptyState();
  return { ...base, doc: { ...base.doc, blocks: blockIds.map(md) }, revising };
}

const upsert = (id: string): WireFrame => ({ schemaVersion: 1, type: 'block.upserted', block: md(id) });
const removed = (id: string): WireFrame => ({ schemaVersion: 1, type: 'block.removed', id });
const replaced = (): WireFrame => ({ schemaVersion: 1, type: 'doc.replaced', doc: emptyState().doc, revision: 1 });
const revisingFrame = (blockIds: string[], note?: string): WireFrame => ({
  schemaVersion: 1,
  type: 'revising.changed',
  blockIds,
  ...(note !== undefined ? { note } : {}),
});

beforeEach(() => {
  revisionStore.reset();
});
afterEach(() => {
  vi.useRealTimers();
  revisionStore.reset();
});

describe('revisionStore changed marks', () => {
  it('records nothing from a replay frame — the live gate is closed', () => {
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    expect(revisionStore.unseenChange('b1')).toBeNull();
  });

  it('records a changed mark once live', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    expect(revisionStore.unseenChange('b1')?.kind).toBe('revised');
  });

  it('beginConnection re-closes the gate so the next replay stays badge-free', () => {
    revisionStore.markLive();
    revisionStore.beginConnection();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    expect(revisionStore.unseenChange('b1')).toBeNull();
  });

  it('marks an existing id revised and a fresh id added', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    revisionStore.ingest(upsert('b2'), state(['b1']), { blockIds: [] });
    expect(revisionStore.unseenChange('b1')?.kind).toBe('revised');
    expect(revisionStore.unseenChange('b2')?.kind).toBe('added');
  });

  it('lifts the pre-clear revising note onto the completing upsert', () => {
    revisionStore.markLive();
    const pre = state(['b1'], { blockIds: ['b1'], note: 'reworking per your pick' });
    // The reducer clears revising[b1] on this frame, so postRevising is already empty.
    revisionStore.ingest(upsert('b1'), pre, { blockIds: [] });
    expect(revisionStore.unseenChange('b1')).toEqual({ kind: 'revised', note: 'reworking per your pick' });
  });

  it('lifts the doc-level drafting note onto a fresh added id', () => {
    revisionStore.markLive();
    const pre = state(['b1'], { blockIds: [], note: 'drafting a comparison' });
    revisionStore.ingest(upsert('b2'), pre, { blockIds: [] });
    expect(revisionStore.unseenChange('b2')).toEqual({ kind: 'added', note: 'drafting a comparison' });
  });

  it('clears a mark once seen, then re-badges on a later change', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    revisionStore.markSeen('b1');
    expect(revisionStore.unseenChange('b1')).toBeNull();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    expect(revisionStore.unseenChange('b1')?.kind).toBe('revised');
  });

  it('clears every mark on doc.replaced and drops a removed id', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsert('b2'), state(['b1']), { blockIds: [] });
    revisionStore.ingest(removed('b2'), state(['b1', 'b2']), { blockIds: [] });
    expect(revisionStore.railState('b2')).toBeNull();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    revisionStore.ingest(replaced(), state([]), { blockIds: [] });
    expect(revisionStore.unseenChange('b1')).toBeNull();
  });
});

describe('revisionStore rail priority', () => {
  it('ranks revising over added over changed', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsert('b1'), state(['b1']), { blockIds: [] });
    expect(revisionStore.railState('b1')).toBe('changed');
    revisionStore.ingest(upsert('b2'), state(['b1']), { blockIds: [] });
    expect(revisionStore.railState('b2')).toBe('added');
    // b1 re-enters the revising set: revising wins over its standing changed mark.
    revisionStore.ingest(revisingFrame(['b1']), state(['b1', 'b2']), { blockIds: ['b1'] });
    expect(revisionStore.railState('b1')).toBe('revising');
  });

  it('exposes the revising membership set for next()', () => {
    revisionStore.ingest(revisingFrame(['b1', 'b2']), state(['b1', 'b2']), { blockIds: ['b1', 'b2'] });
    expect([...revisionStore.revisingSet()].sort()).toEqual(['b1', 'b2']);
  });
});

describe('revisionStore decay', () => {
  it('downgrades a revising banner to passive after 120s', () => {
    vi.useFakeTimers();
    revisionStore.ingest(revisingFrame(['b1'], 'rewriting'), state(['b1']), { blockIds: ['b1'], note: 'rewriting' });
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting', passive: false });
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting', passive: true });
  });

  it('surfaces a doc-level drafting note and decays it', () => {
    vi.useFakeTimers();
    revisionStore.ingest(revisingFrame([], 'drafting a step'), state(['b1']), { blockIds: [], note: 'drafting a step' });
    expect(revisionStore.draftingView()).toEqual({ note: 'drafting a step', passive: false });
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.draftingView()).toEqual({ note: 'drafting a step', passive: true });
  });

  it('an upsert that lands the revision clears its banner and the timer', () => {
    vi.useFakeTimers();
    revisionStore.markLive();
    revisionStore.ingest(revisingFrame(['b1'], 'rewriting'), state(['b1']), { blockIds: ['b1'], note: 'rewriting' });
    // The completing upsert drains the set (reducer), so the mirror clears too.
    revisionStore.ingest(upsert('b1'), state(['b1'], { blockIds: ['b1'], note: 'rewriting' }), { blockIds: [] });
    expect(revisionStore.revisingView('b1')).toBeNull();
    expect(revisionStore.unseenChange('b1')).toEqual({ kind: 'revised', note: 'rewriting' });
  });

  it('un-sticks a decayed revising id when the agent re-announces it with a fresh note', () => {
    vi.useFakeTimers();
    revisionStore.ingest(revisingFrame(['b1'], 'rewriting'), state(['b1']), { blockIds: ['b1'], note: 'rewriting' });
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting', passive: true });
    // The re-announcement re-arms the window and drops passivity, so the fresh note shows.
    revisionStore.ingest(revisingFrame(['b1'], 'rewriting, take 2'), state(['b1']), {
      blockIds: ['b1'],
      note: 'rewriting, take 2',
    });
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting, take 2', passive: false });
    // The new window still decays on its own 120s clock.
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting, take 2', passive: true });
  });

  it('keeps a decayed revising id passive across an unrelated frame that leaves the note', () => {
    vi.useFakeTimers();
    revisionStore.markLive();
    revisionStore.ingest(revisingFrame(['b1'], 'rewriting'), state(['b1']), { blockIds: ['b1'], note: 'rewriting' });
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting', passive: true });
    // An unrelated upsert arrives with b1 still revising under the same note: the
    // decayed state must not silently re-arm.
    revisionStore.ingest(upsert('b2'), state(['b1', 'b2'], { blockIds: ['b1'], note: 'rewriting' }), {
      blockIds: ['b1'],
      note: 'rewriting',
    });
    expect(revisionStore.revisingView('b1')).toEqual({ note: 'rewriting', passive: true });
  });

  it('decays each revising id on its own clock when a sibling joins mid-window', () => {
    vi.useFakeTimers();
    revisionStore.ingest(revisingFrame(['b1'], 'reworking'), state(['b1']), { blockIds: ['b1'], note: 'reworking' });
    vi.advanceTimersByTime(DECAY_MS / 2);
    // b2 joins under the same note; b1's clock must not restart.
    revisionStore.ingest(revisingFrame(['b1', 'b2'], 'reworking'), state(['b1', 'b2']), {
      blockIds: ['b1', 'b2'],
      note: 'reworking',
    });
    vi.advanceTimersByTime(DECAY_MS / 2);
    expect(revisionStore.revisingView('b1')?.passive).toBe(true);
    expect(revisionStore.revisingView('b2')?.passive).toBe(false);
  });

  it('restarts the drafting window and un-sticks it when the doc-level note changes', () => {
    vi.useFakeTimers();
    revisionStore.ingest(revisingFrame([], 'drafting one'), state(['b1']), { blockIds: [], note: 'drafting one' });
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.draftingView()).toEqual({ note: 'drafting one', passive: true });
    // A fresh drafting note re-announces the work: it shows on a new, non-passive window.
    revisionStore.ingest(revisingFrame([], 'drafting two'), state(['b1']), { blockIds: [], note: 'drafting two' });
    expect(revisionStore.draftingView()).toEqual({ note: 'drafting two', passive: false });
    vi.advanceTimersByTime(DECAY_MS);
    expect(revisionStore.draftingView()).toEqual({ note: 'drafting two', passive: true });
  });
});
