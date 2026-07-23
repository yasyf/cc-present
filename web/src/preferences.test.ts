import { describe, expect, it } from 'vitest';
import {
  loadTheme,
  loadView,
  PREFERENCES_KEY,
  PREFERENCES_SCHEMA,
  saveTheme,
  saveView,
} from './preferences';

class MemoryStorage {
  readonly values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }
}

const validEnvelope = () => ({ schema: PREFERENCES_SCHEMA, theme: 'dark', views: { alpha: 'focus' } });

describe('exact preference state v1', () => {
  it('uses product defaults only when state is missing', () => {
    const storage = new MemoryStorage();
    storage.setItem('cc-present:theme', 'dark');
    storage.setItem('cc-present:view:alpha', 'focus');

    expect(loadTheme(storage)).toBe('system');
    expect(loadView('alpha', storage)).toBeNull();
    expect(storage.getItem('cc-present:theme')).toBe('dark');
    expect(storage.getItem('cc-present:view:alpha')).toBe('focus');
    expect(storage.getItem(PREFERENCES_KEY)).toBeNull();
  });

  it('round-trips consolidated theme and subject views', () => {
    const storage = new MemoryStorage();
    saveTheme('dark', storage);
    saveView('beta', 'board', storage);
    saveView('alpha', 'focus', storage);

    expect(loadTheme(storage)).toBe('dark');
    expect(loadView('alpha', storage)).toBe('focus');
    expect(loadView('beta', storage)).toBe('board');
    expect(storage.getItem(PREFERENCES_KEY)).toBe(
      JSON.stringify({ schema: PREFERENCES_SCHEMA, theme: 'dark', views: { alpha: 'focus', beta: 'board' } }),
    );

    saveTheme('system', storage);
    expect(loadTheme(storage)).toBe('system');
    expect(storage.getItem(PREFERENCES_KEY)).not.toBeNull();
  });

  const invalid: Record<string, string> = {
    corrupt: '{',
    'foreign identity': JSON.stringify({ ...validEnvelope(), schema: { ...PREFERENCES_SCHEMA, identity: 'other-v1' } }),
    'wrong version': JSON.stringify({ ...validEnvelope(), schema: { ...PREFERENCES_SCHEMA, version: 2 } }),
    'wrong fingerprint': JSON.stringify({ ...validEnvelope(), schema: { ...PREFERENCES_SCHEMA, fingerprint: 'wrong' } }),
    'unknown envelope key': JSON.stringify({ ...validEnvelope(), legacy: true }),
    'unknown schema key': JSON.stringify({ ...validEnvelope(), schema: { ...PREFERENCES_SCHEMA, legacy: true } }),
    'missing theme': JSON.stringify({ schema: PREFERENCES_SCHEMA, views: {} }),
    'null theme': JSON.stringify({ ...validEnvelope(), theme: null }),
    'wrong theme type': JSON.stringify({ ...validEnvelope(), theme: 1 }),
    'null views': JSON.stringify({ ...validEnvelope(), views: null }),
    'wrong views type': JSON.stringify({ ...validEnvelope(), views: [] }),
    'wrong view value': JSON.stringify({ ...validEnvelope(), views: { alpha: 'grid' } }),
    'empty subject': JSON.stringify({ ...validEnvelope(), views: { '': 'focus' } }),
    'noncanonical key order': JSON.stringify({ theme: 'dark', schema: PREFERENCES_SCHEMA, views: { alpha: 'focus' } }),
    'noncanonical view order': JSON.stringify({ schema: PREFERENCES_SCHEMA, theme: 'dark', views: { beta: 'board', alpha: 'focus' } }),
    trailing: `${JSON.stringify(validEnvelope())}{}`,
    duplicate: JSON.stringify(validEnvelope()).replace('"theme":"dark"', '"theme":"dark","theme":"light"'),
  };

  for (const [name, raw] of Object.entries(invalid)) {
    it(`rejects ${name} without deleting or replacing it`, () => {
      const storage = new MemoryStorage();
      storage.setItem(PREFERENCES_KEY, raw);

      expect(() => loadTheme(storage)).toThrow();
      expect(() => loadView('alpha', storage)).toThrow();
      expect(() => saveTheme('light', storage)).toThrow();
      expect(() => saveView('alpha', 'board', storage)).toThrow();
      expect(storage.getItem(PREFERENCES_KEY)).toBe(raw);
    });
  }
});
