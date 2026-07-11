import { afterEach, describe, expect, it } from 'vitest';
import { resetTokenForTest, withToken } from './token';

function setSearch(search: string | undefined): void {
  if (search === undefined) {
    delete (globalThis as { window?: unknown }).window;
  } else {
    (globalThis as { window?: unknown }).window = { location: { search } };
  }
  resetTokenForTest();
}

afterEach(() => {
  delete (globalThis as { window?: unknown }).window;
  resetTokenForTest();
});

describe('withToken', () => {
  it('returns the path unchanged when the page URL carries no token', () => {
    setSearch('');
    expect(withToken('/api/packs')).toBe('/api/packs');
    expect(withToken('/packs/ex/dist/pack.js?v=0.1.0')).toBe('/packs/ex/dist/pack.js?v=0.1.0');
  });

  it('returns the path unchanged when window is absent', () => {
    setSearch(undefined);
    expect(withToken('/api/interactions')).toBe('/api/interactions');
  });

  it('appends ?token when the path has no query', () => {
    setSearch('?token=deadbeef');
    expect(withToken('/api/interactions')).toBe('/api/interactions?token=deadbeef');
  });

  it('appends &token when the path already has a query', () => {
    setSearch('?token=deadbeef');
    expect(withToken('/packs/ex/dist/pack.js?v=0.1.0')).toBe('/packs/ex/dist/pack.js?v=0.1.0&token=deadbeef');
    expect(withToken('/events?session=abc')).toBe('/events?session=abc&token=deadbeef');
  });

  it('reads the token once and ignores later URL changes', () => {
    setSearch('?token=first');
    expect(withToken('/api/packs')).toBe('/api/packs?token=first');
    (globalThis as { window?: unknown }).window = { location: { search: '?token=second' } };
    expect(withToken('/api/packs')).toBe('/api/packs?token=first');
  });

  it('percent-encodes a token with URL-significant characters', () => {
    setSearch(`?token=${encodeURIComponent('a b&c')}`);
    expect(withToken('/api/packs')).toBe('/api/packs?token=a%20b%26c');
  });
});
