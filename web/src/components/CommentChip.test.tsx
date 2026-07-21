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
  return <div data-testid="active">{useActiveBlock().activeId ?? 'none'}</div>;
}

function render(count: number, addLabel: string): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[]} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <CommentChip blockId="a1" count={count} addLabel={addLabel} />
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

  it('pins its block on click', () => {
    render(2, 'Add note');
    expect(container.querySelector('[data-testid="active"]')!.textContent).toBe('none');
    act(() => chip().click());
    expect(container.querySelector('[data-testid="active"]')!.textContent).toBe('a1');
  });
});
