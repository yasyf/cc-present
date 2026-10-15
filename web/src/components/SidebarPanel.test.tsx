// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { ActiveBlockProvider, useActiveBlock } from '../activeBlock';
import { SidebarPanel } from './SidebarPanel';
import { emptyState } from '../reduce';
import type { Approval } from '../schema';
import type { ThreadProjection } from '../threadFeed';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// The panel reads the rail breakpoint via useMediaQuery; drive it per test. Only the
// min-width query decides sheet vs. desktop — every other query (reduced motion)
// resolves false.
function setViewport(desktop: boolean): void {
  window.matchMedia = ((query: string) => ({
    matches: query.includes('min-width: 1100px') ? desktop : false,
    media: query,
    onchange: null,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
}

const projection: ThreadProjection = {
  pinned: { blockId: 'a1', type: 'approval', label: 'Ship one', feedback: [{ id: 'f1', text: 'go' }], replies: [], locked: false, live: true, lastComment: 'go' },
  feed: [
    { blockId: 'a2', type: 'choice', label: 'Pick a plan', feedback: [{ id: 'f2', text: 'hmm' }], replies: [], locked: false, live: true, lastComment: 'hmm' },
    { blockId: 'h1', type: 'approval', label: 'Old ship', feedback: [{ id: 'f3', text: 'past' }], replies: [], locked: true, live: false, lastComment: 'past' },
  ],
};
const empty: ThreadProjection = { pinned: null, feed: [] };
const cardFeed: ThreadProjection = {
  pinned: null,
  feed: [
    { blockId: 'k1', type: 'card', label: 'Rollout', feedback: [{ id: 'f4', text: 'tighten' }], replies: [], locked: false, live: true, lastComment: 'tighten' },
  ],
};

// a2 sits in the live ring so a live jump routes through the keyboard and announces.
const ringBlocks: Approval[] = [{ id: 'a2', type: 'approval' }];

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  setViewport(false);
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  Element.prototype.scrollIntoView = vi.fn();
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function Revealed() {
  const { reveal, activeId } = useActiveBlock();
  return (
    <>
      <div data-testid="revealed">{reveal?.blockId ?? 'none'}</div>
      <div data-testid="active">{activeId ?? 'none'}</div>
    </>
  );
}

const revealed = (): string | null => container.querySelector('[data-testid="revealed"]')!.textContent;
const active = (): string | null => container.querySelector('[data-testid="active"]')!.textContent;

function render(proj: ThreadProjection = projection, onJumped?: () => void): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={ringBlocks} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <SidebarPanel projection={proj} composeEpoch={0} openComposerOnMount={false} onJumped={onJumped} />
            <Revealed />
          </ActiveBlockProvider>
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

describe('SidebarPanel masthead', () => {
  it('stamps the NOTES label and a mono total across every thread', () => {
    render();
    expect(container.querySelector('.rail-mast-label')!.textContent).toBe('Notes');
    expect(container.querySelector('.rail-mast-count')!.textContent).toBe('3');
  });
});

describe('SidebarPanel render', () => {
  it('pins the active thread with its label, note, and composer affordance', () => {
    render();
    expect(container.querySelector('.rail-pin-label')!.textContent).toBe('Ship one');
    expect(container.querySelector('.rail-pinned .thread-text')!.textContent).toBe('go');
    expect(container.querySelector('.rail-pinned .link-btn')!.textContent).toBe('Add feedback');
  });

  it('lists the other conversations as feed rows with a count and a last-comment excerpt', () => {
    render();
    const rows = [...container.querySelectorAll('.rail-feed-row')];
    expect(rows.map((r) => r.querySelector('.rail-feed-title')!.textContent)).toEqual(['Pick a plan', 'Old ship']);
    expect(rows[1]!.hasAttribute('data-locked')).toBe(true);
    expect(rows[0]!.querySelector('.rail-feed-count')!.textContent).toBe('1');
    expect(rows[0]!.querySelector('.rail-feed-excerpt')!.textContent).toBe('hmm');
  });
});

describe('SidebarPanel jump', () => {
  it('routes a live jump through the keyboard cursor', () => {
    const onJumped = vi.fn();
    render(projection, onJumped);
    act(() => (container.querySelectorAll('.rail-feed-row')[0] as HTMLButtonElement).click());
    expect(document.querySelector('.sr-only')!.textContent).toBe('Item 1 of 1');
    expect(revealed()).toBe('none');
    expect(onJumped).toHaveBeenCalledTimes(1);
  });

  it('pins a card entry and scrolls to its row, off the keyboard ring', () => {
    const row = document.createElement('div');
    row.dataset.blockId = 'k1';
    document.body.appendChild(row);
    render(cardFeed);
    expect(active()).toBe('none');
    act(() => (container.querySelector('.rail-feed-row') as HTMLButtonElement).click());
    expect(active()).toBe('k1');
    expect(row.scrollIntoView).toHaveBeenCalledTimes(1);
    expect(document.querySelector('.sr-only')!.textContent).toBe('');
    row.remove();
  });

  it('reveals a history jump inside its frozen round', () => {
    const onJumped = vi.fn();
    render(projection, onJumped);
    act(() => (container.querySelectorAll('.rail-feed-row')[1] as HTMLButtonElement).click());
    expect(revealed()).toBe('h1');
    expect(active()).toBe('h1');
    expect(document.querySelector('.sr-only')!.textContent).not.toBe('Item 1 of 1');
    expect(onJumped).toHaveBeenCalledTimes(1);
  });
});

describe('SidebarPanel empty', () => {
  it('draws a designed vacancy — a mark over a two-line explainer, no thread or feed', () => {
    render(empty);
    expect(container.querySelector('.rail-mast-count')!.textContent).toBe('0');
    expect(container.querySelector('.rail-empty-mark svg')).not.toBeNull();
    expect(container.querySelector('.rail-empty-lead')!.textContent).toBe('No notes yet.');
    expect(container.querySelector('.rail-empty-how')!.textContent).toContain('note chip on any block');
    expect(container.querySelector('.rail-pinned')).toBeNull();
    expect(container.querySelector('.rail-feed')).toBeNull();
  });
});

describe('SidebarPanel chrome', () => {
  it('gives the sheet a close control and drops the keyboard hint', () => {
    setViewport(false);
    render(empty);
    expect(container.querySelector('.rail-close')).not.toBeNull();
    expect(container.querySelector('.rail-empty-how kbd')).toBeNull();
  });

  it('gives the desktop overlay no close control but names the keyboard shortcut', () => {
    setViewport(true);
    render(empty);
    expect(container.querySelector('.rail-close')).toBeNull();
    expect(container.querySelector('.rail-empty-how kbd')!.textContent).toBe('f');
  });
});
