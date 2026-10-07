// Per-tab record of the blocks the human opened this round, persisted per subject;
// the submit carries it as `viewed`.

import { useEffect } from 'react';
import type { RefObject } from 'react';
import type { Block } from './schema';
import { flatten } from './decide';

export const VIEWED_KEY_PREFIX = 'cc-present:viewed:v1:';
export const FOCUS_DWELL_MS = 800;
export const BOARD_DWELL_MS = 1_500;

interface ViewedStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

interface StoredViewed {
  round: number;
  ids: string[];
}

// ViewedStore records in memory only until PresentView scopes it to a subject.
class ViewedStore {
  private subject: string | null = null;
  private round = 0;
  private ids = new Set<string>();

  // scope loads what an earlier tab recorded for this round; an older round starts fresh.
  scope(subject: string, round: number, storage: ViewedStorage = localStorage): void {
    if (this.subject === subject && this.round === round) return;
    this.subject = subject;
    this.round = round;
    const raw = storage.getItem(VIEWED_KEY_PREFIX + subject);
    const stored = raw === null ? null : decodeViewed(raw);
    this.ids = new Set(stored?.round === round ? stored.ids : []);
  }

  mark(ids: readonly string[], storage: ViewedStorage = localStorage): void {
    const before = this.ids.size;
    for (const id of ids) this.ids.add(id);
    if (this.ids.size === before || this.subject === null) return;
    const stored: StoredViewed = { round: this.round, ids: [...this.ids] };
    storage.setItem(VIEWED_KEY_PREFIX + this.subject, JSON.stringify(stored));
  }

  viewed(): string[] {
    return [...this.ids];
  }

  reset(): void {
    this.subject = null;
    this.round = 0;
    this.ids.clear();
  }
}

function decodeViewed(raw: string): StoredViewed {
  const parsed = JSON.parse(raw) as Partial<StoredViewed> | null;
  if (
    parsed === null ||
    typeof parsed !== 'object' ||
    typeof parsed.round !== 'number' ||
    !Array.isArray(parsed.ids) ||
    !parsed.ids.every((id) => typeof id === 'string')
  ) {
    throw new Error('cc-present viewed record is corrupt');
  }
  return { round: parsed.round, ids: parsed.ids };
}

export const viewedStore = new ViewedStore();

// blockViewIds adds card children: opening a card shows every block nested in it.
export function blockViewIds(blocks: Block[]): string[] {
  return flatten(blocks).map((b) => b.id);
}

// A row past half on screen, or one spanning the viewport's middle line, counts as
// on screen; the second test is what lets a row many viewports tall qualify.
const OBSERVED: { init: IntersectionObserverInit; on: (entry: IntersectionObserverEntry) => boolean }[] = [
  { init: { threshold: 0.5 }, on: (entry) => entry.intersectionRatio >= 0.5 },
  { init: { rootMargin: '-50% 0px -50% 0px' }, on: (entry) => entry.isIntersecting },
];

// useViewedOnScreen marks block opened once ref's element stays on screen for
// BOARD_DWELL_MS; leaving the screen first cancels the dwell.
export function useViewedOnScreen(ref: RefObject<HTMLElement | null>, block: Block): void {
  useEffect(() => {
    const el = ref.current!;
    const visible = OBSERVED.map(() => false);
    let timer: ReturnType<typeof setTimeout> | null = null;
    const observers = OBSERVED.map(
      ({ init, on }, i) =>
        new IntersectionObserver((entries) => {
          visible[i] = on(entries[entries.length - 1]!);
          if (!visible.some(Boolean)) {
            if (timer) clearTimeout(timer);
            timer = null;
            return;
          }
          timer ??= setTimeout(() => {
            viewedStore.mark(blockViewIds([block]));
            for (const observer of observers) observer.disconnect();
          }, BOARD_DWELL_MS);
        }, init),
    );
    for (const observer of observers) observer.observe(el);
    return () => {
      for (const observer of observers) observer.disconnect();
      if (timer) clearTimeout(timer);
    };
  }, [ref, block]);
}
