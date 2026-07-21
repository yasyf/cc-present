import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { loadPacks, resetLoadForTest } from './load';
import { getInteractivePackTypes, getPackComponent, getPackDefState, resetPacksForTest } from './registry';
import type { PackComponent } from './registry';
import type { PacksResponse } from './manifest';
import { resetTokenForTest } from '../token';

const RatingComponent: PackComponent = () => null;

function manifest(patch: { styles?: string } = {}): PacksResponse {
  return {
    hostApi: 1,
    dropped: [],
    packs: [
      {
        name: 'ex',
        version: '0.1.0',
        description: 'Example.',
        bundle: '/packs/ex/dist/pack.js?v=0.1.0',
        ...(patch.styles ? { styles: patch.styles } : {}),
        blocks: [
          { type: 'ex.rating', interactive: true, schema: {} },
          { type: 'ex.callout', interactive: false, schema: {} },
        ],
      },
    ],
  };
}

function fetchOk(resp: PacksResponse): typeof fetch {
  return vi.fn(async () => ({ ok: true, json: async () => resp })) as unknown as typeof fetch;
}

function fetchStatus(ok: boolean): typeof fetch {
  return vi.fn(async () => ({ ok, json: async () => ({}) })) as unknown as typeof fetch;
}

function fetchJson(body: unknown): typeof fetch {
  return vi.fn(async () => ({ ok: true, json: async () => body })) as unknown as typeof fetch;
}

type ImportFn = (url: string) => Promise<unknown>;

beforeEach(() => {
  resetPacksForTest();
  resetLoadForTest();
});

describe('loadPacks', () => {
  it('a non-ok manifest leaves dotted types resolving to unknown, importing nothing', async () => {
    const importFn = vi.fn<ImportFn>();
    await loadPacks(fetchStatus(false), importFn);
    expect(importFn).not.toHaveBeenCalled();
    expect(getPackDefState('ex.rating')).toBe('unknown');
  });

  it('a malformed 200 manifest resolves dotted types to unknown without perpetual loading', async () => {
    // A 200 body with no packs array must not reject the load promise (main.tsx
    // does `void loadPacks()`), and every dotted block must fall through to
    // unknown rather than loop on loading forever.
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const importFn = vi.fn<ImportFn>();
    await expect(loadPacks(fetchJson({ hostApi: 1 }), importFn)).resolves.toBeUndefined();
    expect(importFn).not.toHaveBeenCalled();
    expect(getPackDefState('ex.rating')).toBe('unknown');
    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });

  it('a manifest with the exact host api loads its packs', async () => {
    const importFn = vi.fn<ImportFn>(async () => ({ default: { hostApi: 1, blocks: { rating: RatingComponent } } }));
    await loadPacks(fetchJson({ ...manifest(), hostApi: 1 }), importFn);
    expect(importFn).toHaveBeenCalled();
    expect(getPackDefState('ex.rating')).toBe('ready');
    expect(getPackComponent('ex.rating')).toBe(RatingComponent);
  });

  it('a manifest with a retired host api registers nothing and resolves dotted types to unknown', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const importFn = vi.fn<ImportFn>();
    await loadPacks(fetchJson({ ...manifest(), hostApi: 2 }), importFn);
    expect(importFn).not.toHaveBeenCalled();
    expect(getPackDefState('ex.rating')).toBe('unknown');
    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });

  it('a fractional manifest host api registers nothing (Go requires an integer)', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const importFn = vi.fn<ImportFn>();
    await loadPacks(fetchJson({ ...manifest(), hostApi: 1.5 }), importFn);
    expect(importFn).not.toHaveBeenCalled();
    expect(getPackDefState('ex.rating')).toBe('unknown');
    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });

  it('a rejected bundle marks the pack failed and drops it from the ring', async () => {
    const importFn: ImportFn = () => Promise.reject(new Error('boom'));
    await loadPacks(fetchOk(manifest()), importFn);
    expect(getPackDefState('ex.rating')).toBe('failed');
    expect(getPackComponent('ex.rating')).toBeUndefined();
    expect(getInteractivePackTypes().has('ex.rating')).toBe(false);
  });

  it('a bundle with the exact host api loads', async () => {
    const importFn: ImportFn = async () => ({ default: { hostApi: 1, blocks: { rating: RatingComponent } } });
    await loadPacks(fetchOk(manifest()), importFn);
    expect(getPackDefState('ex.rating')).toBe('ready');
    expect(getPackComponent('ex.rating')).toBe(RatingComponent);
  });

  it('a bundle with a retired host api marks the pack failed', async () => {
    const importFn: ImportFn = async () => ({ default: { hostApi: 2, blocks: { rating: RatingComponent } } });
    await loadPacks(fetchOk(manifest()), importFn);
    expect(getPackDefState('ex.rating')).toBe('failed');
    expect(getPackComponent('ex.rating')).toBeUndefined();
  });

  it('a bundle with a fractional host api marks the pack failed (Go requires an integer)', async () => {
    const importFn: ImportFn = async () => ({ default: { hostApi: 1.5, blocks: { rating: RatingComponent } } });
    await loadPacks(fetchOk(manifest()), importFn);
    expect(getPackDefState('ex.rating')).toBe('failed');
    expect(getPackComponent('ex.rating')).toBeUndefined();
  });

  it('a missing export leaves the pack ready but the type without a component', async () => {
    const importFn: ImportFn = async () => ({ default: { hostApi: 1, blocks: {} } });
    await loadPacks(fetchOk(manifest()), importFn);
    expect(getPackDefState('ex.rating')).toBe('ready');
    expect(getPackComponent('ex.rating')).toBeUndefined();
    // The type stays interactive off the manifest even with no export.
    expect(getInteractivePackTypes().has('ex.rating')).toBe(true);
  });

  it('registers a component qualified off the manifest name, not the bundle key', async () => {
    // The bundle also exports a squatted namespace key; only the manifest's bare
    // name is honored, and never as another pack's type.
    const importFn: ImportFn = async () => ({
      default: { hostApi: 1, blocks: { rating: RatingComponent, callout: () => null, 'other.rating': () => null } },
    });
    await loadPacks(fetchOk(manifest()), importFn);
    expect(getPackComponent('ex.rating')).toBe(RatingComponent);
    expect(getPackDefState('ex.rating')).toBe('ready');
    expect(getPackComponent('other.rating')).toBeUndefined();
    expect([...getInteractivePackTypes()]).toEqual(['ex.rating']);
  });

  it('dedupes concurrent and repeat calls behind one singleton promise', async () => {
    const fetchFn = fetchOk(manifest());
    const importFn = vi.fn<ImportFn>(async () => ({ default: { hostApi: 1, blocks: { rating: RatingComponent } } }));
    await Promise.all([loadPacks(fetchFn, importFn), loadPacks(fetchFn, importFn)]);
    await loadPacks(fetchFn, importFn);
    expect(fetchFn).toHaveBeenCalledTimes(1);
    expect(importFn).toHaveBeenCalledTimes(1);
  });
});

