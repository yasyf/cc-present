// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import type { Chart } from '../schema';

// The resolved --accent differs by theme so a flip visibly re-inks the marks; jsdom
// can't resolve a CSS custom property, so the probe is mocked at the module boundary.
vi.mock('../cssColor', () => ({
  resolveColor: () =>
    document.documentElement.dataset.theme === 'dark' ? 'rgb(145, 163, 242)' : 'rgb(61, 86, 197)',
}));

import { ChartView } from './ChartView';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

type MqlListener = (e: { matches: boolean }) => void;
const mqlListeners = new Set<MqlListener>();
window.matchMedia = ((query: string) => ({
  get matches() {
    return false;
  },
  media: query,
  onchange: null,
  addEventListener: (_: string, l: MqlListener) => mqlListeners.add(l),
  removeEventListener: (_: string, l: MqlListener) => mqlListeners.delete(l),
  addListener: (l: MqlListener) => mqlListeners.add(l),
  removeListener: (l: MqlListener) => mqlListeners.delete(l),
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const chart = (kind: 'bar' | 'line', series: Chart['series'], title?: string): Chart => ({
  id: 'ch',
  type: 'chart',
  kind,
  title,
  unit: '%',
  categories: ['A', 'B', 'C'],
  series,
});

const twoSeries: Chart['series'] = [
  { label: 'One', values: [10, 20, 30] },
  { label: 'Two', values: [5, 15, 25] },
];

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  mqlListeners.clear();
  document.documentElement.removeAttribute('data-theme');
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  document.documentElement.removeAttribute('data-theme');
});

function renderChart(block: Chart): void {
  act(() => root.render(<ChartView block={block} />));
}

describe('ChartView', () => {
  it('renders an accessible bar chart: one bar per series per category, with a legend', () => {
    renderChart(chart('bar', twoSeries, 'Latency'));
    expect(container.querySelector('.chart-title')?.textContent).toBe('Latency');
    const svg = container.querySelector<SVGSVGElement>('svg.chart-svg');
    expect(svg?.getAttribute('role')).toBe('img');
    expect(svg?.querySelector('title')?.textContent).toBe('Latency');
    expect(svg?.querySelector('desc')?.textContent).toBe('bar chart, 3 categories, 2 series');
    expect(container.querySelectorAll('.chart-bar')).toHaveLength(6);
    expect(container.querySelectorAll('.chart-legend-item')).toHaveLength(2);
    expect(container.querySelector('.chart-line')).toBeNull();
  });

  it('titles each mark with its category, series, and unit-suffixed value', () => {
    renderChart(chart('bar', twoSeries));
    const first = container.querySelector('.chart-bar title');
    expect(first?.textContent).toBe('A · One: 10 %');
  });

  it('drops the legend for a single series', () => {
    renderChart(chart('bar', [{ label: 'Only', values: [1, 2, 3] }]));
    expect(container.querySelector('.chart-legend')).toBeNull();
    expect(container.querySelectorAll('.chart-bar')).toHaveLength(3);
  });

  it('renders a polyline and per-point dots for a line chart', () => {
    renderChart(chart('line', twoSeries));
    expect(container.querySelectorAll('.chart-line')).toHaveLength(2);
    expect(container.querySelectorAll('.chart-dot')).toHaveLength(6);
    expect(container.querySelector('.chart-bar')).toBeNull();
  });

  it('renders every bar with a positive width at the documented caps (100 × 6)', () => {
    const categories = Array.from({ length: 100 }, (_, i) => `c${i}`);
    const series = Array.from({ length: 6 }, (_, s) => ({
      label: `s${s}`,
      values: categories.map(() => 10),
    }));
    renderChart({ id: 'ch', type: 'chart', kind: 'bar', unit: 'ms', categories, series });
    const bars = [...container.querySelectorAll<SVGPathElement>('.chart-bar')];
    expect(bars).toHaveLength(600);
    for (const bar of bars) {
      const d = bar.getAttribute('d') ?? '';
      expect(d).not.toContain('NaN');
      const left = Number(d.match(/^M(-?[\d.]+),/)?.[1]);
      const rights = [...d.matchAll(/L(-?[\d.]+),/g)];
      const right = Number(rights.at(-1)?.[1]);
      expect(right - left).toBeGreaterThan(0);
    }
  });

  it('re-inks the marks when the resolved theme flips', async () => {
    renderChart(chart('bar', twoSeries));
    const accentBar = () => container.querySelector('.chart-bar')?.getAttribute('fill');
    expect(accentBar()).toBe('rgb(61, 86, 197)');
    await act(async () => {
      document.documentElement.dataset.theme = 'dark';
      await Promise.resolve();
    });
    expect(accentBar()).toBe('rgb(145, 163, 242)');
  });
});
