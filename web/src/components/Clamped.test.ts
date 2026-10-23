import { describe, expect, it } from 'vitest';
import { snapHeight } from './Clamped';

describe('snapHeight', () => {
  it('ends on the bottom of the last whole line, skipping paragraph gaps', () => {
    const boxes = [
      { top: 0, bottom: 20 },
      { top: 20, bottom: 40 },
      { top: 56, bottom: 76 },
      { top: 76, bottom: 96 },
    ];
    expect(snapHeight(boxes, 0, 3)).toBe(76);
  });

  it('groups fragments that share a line, taking the tallest', () => {
    const boxes = [
      { top: 0, bottom: 20 },
      { top: -2, bottom: 23 },
      { top: 22, bottom: 42 },
      { top: 42, bottom: 62 },
    ];
    expect(snapHeight(boxes, 0, 1)).toBe(23);
  });

  it('measures from the content origin', () => {
    expect(snapHeight([{ top: 100, bottom: 120 }, { top: 120, bottom: 140 }], 100, 1)).toBe(20);
  });

  it('is null when the text fits', () => {
    expect(snapHeight([{ top: 0, bottom: 20 }, { top: 20, bottom: 40 }], 0, 2)).toBeNull();
  });
});
