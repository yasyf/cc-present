import { describe, expect, it } from 'vitest';
import { nextMode, resolveMode, type ThemeMode } from './theme';

describe('nextMode', () => {
  const cases: [ThemeMode, ThemeMode][] = [
    ['system', 'light'],
    ['light', 'dark'],
    ['dark', 'system'],
  ];
  it.each(cases)('cycles %s -> %s', (from, to) => {
    expect(nextMode(from)).toBe(to);
  });
});

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
