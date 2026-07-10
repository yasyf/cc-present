import { describe, expect, it } from 'vitest';
import { undecidedKey } from './submit';

describe('undecidedKey', () => {
  it('is order-independent', () => {
    expect(undecidedKey(['a2', 'a1'])).toBe(undecidedKey(['a1', 'a2']));
  });

  it('changes when a same-count block swap replaces an undecided id', () => {
    expect(undecidedKey(['a1', 'a2'])).not.toBe(undecidedKey(['a1', 'a3']));
  });

  it('is empty when nothing is undecided', () => {
    expect(undecidedKey([])).toBe('');
  });
});
