// The pack loader: one fetch of GET /api/packs on boot, then a dynamic import of
// each bundle into the registry. A module-scope singleton promise makes it
// StrictMode/HMR-safe (the second call reuses the first). Every failure is
// per-pack fail-soft — a bad bundle marks that pack failed and the rest load.

import { markFailed, markPacksLoaded, registerPack } from './registry';
import type { PackComponent } from './registry';
import type { PackInfo, PacksResponse } from './manifest';
import { withToken } from '../token';

type FetchFn = typeof fetch;
type ImportFn = (url: string) => Promise<unknown>;

interface PackModule {
  default?: { hostApi?: unknown; blocks?: Record<string, unknown> };
}

let loadPromise: Promise<void> | null = null;

// loadPacks fetches the manifest and imports every bundle, once per page. The
// injectable fetch/import let tests drive it without a network or real modules.
export function loadPacks(
  fetchFn: FetchFn = fetch,
  importFn: ImportFn = (url) => import(/* @vite-ignore */ url),
): Promise<void> {
  if (!loadPromise) loadPromise = doLoad(fetchFn, importFn);
  return loadPromise;
}

export function resetLoadForTest(): void {
  loadPromise = null;
}

async function doLoad(fetchFn: FetchFn, importFn: ImportFn): Promise<void> {
  const packs = await loadManifest(fetchFn);
  markPacksLoaded();
  await Promise.all(packs.map((def) => importPack(def, importFn)));
}

// loadManifest fetches GET /api/packs and registers every pack it declares. It
// never rejects: a network error or non-ok status yields no packs silently, and a
// contract-violating 200 body is warned once and also yields none. The caller then
// markPacksLoaded so unresolved dotted blocks fall through to "unknown".
async function loadManifest(fetchFn: FetchFn): Promise<PackInfo[]> {
  let body: unknown;
  try {
    const res = await fetchFn(withToken('/api/packs'));
    if (!res.ok) return [];
    body = await res.json();
  } catch {
    return [];
  }
  if (!isPacksResponse(body)) {
    console.warn('cc-present: /api/packs returned an unusable manifest; pack blocks render as unknown', body);
    return [];
  }
  for (const def of body.packs) registerPack(def);
  return body.packs;
}

// isPacksResponse gates the whole manifest before any pack registers, so a
// contract-violating body registers nothing rather than a partial set.
function isPacksResponse(body: unknown): body is PacksResponse {
  return isPlainObject(body) && body.hostApi === 1 && Array.isArray(body.packs) && body.packs.every(isPackInfo);
}

function isPackInfo(def: unknown): def is PackInfo {
  return (
    isPlainObject(def) &&
    typeof def.name === 'string' &&
    typeof def.bundle === 'string' &&
    Array.isArray(def.blocks) &&
    def.blocks.every(isPlainObject)
  );
}

async function importPack(def: PackInfo, importFn: ImportFn): Promise<void> {
  try {
    const mod = (await importFn(withToken(def.bundle))) as PackModule;
    const bundle = mod.default;
    if (!bundle || bundle.hostApi !== 1 || !isPlainObject(bundle.blocks)) {
      markFailed(def.name);
      return;
    }
    // Qualify components off the MANIFEST's declared types, never the bundle's
    // own keys, so a bundle cannot register a type it did not declare.
    const bundleBlocks = bundle.blocks;
    const components: Record<string, PackComponent> = {};
    for (const bt of def.blocks) {
      const bare = bt.type.slice(bt.type.indexOf('.') + 1);
      const comp = bundleBlocks[bare];
      if (isComponent(comp)) components[bare] = comp as PackComponent;
    }
    if (def.styles) injectStyles(withToken(def.styles));
    registerPack(def, components);
  } catch {
    markFailed(def.name);
  }
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null;
}

function isComponent(v: unknown): boolean {
  return typeof v === 'function' || isPlainObject(v);
}

function injectStyles(href: string): void {
  if (document.querySelector(`link[data-pack-style="${href}"]`)) return;
  const link = document.createElement('link');
  link.rel = 'stylesheet';
  link.href = href;
  link.dataset.packStyle = href;
  document.head.appendChild(link);
}
