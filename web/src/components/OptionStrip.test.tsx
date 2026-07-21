import { describe, expect, it } from 'vitest';
import { activeCard } from './OptionStrip';

describe('activeCard', () => {
  const cases: { name: string; scrollLeft: number; cardWidth: number; want: number }[] = [
    { name: 'start of the strip', scrollLeft: 0, cardWidth: 200, want: 0 },
    { name: 'just short of the second card rounds back', scrollLeft: 80, cardWidth: 200, want: 0 },
    { name: 'past the midpoint rounds to the second card', scrollLeft: 120, cardWidth: 200, want: 1 },
    { name: 'exactly on the third card', scrollLeft: 400, cardWidth: 200, want: 2 },
    { name: 'a half-stride rounds up', scrollLeft: 300, cardWidth: 200, want: 2 },
    { name: 'a zero stride (no layout) is the first card', scrollLeft: 0, cardWidth: 0, want: 0 },
    { name: 'a negative overscroll clamps to the first card', scrollLeft: -40, cardWidth: 200, want: 0 },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(activeCard(c.scrollLeft, c.cardWidth)).toBe(c.want);
    });
  }
});
