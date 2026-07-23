// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { emptyState } from '../reduce';
import type { Approval as ApprovalBlock } from '../schema';
import { Approval } from './Approval';
import { ThreadHostContext } from './threadHost';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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

function renderRail(block: ApprovalBlock): void {
  const interactions = emptyState().interactions;
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <ThreadHostContext.Provider value="rail">
          <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
            <Approval block={block} interactions={interactions} />
          </KeyboardProvider>
        </ThreadHostContext.Provider>
      </PresentContext.Provider>,
    ),
  );
}

describe('Approval rail comment chip', () => {
  it('places the chip in the decision bar after the verdict pair', () => {
    renderRail({ id: 'a1', type: 'approval' });
    const bar = container.querySelector('.decision-bar') as HTMLElement;
    expect([...bar.children].map((child) => child.className)).toEqual([
      'btn btn-ghost btn-lg verdict verdict-approve',
      'btn btn-ghost btn-lg verdict verdict-reject',
      'comment-chip-row',
    ]);
    expect(bar.querySelector(':scope > .comment-chip-row .comment-chip')?.textContent).toContain('Add feedback');
    expect(container.querySelector('.approval > .comment-chip-row')).toBeNull();
  });
});
