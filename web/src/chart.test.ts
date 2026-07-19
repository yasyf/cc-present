import { describe, expect, it } from 'vitest';
import { formatValue, niceScale, seriesColors } from './chart';

describe('niceScale', () => {
  it('anchors the baseline at 0 for positive-only data', () => {
    const scale = niceScale(20, 80);
    expect(scale.min).toBe(0);
    expect(scale.ticks).toContain(0);
    expect(scale).toEqual({ min: 0, max: 80, ticks: [0, 20, 40, 60, 80] });
  });

  it('extends the domain below 0 for negative data while still spanning 0', () => {
    expect(niceScale(-30, 40)).toEqual({ min: -40, max: 40, ticks: [-40, -20, 0, 20, 40] });
  });

  it('rounds a ragged max up to a clean tick', () => {
    expect(niceScale(0, 95)).toEqual({ min: 0, max: 100, ticks: [0, 20, 40, 60, 80, 100] });
  });

  it('keeps an all-zero series from collapsing, still including 0', () => {
    const scale = niceScale(0, 0);
    expect(scale.min).toBe(0);
    expect(scale.max).toBeGreaterThan(0);
    expect(scale.ticks).toContain(0);
  });

  it('emits fractional ticks free of floating-point noise', () => {
    expect(niceScale(0, 1).ticks).toEqual([0, 0.2, 0.4, 0.6, 0.8, 1]);
  });
});

describe('formatValue', () => {
  it('compacts large magnitudes', () => {
    expect(formatValue(1284)).toBe('1.3K');
    expect(formatValue(4200000)).toBe('4.2M');
  });

  it('renders small values verbatim', () => {
    expect(formatValue(-30)).toBe('-30');
    expect(formatValue(0)).toBe('0');
  });

  it('appends the optional unit with a separating space', () => {
    expect(formatValue(80, '%')).toBe('80 %');
    expect(formatValue(12900, 'req')).toBe('12.9K req');
  });
});

describe('seriesColors', () => {
  it('is deterministic for the same inputs', () => {
    expect(seriesColors('#3d56c5', 'light', 6)).toEqual(seriesColors('#3d56c5', 'light', 6));
  });

  it('places the accent verbatim in slot 1 and derives the rest', () => {
    const colors = seriesColors('#3d56c5', 'light', 4);
    expect(colors).toHaveLength(4);
    expect(colors[0]).toBe('#3d56c5');
  });

  it('rotates a fixed light palette off the accent hue', () => {
    expect(seriesColors('#3d56c5', 'light', 6)).toEqual([
      '#3d56c5',
      '#c86269',
      '#0092c7',
      '#a58100',
      '#9970c4',
      '#1c9e6d',
    ]);
  });

  it('re-steps the derived slots for the dark surface', () => {
    expect(seriesColors('#91a3f2', 'dark', 6)).toEqual([
      '#91a3f2',
      '#c25d5d',
      '#098ac4',
      '#997d00',
      '#9768ba',
      '#00976e',
    ]);
  });

  it('accepts a resolved rgb() accent verbatim', () => {
    expect(seriesColors('rgb(61, 86, 197)', 'light', 2)).toEqual(['rgb(61, 86, 197)', '#c86269']);
  });
});
