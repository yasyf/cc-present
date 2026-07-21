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
window.matchMedia = ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener() {},
  removeEventListener() {},
  addListener() {},
  removeListener() {},
  dispatchEvent: () => false,
})) as typeof window.matchMedia;

const projection: ThreadProjection = {
  pinned: { blockId: 'a1', kind: 'approval', label: 'Ship one', feedback: [{ id: 'f1', text: 'go' }], replies: [], locked: false },
  feed: [
    { blockId: 'a2', kind: 'choice', label: 'Pick a plan', feedback: [{ id: 'f2', text: 'hmm' }], replies: [], locked: false },
    { blockId: 'h1', kind: 'approval', label: 'Old ship', feedback: [{ id: 'f3', text: 'past' }], replies: [], locked: true },
  ],
};

// a2 sits in the live ring so a live jump routes through the keyboard and announces.
const ringBlocks: Approval[] = [{ id: 'a2', type: 'approval' }];

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  Element.prototype.scrollIntoView = vi.fn();
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function render(onJumped?: () => void): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={ringBlocks} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <SidebarPanel projection={projection} composeEpoch={0} openComposerOnMount={false} onJumped={onJumped} />
          </ActiveBlockProvider>
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

describe('SidebarPanel render', () => {
  it('pins the active thread with its label, note, and composer affordance', () => {
    render();
    expect(container.querySelector('.rail-pin-label')!.textContent).toBe('Ship one');
    expect(container.querySelector('.rail-pinned .thread-text')!.textContent).toBe('go');
    expect(container.querySelector('.rail-pinned .link-btn')!.textContent).toBe('Add feedback');
  });

  it('lists the other conversations as read-only feed rows', () => {
    render();
    const rows = [...container.querySelectorAll('.rail-feed-row')];
    expect(rows.map((r) => r.querySelector('.rail-feed-jump')!.textContent)).toEqual(['Pick a plan', 'Old ship']);
    expect(rows[1]!.hasAttribute('data-locked')).toBe(true);
    expect(rows[0]!.querySelector('.thread-text')!.textContent).toBe('hmm');
  });
});

describe('SidebarPanel jump', () => {
  it('routes a live jump through the keyboard cursor', () => {
    const onJumped = vi.fn();
    render(onJumped);
    act(() => (container.querySelectorAll('.rail-feed-jump')[0] as HTMLButtonElement).click());
    expect(document.querySelector('.sr-only')!.textContent).toBe('Item 1 of 1');
    expect(onJumped).toHaveBeenCalledTimes(1);
  });

  it('scrolls a history jump into view by its data-block-id', () => {
    const onJumped = vi.fn();
    const frozen = document.createElement('div');
    frozen.setAttribute('data-block-id', 'h1');
    document.body.appendChild(frozen);
    render(onJumped);
    act(() => (container.querySelectorAll('.rail-feed-jump')[1] as HTMLButtonElement).click());
    expect(frozen.scrollIntoView).toHaveBeenCalledTimes(1);
    expect(onJumped).toHaveBeenCalledTimes(1);
    frozen.remove();
  });
});
