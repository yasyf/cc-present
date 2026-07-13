// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { TableView } from './TableView';
import type { Column, Table as TableBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// useScrollEdges constructs a ResizeObserver; jsdom ships none.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

const table = (rowCount: number, columns: Column[] = [{ key: 'a', label: 'A' }]): TableBlock => ({
  id: 't',
  type: 'table',
  columns,
  rows: Array.from({ length: rowCount }, (_, i) => ({ a: `r${i}` })),
});

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

function render(block: TableBlock): void {
  act(() => root.render(<TableView block={block} />));
}

// jsdom computes no layout, so scroll metrics default to 0; override them on the
// scroller and fire a scroll event to drive useScrollEdges, as other suites stub
// ResizeObserver.
function setMetrics(el: HTMLElement, scrollLeft: number, scrollWidth: number, clientWidth: number): void {
  Object.defineProperty(el, 'scrollLeft', { configurable: true, get: () => scrollLeft });
  Object.defineProperty(el, 'scrollWidth', { configurable: true, get: () => scrollWidth });
  Object.defineProperty(el, 'clientWidth', { configurable: true, get: () => clientWidth });
  act(() => el.dispatchEvent(new Event('scroll')));
}

const wrapper = (): HTMLElement => container.querySelector('.table-block') as HTMLElement;

describe('TableView row cap', () => {
  it('leaves a 12-row table uncapped (no sticky header)', () => {
    render(table(12));
    expect(wrapper().classList.contains('table-capped')).toBe(false);
  });

  it('caps past 12 rows, gating the sticky header', () => {
    render(table(13));
    expect(wrapper().classList.contains('table-capped')).toBe(true);
  });
});

describe('TableView tabular-nums', () => {
  it('sets tabular-nums on right-aligned columns only', () => {
    render(
      table(1, [
        { key: 'a', label: 'A', align: 'right' },
        { key: 'b', label: 'B' },
      ]),
    );
    const [right, left] = [...container.querySelectorAll('tbody td')] as HTMLElement[];
    expect(right!.style.fontVariantNumeric).toBe('tabular-nums');
    expect(left!.style.fontVariantNumeric).toBe('');
  });
});

describe('TableView edge fades', () => {
  it('shows no fade class when the content fits', () => {
    render(table(3));
    const w = wrapper();
    expect(w.classList.contains('fade-start')).toBe(false);
    expect(w.classList.contains('fade-end')).toBe(false);
  });

  it('fades the trailing edge when scrolled to the start of an overflow', () => {
    render(table(3));
    const w = wrapper();
    setMetrics(w, 0, 300, 100);
    expect(w.classList.contains('fade-start')).toBe(false);
    expect(w.classList.contains('fade-end')).toBe(true);
  });

  it('fades the leading edge when scrolled to the end', () => {
    render(table(3));
    const w = wrapper();
    setMetrics(w, 200, 300, 100);
    expect(w.classList.contains('fade-start')).toBe(true);
    expect(w.classList.contains('fade-end')).toBe(false);
  });

  it('fades both edges mid-scroll', () => {
    render(table(3));
    const w = wrapper();
    setMetrics(w, 100, 300, 100);
    expect(w.classList.contains('fade-start')).toBe(true);
    expect(w.classList.contains('fade-end')).toBe(true);
  });
});
