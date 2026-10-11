// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, createRef } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { draftStore } from '../drafts';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import { ThreadHostContext } from './threadHost';
import type { ThreadHost } from './threadHost';

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
  draftStore.reset();
});

function render(
  host: ThreadHost,
  onComposingChange?: (composing: boolean) => void,
  post: PresentApi['post'] = async () => true,
): React.RefObject<FeedbackHandle | null> {
  const present: PresentApi = { post, closed: false, currentRound: 1 };
  const feedbackRef = createRef<FeedbackHandle>();
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <ThreadHostContext.Provider value={host}>
          <FeedbackThread
            ref={feedbackRef}
            blockId="a1"
            feedback={[]}
            replies={[]}
            locked={false}
            addLabel="Add feedback"
            placeholder="Add feedback for the agent…"
            onComposingChange={onComposingChange}
          />
        </ThreadHostContext.Provider>
      </PresentContext.Provider>,
    ),
  );
  return feedbackRef;
}

function type(value: string): void {
  const textarea = container.querySelector('textarea')!;
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
  act(() => {
    setter.call(textarea, value);
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function remount(): void {
  act(() => root.unmount());
  root = createRoot(container);
}

function action(label: string): HTMLButtonElement {
  return [...container.querySelectorAll<HTMLButtonElement>('.feedback-actions button')].find(
    (b) => b.textContent === label,
  )!;
}

async function settle(run: () => void): Promise<void> {
  await act(async () => {
    run();
    await Promise.resolve();
    await Promise.resolve();
  });
}

function openAndMoveFocus(feedbackRef: React.RefObject<FeedbackHandle | null>): HTMLButtonElement {
  act(() => feedbackRef.current?.open());
  const other = document.createElement('button');
  document.body.appendChild(other);
  other.focus();
  return other;
}

describe('FeedbackThread composing latch', () => {
  it('reports composing only while the draft holds text, and releases on unmount', () => {
    let composing = false;
    const feedbackRef = render('rail', (c) => (composing = c));

    // An open, empty composer must not latch the host.
    act(() => feedbackRef.current?.open());
    expect(composing).toBe(false);

    type('hold the rail');
    expect(composing).toBe(true);

    type('');
    expect(composing).toBe(false);

    // Unmounting mid-draft (the sheet closing) must release the latch.
    type('half a thought');
    expect(composing).toBe(true);
    remount();
    expect(composing).toBe(false);
  });
});

describe('FeedbackThread unsent note', () => {
  it('restores the text when the post fails', async () => {
    const feedbackRef = render('inline', undefined, async () => false);
    act(() => feedbackRef.current?.open());
    type('lost on the wire');

    await settle(() => action('Send').click());

    expect(container.querySelector('textarea')?.value).toBe('lost on the wire');
    expect(container.querySelector('.feedback-item.pending')).toBeNull();
  });

  it('restores a failed note ahead of text typed since, even after the thread unmounted', async () => {
    let fail = (): void => {};
    const post: PresentApi['post'] = () => new Promise((resolve) => (fail = () => resolve(false)));
    const feedbackRef = render('inline', undefined, post);
    act(() => feedbackRef.current?.open());
    type('first');
    act(() => action('Send').click());
    expect(container.querySelector('textarea')).toBeNull();

    act(() => feedbackRef.current?.open());
    type('second');
    remount();
    await settle(fail);

    render('inline');
    expect(container.querySelector('textarea')?.value).toBe('first\nsecond');
  });

  it('keeps the text across a remount without taking focus', () => {
    const feedbackRef = render('rail');
    act(() => feedbackRef.current?.open());
    type('half a thought');
    remount();

    render('rail');

    const textarea = container.querySelector('textarea')!;
    expect(textarea.value).toBe('half a thought');
    expect(document.activeElement).not.toBe(textarea);
  });

  it('discards the text on Cancel', () => {
    const feedbackRef = render('inline');
    act(() => feedbackRef.current?.open());
    type('never mind');

    act(() => action('Cancel').click());

    expect(container.querySelector('textarea')).toBeNull();
    expect(draftStore.get('a1', 'note')).toBeUndefined();
    expect(container.querySelector('.feedback-affordance .link-btn')?.textContent).toBe('Add feedback');
  });
});

describe('FeedbackThread imperative composer', () => {
  it('refocuses an already-open rail composer on a repeated request', () => {
    const feedbackRef = render('rail');
    const other = openAndMoveFocus(feedbackRef);

    act(() => feedbackRef.current?.open());

    expect(document.activeElement).toBe(container.querySelector('textarea'));
    other.remove();
  });

  it('leaves an already-open inline composer unchanged on a repeated request', () => {
    const feedbackRef = render('inline');
    const other = openAndMoveFocus(feedbackRef);

    act(() => feedbackRef.current?.open());

    expect(document.activeElement).toBe(other);
    other.remove();
  });
});
