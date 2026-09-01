import { afterEach, describe, expect, it, vi } from 'vitest';
import { uuid } from './uuid';

const v4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

afterEach(() => {
  vi.unstubAllGlobals();
});

function withCrypto(overrides: Partial<Crypto>): void {
  const real = globalThis.crypto;
  vi.stubGlobal('crypto', {
    randomUUID: real.randomUUID.bind(real),
    getRandomValues: real.getRandomValues.bind(real),
    ...overrides,
  });
}

describe('uuid', () => {
  it('delegates to crypto.randomUUID in a secure context', () => {
    const randomUUID = vi.fn(() => '11111111-2222-4333-8444-555555555555' as const);
    withCrypto({ randomUUID });
    expect(uuid()).toBe('11111111-2222-4333-8444-555555555555');
    expect(randomUUID).toHaveBeenCalledOnce();
  });

  it('builds a v4 from getRandomValues when randomUUID is undefined', () => {
    withCrypto({ randomUUID: undefined as unknown as Crypto['randomUUID'] });
    expect(uuid()).toMatch(v4);
  });

  it('sets the version and variant bits over an all-zero source', () => {
    withCrypto({
      randomUUID: undefined as unknown as Crypto['randomUUID'],
      getRandomValues: (<T extends ArrayBufferView | null>(array: T): T => array) as Crypto['getRandomValues'],
    });
    expect(uuid()).toBe('00000000-0000-4000-8000-000000000000');
  });

  it('sets the version and variant bits over an all-ones source', () => {
    withCrypto({
      randomUUID: undefined as unknown as Crypto['randomUUID'],
      getRandomValues: (<T extends ArrayBufferView | null>(array: T): T => {
        new Uint8Array((array as ArrayBufferView).buffer).fill(0xff);
        return array;
      }) as Crypto['getRandomValues'],
    });
    expect(uuid()).toBe('ffffffff-ffff-4fff-bfff-ffffffffffff');
  });

  it('does not repeat', () => {
    withCrypto({ randomUUID: undefined as unknown as Crypto['randomUUID'] });
    const seen = new Set(Array.from({ length: 500 }, () => uuid()));
    expect(seen.size).toBe(500);
  });
});
