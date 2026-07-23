// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, createRef } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
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
});

function render(host: ThreadHost, onComposingChange?: (composing: boolean) => void): React.RefObject<FeedbackHandle | null> {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
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
    act(() => root.unmount());
    expect(composing).toBe(false);
    root = createRoot(container);
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