describe('loadPacks style injection', () => {
  const links: { rel: string; href: string; dataset: Record<string, string> }[] = [];

  beforeEach(() => {
    links.length = 0;
    (globalThis as { document?: unknown }).document = {
      querySelector: () => null,
      createElement: () => ({ rel: '', href: '', dataset: {} as Record<string, string> }),
      head: { appendChild: (el: (typeof links)[number]) => links.push(el) },
    };
  });

  afterEach(() => {
    delete (globalThis as { document?: unknown }).document;
  });

  it('injects a stylesheet link for a pack that declares styles', async () => {
    const importFn: ImportFn = async () => ({ default: { hostApi: 1, blocks: { rating: RatingComponent } } });
    await loadPacks(fetchOk(manifest({ styles: '/packs/ex/dist/pack.css?v=0.1.0' })), importFn);
    expect(links).toHaveLength(1);
    expect(links[0]?.rel).toBe('stylesheet');
    expect(links[0]?.href).toBe('/packs/ex/dist/pack.css?v=0.1.0');
    expect(links[0]?.dataset.packStyle).toBe('/packs/ex/dist/pack.css?v=0.1.0');
  });
});

describe('loadPacks token propagation', () => {
  const links: { rel: string; href: string; dataset: Record<string, string> }[] = [];

  beforeEach(() => {
    links.length = 0;
    (globalThis as { document?: unknown }).document = {
      querySelector: () => null,
      createElement: () => ({ rel: '', href: '', dataset: {} as Record<string, string> }),
      head: { appendChild: (el: (typeof links)[number]) => links.push(el) },
    };
  });

  afterEach(() => {
    delete (globalThis as { document?: unknown }).document;
    delete (globalThis as { window?: unknown }).window;
    resetTokenForTest();
  });

  it('threads the page token through the manifest fetch, bundle import, and styles link', async () => {
    (globalThis as { window?: unknown }).window = { location: { search: '?token=T0' } };
    resetTokenForTest();
    const fetchFn = fetchOk(manifest({ styles: '/packs/ex/dist/pack.css?v=0.1.0' }));
    const importFn = vi.fn<ImportFn>(async () => ({ default: { hostApi: 1, blocks: { rating: RatingComponent } } }));
    await loadPacks(fetchFn, importFn);
    expect(fetchFn).toHaveBeenCalledWith('/api/packs?token=T0');
    expect(importFn).toHaveBeenCalledWith('/packs/ex/dist/pack.js?v=0.1.0&token=T0');
    expect(links[0]?.href).toBe('/packs/ex/dist/pack.css?v=0.1.0&token=T0');
    expect(links[0]?.dataset.packStyle).toBe('/packs/ex/dist/pack.css?v=0.1.0&token=T0');
  });

  it('leaves every request URL byte-identical when no token is present', async () => {
    resetTokenForTest();
    const fetchFn = fetchOk(manifest({ styles: '/packs/ex/dist/pack.css?v=0.1.0' }));
    const importFn = vi.fn<ImportFn>(async () => ({ default: { hostApi: 1, blocks: { rating: RatingComponent } } }));
    await loadPacks(fetchFn, importFn);
    expect(fetchFn).toHaveBeenCalledWith('/api/packs');
    expect(importFn).toHaveBeenCalledWith('/packs/ex/dist/pack.js?v=0.1.0');
    expect(links[0]?.href).toBe('/packs/ex/dist/pack.css?v=0.1.0');
  });
});
