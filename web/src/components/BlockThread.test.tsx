// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, createRef } from 'react';
import type { RefObject } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { ActiveBlockProvider, useActiveBlock } from '../activeBlock';
import { draftStore } from '../drafts';
import { emptyState } from '../reduce';
import { revisionStore } from '../revision';
import type { Feedback } from '../events';
import { BlockThread } from './BlockThread';
import type { FeedbackHandle } from './FeedbackThread';
import { ThreadHostContext } from './threadHost';
import type { ThreadHost } from './threadHost';

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
  revisionStore.reset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  revisionStore.reset();
  draftStore.reset();
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

function render(
  host: ThreadHost,
  over: { type?: string; feedback?: Feedback[]; locked?: boolean } = {},
): RefObject<FeedbackHandle | null> {
  const ref = createRef<FeedbackHandle>();
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[]} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <ThreadHostContext.Provider value={host}>
              <BlockThread
                ref={ref}
                blockId="k1"
                type={over.type ?? 'card'}
                feedback={over.feedback ?? []}
                replies={[]}
                locked={over.locked ?? false}
              />
            </ThreadHostContext.Provider>
            <Active />
          </ActiveBlockProvider>
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
  return ref;
}

const chip = (): HTMLButtonElement | null => container.querySelector('.comment-chip');
const probe = (id: string): string | null => container.querySelector(`[data-testid="${id}"]`)!.textContent;

describe('BlockThread inline host', () => {
  it('renders the feedback thread with the note copy for its block type', () => {
    render('inline');
    expect(container.querySelector('.feedback-affordance .link-btn')!.textContent).toBe('Add note');
    expect(chip()).toBeNull();
    render('inline', { type: 'approval' });
    expect(container.querySelector('.feedback-affordance .link-btn')!.textContent).toBe('Add feedback');
  });

  it('raises the inline composer on open()', () => {
    const ref = render('inline');
    act(() => ref.current!.open());
    expect(container.querySelector('.feedback-editor textarea')).not.toBeNull();
    expect(probe('active')).toBe('none');
  });
});

describe('BlockThread rail host', () => {
  it('renders a chip in place of the thread', () => {
    render('rail');
    expect(chip()!.textContent).toBe('Add note');
    expect(container.querySelector('.feedback-affordance')).toBeNull();
  });

  it('renders nothing for a locked thread with no notes', () => {
    render('rail', { locked: true });
    expect(chip()).toBeNull();
    render('rail', { locked: true, feedback: [{ id: 'f1', text: 'kept' }] });
    expect(chip()!.textContent).toBe('1 comment');
  });

  it('reads Continue note while the block holds an unsent draft', () => {
    render('rail');
    act(() => draftStore.set('k1', 'note', 'half a thought'));
    expect(chip()!.textContent).toBe('Continue note');
    act(() => draftStore.clear('k1', 'note'));
    expect(chip()!.textContent).toBe('Add note');
  });

  it('pins its block and requests a composer on open()', () => {
    const ref = render('rail');
    expect(probe('active')).toBe('none');
    act(() => ref.current!.open());
    expect(probe('active')).toBe('k1');
    expect(probe('compose-epoch')).toBe('1');
    expect(probe('pinned-open')).toBe('true');
  });
});
