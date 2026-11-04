// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
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

describe('TableView rows', () => {
  it('renders every row of a long table on the page', () => {
    render(table(40));
    expect(container.querySelectorAll('tbody tr')).toHaveLength(40);
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

describe('a table worked with in place', () => {
  const working: TableBlock = {
    id: 't',
    type: 'table',
    columns: [
      { key: 'team', label: 'Team', filter: true, sort: true },
      { key: 'behind', label: 'Behind', sort: true },
    ],
    rows: [
      { team: 'SoFi', behind: '10' },
      { team: 'AIG', behind: '9' },
    ],
    rowKey: 'team',
    details: { AIG: 'Waiting on a **restart**.' },
    actions: [{ id: 'why', label: 'Why?', target: 'talk' }],
  };
  const firstCells = () => [...container.querySelectorAll('tbody tr td:first-child')].map((td) => td.textContent);

  function renderWorking(post: (i: unknown) => Promise<boolean> = async () => true) {
    act(() =>
      root.render(
        <PresentContext.Provider value={{ post, closed: false, currentRound: 1 }}>
          <TableView block={working} />
        </PresentContext.Provider>,
      ),
    );
  }

  it('sorts when a header is pressed, then reverses', () => {
    renderWorking();
    const behind = container.querySelectorAll<HTMLButtonElement>('.table-sort')[1]!;
    act(() => behind.click());
    expect(firstCells()).toEqual(['AIG', 'SoFi']);
    act(() => behind.click());
    expect(firstCells()).toEqual(['SoFi', 'AIG']);
  });

  it('narrows to a chip and restores when pressed again', () => {
    renderWorking();
    const chip = [...container.querySelectorAll<HTMLButtonElement>('.table-chip')].find((b) => b.textContent === 'AIG')!;
    act(() => chip.click());
    expect(firstCells()).toEqual(['AIG']);
    act(() => chip.click());
    expect(firstCells()).toEqual(['SoFi', 'AIG']);
  });

  it('opens a row to its detail, and an action posts a note naming the row on its target', () => {
    const post = vi.fn(async () => true);
    renderWorking(post);
    const row = [...container.querySelectorAll<HTMLTableRowElement>('.table-row-opens')].find((r) => r.textContent?.includes('AIG'))!;
    act(() => row.click());
    expect(container.querySelector('.table-row-detail')?.textContent).toContain('restart');
    act(() => container.querySelector<HTMLButtonElement>('.table-row-actions button')!.click());
    expect(post).toHaveBeenCalledWith(expect.objectContaining({ type: 'feedback.created', blockId: 'talk', text: 'Why?: AIG' }));
  });
});
