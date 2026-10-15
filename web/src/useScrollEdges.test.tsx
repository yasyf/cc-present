// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { useScrollEdges } from './useScrollEdges';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

function Probe({ generation }: { generation: number }) {
  const { ref, edges } = useScrollEdges<HTMLDivElement>('x');
  return (
    <div
      key={generation}
      ref={ref}
      className="scroller"
      data-start={edges.atStart || undefined}
      data-end={edges.atEnd || undefined}
    />
  );
}

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

function setMetrics(el: HTMLElement, scrollLeft: number, scrollWidth: number, clientWidth: number): void {
  Object.defineProperty(el, 'scrollLeft', { configurable: true, get: () => scrollLeft });
  Object.defineProperty(el, 'scrollWidth', { configurable: true, get: () => scrollWidth });
  Object.defineProperty(el, 'clientWidth', { configurable: true, get: () => clientWidth });
  act(() => el.dispatchEvent(new Event('scroll')));
}

const scroller = (): HTMLElement => container.querySelector('.scroller') as HTMLElement;

describe('useScrollEdges', () => {
  it('tracks the replacement node after the scroller remounts', () => {
    act(() => root.render(<Probe generation={1} />));
    const first = scroller();
    act(() => root.render(<Probe generation={2} />));
    expect(scroller()).not.toBe(first);
    setMetrics(scroller(), 600, 1000, 400);
    expect(scroller().dataset.start).toBeUndefined();
    expect(scroller().dataset.end).toBe('true');
  });
});
