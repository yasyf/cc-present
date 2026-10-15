// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { anchorScroll } from './scrollAnchor';

let resize: () => void;
let disconnect: ReturnType<typeof vi.fn>;

class ResizeObserverStub {
  constructor(cb: () => void) {
    resize = cb;
  }
  observe(): void {}
  disconnect(): void {
    disconnect();
  }
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

let target: HTMLElement;
let container: HTMLElement;
let scroll: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.useFakeTimers();
  disconnect = vi.fn();
  container = document.createElement('div');
  target = document.createElement('div');
  container.appendChild(target);
  document.body.appendChild(container);
  scroll = vi.fn();
  target.scrollIntoView = scroll;
});

afterEach(() => {
  vi.useRealTimers();
  container.remove();
});

describe('anchorScroll', () => {
  it('scrolls the target to the top, then again on each container resize', () => {
    anchorScroll(target, container, 3000);
    expect(scroll).toHaveBeenCalledWith({ block: 'start' });
    resize();
    resize();
    expect(scroll).toHaveBeenCalledTimes(3);
  });

  it('lets go at the first user scroll input', () => {
    anchorScroll(target, container, 3000);
    window.dispatchEvent(new Event('wheel'));
    expect(disconnect).toHaveBeenCalledOnce();
  });

  it('lets go once the bound elapses', () => {
    anchorScroll(target, container, 3000);
    vi.advanceTimersByTime(2999);
    expect(disconnect).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(disconnect).toHaveBeenCalledOnce();
    window.dispatchEvent(new Event('keydown'));
    expect(disconnect).toHaveBeenCalledOnce();
  });
});
