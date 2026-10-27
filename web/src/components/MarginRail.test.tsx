// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { MarginRail } from './MarginRail';
import { revisionStore } from '../revision';
import type { ThreadEntry } from '../threadFeed';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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
});

const entry = (blockId: string): ThreadEntry => ({
  blockId,
  type: 'approval',
  label: blockId,
  feedback: [],
  replies: [{ id: `r-${blockId}`, md: 'ok' }],
  locked: false,
  live: true,
  lastComment: 'ok',
});

describe('MarginRail', () => {
  it('flags the tick and the count of an entry with an unread reply, then clears them', () => {
    act(() => {
      root.render(
        <MarginRail
          open={false}
          projection={{ pinned: null, feed: [entry('a1'), entry('a2')] }}
          activeId={null}
          total={2}
          onToggle={() => {}}
          railRef={() => {}}
        >
          <div>Threads</div>
        </MarginRail>,
      );
    });
    expect(container.querySelector('[data-unread]')).toBeNull();

    act(() => {
      revisionStore.markLive();
      revisionStore.ingest({ schemaVersion: 1, type: 'reply.created', id: 'r2', blockId: 'a1', md: 'ok' }, undefined, {
        blockIds: [],
      });
    });
    const ticks = [...container.querySelectorAll('.rail-tick')];
    expect(ticks.map((tick) => tick.hasAttribute('data-unread'))).toEqual([true, false]);
    expect(container.querySelector('.rail-strip-count')!.hasAttribute('data-unread')).toBe(true);

    act(() => revisionStore.markRepliesRead('a1'));
    expect(container.querySelector('[data-unread]')).toBeNull();
  });
});
