// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { draftStore } from '../drafts';
import { PresentContext } from '../present';
import { emptyState } from '../reduce';
import { TalkPane } from './TalkView';

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
  localStorage.clear();
});

async function pressEnter(post: (i: unknown) => Promise<boolean>, draft: string, shiftKey = false) {
  draftStore.set('talk', 'note', draft);
  act(() =>
    root.render(
      <PresentContext.Provider value={{ post, closed: false, currentRound: 1 }}>
        <TalkPane talkId="talk" entries={[]} interactions={emptyState().interactions} closed={false} />
      </PresentContext.Provider>,
    ),
  );
  const textarea = container.querySelector('textarea')!;
  await act(async () => {
    textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey, bubbles: true }));
  });
}

describe('TalkPane', () => {
  it('sends the draft as a note on the talk section at Enter', async () => {
    const post = vi.fn(async () => true);
    await pressEnter(post, 'how is the gateway?');
    expect(post).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'feedback.created', blockId: 'talk', text: 'how is the gateway?' }),
    );
    expect(draftStore.get('talk', 'note')).toBeUndefined();
  });

  it('keeps the draft on shift-enter', async () => {
    const post = vi.fn(async () => true);
    await pressEnter(post, 'two lines', true);
    expect(post).not.toHaveBeenCalled();
    expect(draftStore.get('talk', 'note')).toBe('two lines');
  });

  it('shows a sent note as sending until it echoes back', async () => {
    const post = vi.fn(async () => true);
    await pressEnter(post, 'on its way');
    expect(container.querySelector('.thread-item.pending')?.textContent).toContain('on its way');
  });

  it('puts the text back when the post fails', async () => {
    const post = vi.fn(async () => false);
    await pressEnter(post, 'lost?');
    expect(draftStore.get('talk', 'note')).toBe('lost?');
  });
});
