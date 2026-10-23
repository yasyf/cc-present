import { describe, expect, it } from 'vitest';
import { placePopover } from './refs';

describe('placePopover', () => {
  const size = { width: 400, maxHeight: 300 };
  const viewport = { width: 1000, height: 800 };

  it('opens below the reference when there is room', () => {
    expect(placePopover({ top: 100, bottom: 120, left: 50 }, viewport, size)).toEqual({ top: 126, left: 50, above: false });
  });

  it('flips above when the space below is short and above is larger', () => {
    expect(placePopover({ top: 700, bottom: 720, left: 50 }, viewport, size)).toEqual({ top: 694, left: 50, above: true });
  });

  it('keeps the popover inside the right edge', () => {
    expect(placePopover({ top: 100, bottom: 120, left: 900 }, viewport, size).left).toBe(592);
  });
});
