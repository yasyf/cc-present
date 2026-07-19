// Per-tab live-revision store: which steps changed since the human saw them and
// which the agent is revising. Marks record only from live frames; 120s decay.

import { useSyncExternalStore } from 'react';
import type { PresentState, Revising, WireFrame } from './events';

// Orphaned revising / drafting announcements downgrade to a passive line after
// this window — the client's presentation-only staleness cutoff.
export const DECAY_MS = 120_000;

export type RevisionKind = 'revised' | 'added';

// A step changed since the human last saw it: revised (id existed or was announced)
// or added (a fresh id joined the doc).
export interface ChangedView {
  kind: RevisionKind;
  note?: string;
}

// A step the agent has declared it is rewriting. passive once the 120s decay fired.
export interface RevisingView {
  note?: string;
  passive: boolean;
}

// The doc-level drafting announcement (revising set empty, note set).
export interface DraftingView {
  note: string;
  passive: boolean;
}

// The dominant revision state for a rail dot, priority revising > added > changed.
export type RailRevisionState = 'revising' | 'added' | 'changed';

// A coarse roll-up the later progress lane consumes for its Review warning line.
export interface RevisionSummary {
  revisingCount: number;
  drafting: boolean;
}

interface ChangedMark {
  kind: RevisionKind;
  note?: string;
  seq: number;
}

// RevisionStore is the singleton behind the hooks below; its mutators return whether
// anything user-visible changed so the stream feed emits at most one notify per frame.
class RevisionStore {
  private live = false;
  private version = 0;
  private listeners = new Set<() => void>();

  private changed = new Map<string, ChangedMark>();
  private seen = new Map<string, number>();
  private changeSeq = 0;

  private revisingIds: string[] = [];
  private revisingNote: string | undefined = undefined;
  private decayTimers = new Map<string, ReturnType<typeof setTimeout>>();
  private passive = new Set<string>();

  private draftingActive = false;
  private draftPassive = false;
  private draftTimer: ReturnType<typeof setTimeout> | null = null;

  // beginConnection marks the store non-live for a fresh EventSource so its from-zero
  // replay records no badges; the stream's url builder calls it per connection.
  beginConnection(): void {
    this.live = false;
  }

  // markLive opens the changed-badge gate at the caught-up boundary; onCaughtUp calls it.
  markLive(): void {
    this.live = true;
  }

  // ingest folds one wire frame. `pre` (pre-reduce) still holds the note the reducer
  // clears on the completing upsert; `postRevising` is the set the mirror syncs to.
  ingest(frame: WireFrame, pre: PresentState | undefined, postRevising: Revising): void {
    let dirty = false;
    if (frame.type === 'block.upserted') {
      if (this.live && pre) dirty = this.recordUpsert(frame.block.id, pre) || dirty;
    } else if (frame.type === 'block.removed') {
      dirty = this.dropId(frame.id) || dirty;
    } else if (frame.type === 'doc.replaced') {
      dirty = this.clearChangedMarks() || dirty;
    }
    dirty = this.syncRevising(postRevising) || dirty;
    if (dirty) this.emit();
  }

  private recordUpsert(id: string, pre: PresentState): boolean {
    const existed = pre.doc.blocks.some((b) => b.id === id);
    const mark: ChangedMark = {
      kind: existed ? 'revised' : 'added',
      note: noteForUpsert(pre.revising, id, existed),
      seq: ++this.changeSeq,
    };
    this.changed.set(id, mark);
    return true;
  }

  private dropId(id: string): boolean {
    const removed = this.changed.delete(id);
    const unseen = this.seen.delete(id);
    return removed || unseen;
  }

  private clearChangedMarks(): boolean {
    const had = this.changed.size > 0 || this.seen.size > 0;
    this.changed.clear();
    this.seen.clear();
    return had;
  }

  private syncRevising(next: Revising): boolean {
    const nextIds = next.blockIds ?? [];
    const nextNote = next.note;
    const nextSet = new Set(nextIds);
    const prevNote = this.revisingNote;
    let changed = false;

    // Ids that left the set: cancel the window and clear passivity. A fired window is
    // tracked only by its `passive` flag, so the scan unions the timer keys with it.
    for (const id of new Set([...this.decayTimers.keys(), ...this.passive])) {
      if (!nextSet.has(id)) {
        const timer = this.decayTimers.get(id);
        if (timer) clearTimeout(timer);
        this.decayTimers.delete(id);
        this.passive.delete(id);
        changed = true;
      }
    }
    // A newly-revising id opens its own window; an id still counting down or already
    // passive keeps its clock, so a sibling joining never restarts it.
    for (const id of nextIds) {
      if (!this.decayTimers.has(id) && !this.passive.has(id)) {
        this.armRevisingDecay(id);
        changed = true;
      }
    }
    // A changed working-set note is a fresh announcement: un-stick any passive id and
    // restart its window. A note-only change before decay leaves the clock be.
    if (nextNote !== prevNote) {
      for (const id of nextIds) {
        if (this.passive.has(id)) {
          this.armRevisingDecay(id);
          changed = true;
        }
      }
    }
    if (!sameIds(this.revisingIds, nextIds)) {
      this.revisingIds = [...nextIds];
      changed = true;
    }
    if (this.revisingNote !== nextNote) {
      this.revisingNote = nextNote;
      changed = true;
    }

    const draft = nextIds.length === 0 && nextNote !== undefined && nextNote !== '';
    if (draft && !this.draftingActive) {
      this.draftingActive = true;
      this.armDraftDecay();
      changed = true;
    } else if (draft && this.draftingActive && nextNote !== prevNote) {
      // A changed drafting note re-announces the doc-level work: restart the window
      // and un-stick passivity so the fresh note surfaces.
      this.armDraftDecay();
      changed = true;
    } else if (!draft && this.draftingActive) {
      this.draftingActive = false;
      this.draftPassive = false;
      if (this.draftTimer) {
        clearTimeout(this.draftTimer);
        this.draftTimer = null;
      }
      changed = true;
    }
    return changed;
  }

