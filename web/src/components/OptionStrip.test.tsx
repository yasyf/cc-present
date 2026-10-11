import { describe, expect, it } from 'vitest';
import { inView } from './OptionStrip';

describe('inView', () => {
  const cases: { name: string; left: number; width: number; scrollLeft: number; clientWidth: number; want: boolean }[] = [
    { name: 'a card fully inside the window', left: 0, width: 200, scrollLeft: 0, clientWidth: 500, want: true },
    { name: 'a card exactly half past the trailing edge', left: 400, width: 200, scrollLeft: 0, clientWidth: 500, want: true },
    { name: 'a card peeking under half at the trailing edge', left: 420, width: 200, scrollLeft: 0, clientWidth: 500, want: false },
    { name: 'a card exactly half scrolled off the leading edge', left: 0, width: 200, scrollLeft: 100, clientWidth: 500, want: true },
    { name: 'a card mostly scrolled off the leading edge', left: 0, width: 200, scrollLeft: 120, clientWidth: 500, want: false },
    { name: 'a card wholly past the trailing edge', left: 600, width: 200, scrollLeft: 0, clientWidth: 500, want: false },
    { name: 'a card wholly before the leading edge', left: 0, width: 200, scrollLeft: 400, clientWidth: 500, want: false },
    { name: 'the last card at the scroll end', left: 880, width: 200, scrollLeft: 580, clientWidth: 500, want: true },
    { name: 'a zero-width card (no layout)', left: 0, width: 0, scrollLeft: 0, clientWidth: 0, want: true },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(inView(c.left, c.width, c.scrollLeft, c.clientWidth)).toBe(c.want);
    });
  }
});
