import { describe, expect, it } from 'vitest';
import { resolveMode } from './theme';

describe('resolveMode', () => {
  it('follows the system preference when mode is system', () => {
    expect(resolveMode('system', true)).toBe('dark');
    expect(resolveMode('system', false)).toBe('light');
  });

  it('ignores the system preference for an explicit mode', () => {
    expect(resolveMode('light', true)).toBe('light');
    expect(resolveMode('dark', false)).toBe('dark');
  });
});
