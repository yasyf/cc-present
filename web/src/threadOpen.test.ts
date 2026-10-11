// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act, createElement } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { hasThreadOpener, openThread, setThreadOpener, useThreadOpener } from './threadOpen';
import type { ThreadEntry } from './threadFeed';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const entry = (blockId: string): ThreadEntry => ({
  blockId,
  type: 'card',
  label: blockId,
  feedback: [],
  replies: [],
  locked: false,
  live: true,
  lastComment: null,
});

afterEach(() => setThreadOpener(null));

describe('thread opener', () => {
  it('throws before an opener is registered (fail loud)', () => {
    expect(hasThreadOpener()).toBe(false);
    expect(() => openThread(entry('k1'))).toThrow(/before the shell mounted/);
  });

  it('hands each entry to the registered opener in order', () => {
    const seen: string[] = [];
    setThreadOpener((e) => seen.push(e.blockId));
    expect(hasThreadOpener()).toBe(true);
    openThread(entry('k1'));
    openThread(entry('a1'));
    expect(seen).toEqual(['k1', 'a1']);
  });

  it('throws again once the opener is cleared', () => {
    const seen: string[] = [];
    setThreadOpener((e) => seen.push(e.blockId));
    setThreadOpener(null);
    expect(() => openThread(entry('k1'))).toThrow();
    expect(seen).toEqual([]);
  });
});

describe('useThreadOpener', () => {
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

  it('registers the opener while the shell is mounted', () => {
    const seen: string[] = [];
    function Shell() {
      useThreadOpener((e) => seen.push(e.blockId));
      return null;
    }
    act(() => root.render(createElement(Shell)));
    expect(hasThreadOpener()).toBe(true);
    openThread(entry('k1'));
    expect(seen).toEqual(['k1']);
  });

  it('clears the opener when the shell unmounts', () => {
    const local = createRoot(document.createElement('div'));
    function Shell() {
      useThreadOpener(() => {});
      return null;
    }
    act(() => local.render(createElement(Shell)));
    act(() => local.unmount());
    expect(hasThreadOpener()).toBe(false);
    expect(() => openThread(entry('k1'))).toThrow(/before the shell mounted/);
  });
});
