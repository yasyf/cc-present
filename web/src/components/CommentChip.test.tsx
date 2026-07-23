// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { ActiveBlockProvider, useActiveBlock } from '../activeBlock';
import { CommentChip } from './CommentChip';
import { emptyState } from '../reduce';

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

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function Active() {
  const { activeId, composeEpoch, pinnedOpen } = useActiveBlock();
  return (
    <div>
      <div data-testid="active">{activeId ?? 'none'}</div>
      <div data-testid="compose-epoch">{composeEpoch}</div>
      <div data-testid="pinned-open">{String(pinnedOpen)}</div>
    </div>
  );
}

function render(count: number, addLabel: string, locked = false): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[]} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <CommentChip blockId="a1" count={count} addLabel={addLabel} locked={locked} />
            <Active />
          </ActiveBlockProvider>
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

const chip = (): HTMLButtonElement => container.querySelector('.comment-chip')!;

describe('CommentChip', () => {
  it('shows the add affordance when there are no comments', () => {
    render(0, 'Add feedback');
    expect(chip().textContent).toBe('Add feedback');
    expect(chip().hasAttribute('data-count')).toBe(false);
  });

  it('pluralises the count and flags a non-empty chip', () => {
    render(1, 'Add note');
    expect(chip().textContent).toBe('1 comment');
    expect(chip().hasAttribute('data-count')).toBe(true);
    render(3, 'Add note');
    expect(chip().textContent).toBe('3 comments');
  });

  it('carries the rail-anchor marker so an outside-dismiss skips it', () => {
    render(2, 'Add note');
    expect(chip().hasAttribute('data-rail-anchor')).toBe(true);
  });

  it('pins its block, requests a composer, and opens the rail when unlocked', () => {
    render(2, 'Add note');
    const active = () => container.querySelector('[data-testid="active"]')!.textContent;
    const composeEpoch = () => container.querySelector('[data-testid="compose-epoch"]')!.textContent;
    const pinnedOpen = () => container.querySelector('[data-testid="pinned-open"]')!.textContent;
    expect(active()).toBe('none');
    expect(composeEpoch()).toBe('0');
    expect(pinnedOpen()).toBe('false');
    act(() => chip().click());
    // pin -> the block is active; requestCompose -> the composer is raised (epoch
    // bumps) and the desktop rail is pinned open.
    expect(active()).toBe('a1');
    expect(composeEpoch()).toBe('1');
    expect(pinnedOpen()).toBe('true');
  });

  it('pins its block and opens the rail without requesting a composer when locked', () => {
    render(2, 'Add note', true);
    const active = () => container.querySelector('[data-testid="active"]')!.textContent;
    const composeEpoch = () => container.querySelector('[data-testid="compose-epoch"]')!.textContent;
    const pinnedOpen = () => container.querySelector('[data-testid="pinned-open"]')!.textContent;
    expect(active()).toBe('none');
    expect(composeEpoch()).toBe('0');
    expect(pinnedOpen()).toBe('false');
    act(() => chip().click());
    expect(active()).toBe('a1');
    expect(composeEpoch()).toBe('0');
    expect(pinnedOpen()).toBe('true');
  });
});
