import { describe, expect, it } from 'vitest';
import { tokens } from './tokens';

const CONTRACT_KEYS = [
  'bg',
  'bgSoft',
  'surface',
  'surfaceRaised',
  'text',
  'dim',
  'border',
  'borderStrong',
  'accent',
  'accentFg',
  'ok',
  'warn',
  'danger',
  'focusRing',
  'radiusSm',
  'radiusMd',
  'radiusLg',
  'fontProse',
  'fontMono',
  'trackCaps',
];

describe('ui.tokens', () => {
  it('is frozen', () => {
    expect(Object.isFrozen(tokens)).toBe(true);
  });

  it('exposes exactly the 20 alias contract keys', () => {
    expect(Object.keys(tokens).sort()).toEqual([...CONTRACT_KEYS].sort());
  });

  it('maps every key to a var() reference, never a resolved value', () => {
    for (const value of Object.values(tokens)) {
      expect(value).toMatch(/^var\(--[a-z-]+\)$/);
    }
  });

  it('kebab-cases camelCase keys into their css var', () => {
    expect(tokens.bg).toBe('var(--bg)');
    expect(tokens.bgSoft).toBe('var(--bg-soft)');
    expect(tokens.surfaceRaised).toBe('var(--surface-raised)');
    expect(tokens.borderStrong).toBe('var(--border-strong)');
    expect(tokens.accentFg).toBe('var(--accent-fg)');
    expect(tokens.focusRing).toBe('var(--focus-ring)');
    expect(tokens.radiusSm).toBe('var(--radius-sm)');
    expect(tokens.fontProse).toBe('var(--font-prose)');
    expect(tokens.trackCaps).toBe('var(--track-caps)');
  });

  it('excludes tokens deliberately kept out of the contract', () => {
    expect(tokens).not.toHaveProperty('radiusPill');
    expect(tokens).not.toHaveProperty('shadowUp');
  });
});
