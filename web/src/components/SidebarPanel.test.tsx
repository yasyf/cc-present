// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { ActiveBlockProvider } from '../activeBlock';
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
  pinned: { blockId: 'a1', kind: 'approval', label: 'Ship one', feedback: [{ id: 'f1', text: 'go' }], replies: [], locked: false, lastComment: 'go' },
  feed: [
    { blockId: 'a2', kind: 'choice', label: 'Pick a plan', feedback: [{ id: 'f2', text: 'hmm' }], replies: [], locked: false, lastComment: 'hmm' },
    { blockId: 'h1', kind: 'approval', label: 'Old ship', feedback: [{ id: 'f3', text: 'past' }], replies: [], locked: true, lastComment: 'past' },
  ],
};
const empty: ThreadProjection = { pinned: null, feed: [] };

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

function render(proj: ThreadProjection = projection, onJumped?: () => void): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={ringBlocks} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <SidebarPanel projection={proj} composeEpoch={0} openComposerOnMount={false} onJumped={onJumped} />
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
    expect(onJumped).toHaveBeenCalledTimes(1);
  });

  it('scrolls a history jump into view by its data-block-id', () => {
    const onJumped = vi.fn();
    const frozen = document.createElement('div');
    frozen.setAttribute('data-block-id', 'h1');
    document.body.appendChild(frozen);
    render(projection, onJumped);
    act(() => (container.querySelectorAll('.rail-feed-row')[1] as HTMLButtonElement).click());
    expect(frozen.scrollIntoView).toHaveBeenCalledTimes(1);
    expect(onJumped).toHaveBeenCalledTimes(1);
    frozen.remove();
  });
});

describe('SidebarPanel empty', () => {
  it('draws a designed vacancy — a mark over a two-line explainer, no thread or feed', () => {
    render(empty);
    expect(container.querySelector('.rail-mast-count')!.textContent).toBe('0');
    expect(container.querySelector('.rail-empty-mark svg')).not.toBeNull();
    expect(container.querySelector('.rail-empty-lead')!.textContent).toBe('No notes yet.');
    expect(container.querySelector('.rail-empty-how')!.textContent).toContain('Add note');
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