  private armRevisingDecay(id: string): void {
    const existing = this.decayTimers.get(id);
    if (existing) clearTimeout(existing);
    this.passive.delete(id);
    this.decayTimers.set(
      id,
      setTimeout(() => {
        this.decayTimers.delete(id);
        this.passive.add(id);
        this.emit();
      }, DECAY_MS),
    );
  }

  private armDraftDecay(): void {
    if (this.draftTimer) clearTimeout(this.draftTimer);
    this.draftPassive = false;
    this.draftTimer = setTimeout(() => {
      this.draftTimer = null;
      this.draftPassive = true;
      this.emit();
    }, DECAY_MS);
  }

  // markSeen clears a step's mark once viewed, so its rail badge drops and returning
  // shows no callout; a later live change bumps the seq past `seen` and re-badges it.
  markSeen(id: string): void {
    const mark = this.changed.get(id);
    if (!mark || (this.seen.get(id) ?? 0) >= mark.seq) return;
    this.seen.set(id, mark.seq);
    this.emit();
  }

  unseenChange(id: string): ChangedView | null {
    const mark = this.changed.get(id);
    if (!mark || mark.seq <= (this.seen.get(id) ?? 0)) return null;
    return { kind: mark.kind, note: mark.note };
  }

  revisingView(id: string): RevisingView | null {
    if (!this.revisingIds.includes(id)) return null;
    return { note: this.revisingNote, passive: this.passive.has(id) };
  }

  railState(id: string): RailRevisionState | null {
    if (this.revisingIds.includes(id)) return 'revising';
    const change = this.unseenChange(id);
    if (!change) return null;
    return change.kind === 'added' ? 'added' : 'changed';
  }

  draftingView(): DraftingView | null {
    if (!this.draftingActive) return null;
    return { note: this.revisingNote ?? '', passive: this.draftPassive };
  }

  summary(): RevisionSummary {
    return { revisingCount: this.revisingIds.length, drafting: this.draftingActive };
  }

  // revisingSet is the membership FocusDeck's next() skips on its momentum first pass
  // — membership only, decay is presentation.
  revisingSet(): ReadonlySet<string> {
    return new Set(this.revisingIds);
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  getVersion = (): number => this.version;

  private emit(): void {
    this.version++;
    for (const listener of this.listeners) listener();
  }

  // reset clears every mark and timer for a test's isolation; listeners persist —
  // component tests unmount (and unsubscribe) between cases.
  reset(): void {
    this.live = false;
    this.version = 0;
    this.changed.clear();
    this.seen.clear();
    this.changeSeq = 0;
    this.revisingIds = [];
    this.revisingNote = undefined;
    for (const timer of this.decayTimers.values()) clearTimeout(timer);
    this.decayTimers.clear();
    this.passive.clear();
    this.draftingActive = false;
    this.draftPassive = false;
    if (this.draftTimer) {
      clearTimeout(this.draftTimer);
      this.draftTimer = null;
    }
  }
}

// noteForUpsert lifts the causal note from the pre-clear revising set: the set's note
// when announced, the doc-level note for a fresh id, else none.
function noteForUpsert(preRevising: Revising, id: string, existed: boolean): string | undefined {
  if (preRevising.blockIds.includes(id)) return preRevising.note;
  if (!existed && preRevising.blockIds.length === 0) return preRevising.note;
  return undefined;
}

function sameIds(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((id, i) => id === b[i]);
}

export const revisionStore = new RevisionStore();

function useRevisionVersion(): number {
  return useSyncExternalStore(revisionStore.subscribe, revisionStore.getVersion, revisionStore.getVersion);
}

// useUnseenChange is the current step's changed-since-seen mark, or null.
export function useUnseenChange(id: string): ChangedView | null {
  useRevisionVersion();
  return revisionStore.unseenChange(id);
}

// useRevisingBanner is the warn banner state for a step in the revising set, or null.
export function useRevisingBanner(id: string): RevisingView | null {
  useRevisionVersion();
  return revisionStore.revisingView(id);
}

// useRailRevisionState is the dominant revision modifier for a rail dot, or null.
export function useRailRevisionState(id: string): RailRevisionState | null {
  useRevisionVersion();
  return revisionStore.railState(id);
}

// useDrafting is the doc-level drafting one-liner state, or null.
export function useDrafting(): DraftingView | null {
  useRevisionVersion();
  return revisionStore.draftingView();
}

// useRevisionSummary is the coarse roll-up the later progress lane reads.
export function useRevisionSummary(): RevisionSummary {
  useRevisionVersion();
  return revisionStore.summary();
}
