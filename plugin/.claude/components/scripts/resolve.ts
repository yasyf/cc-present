// Usage: bun scripts/resolve.ts [--refresh] [--write] <file|->

import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { PR_QUERY, cardFields, commitsFields, landedShaFrom, needsLandedSearch, selectPatch } from './pr';
import type { GhPullRequest } from './pr';

const CACHE_TTL_MS = 10 * 60 * 1000;
const MAX_PATCH_BYTES = 65536;

type Json = Record<string, unknown>;

interface PrRecord {
  pr: GhPullRequest;
  landedSha: string | null;
  fetchedAt: string;
}

interface DiffRecord {
  diff: string;
  fetchedAt: string;
}

const cacheDir = join(process.env.XDG_CACHE_HOME ?? join(homedir(), '.cache'), 'cc-present', 'pr');

function gh(args: string[]): string {
  return execFileSync('gh', args, { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 });
}

function cached<T>(key: string, refresh: boolean, fetch: () => T): T {
  const path = join(cacheDir, key);
  if (!refresh) {
    try {
      if (Date.now() - statSync(path).mtimeMs < CACHE_TTL_MS) return JSON.parse(readFileSync(path, 'utf8')) as T;
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== 'ENOENT') throw err;
    }
  }
  const value = fetch();
  mkdirSync(cacheDir, { recursive: true });
  writeFileSync(path, JSON.stringify(value));
  return value;
}

function cacheKey(repo: string, number: number, kind: string): string {
  return `${repo.replace('/', '__')}__${number}.${kind}.json`;
}

function fetchPr(repo: string, number: number, refresh: boolean): PrRecord {
  return cached(cacheKey(repo, number, 'pr'), refresh, () => {
    const [owner, name] = repo.split('/');
    const out = JSON.parse(
      gh(['api', 'graphql', '-F', `owner=${owner}`, '-F', `name=${name}`, '-F', `number=${number}`, '-f', `query=${PR_QUERY}`]),
    ) as { data: { repository: { pullRequest: GhPullRequest | null } } };
    const pr = out.data.repository.pullRequest;
    if (!pr) throw new Error(`${repo}#${number}: no such pull request`);
    let landedSha: string | null = null;
    if (needsLandedSearch(pr)) {
      const search = JSON.parse(
        gh(['api', '-X', 'GET', 'search/commits', '-f', `q=repo:${repo} "(#${number})"`, '-f', 'per_page=20']),
      ) as { items: { sha: string; commit: { message: string } }[] };
      landedSha = landedShaFrom(number, search.items);
    }
    return { pr, landedSha, fetchedAt: new Date().toISOString() };
  });
}

function fetchDiff(repo: string, number: number, refresh: boolean): DiffRecord {
  return cached(cacheKey(repo, number, 'diff'), refresh, () => ({
    diff: gh(['api', `repos/${repo}/pulls/${number}`, '-H', 'Accept: application/vnd.github.diff']),
    fetchedAt: new Date().toISOString(),
  }));
}

function prRef(block: Json): { repo: string; number: number } {
  const { id, repo, number } = block;
  if (typeof repo !== 'string' || typeof number !== 'number') {
    throw new Error(`${String(block.type)} ${String(id)}: needs repo and number`);
  }
  return { repo, number };
}

function resolveBlock(block: Json, refresh: boolean): Json {
  switch (block.type) {
    case 'pr.card': {
      const { repo, number } = prRef(block);
      const rec = fetchPr(repo, number, refresh);
      return { ...block, ...cardFields(rec.pr, rec.landedSha, rec.fetchedAt) };
    }
    case 'pr.commits': {
      const { repo, number } = prRef(block);
      const rec = fetchPr(repo, number, refresh);
      return { ...block, ...commitsFields(rec.pr, rec.fetchedAt) };
    }
    case 'pr.diff': {
      if (typeof block.patch === 'string') return block;
      const { repo, number } = prRef(block);
      const rec = fetchDiff(repo, number, refresh);
      const patch = selectPatch(rec.diff, {
        ...(Array.isArray(block.paths) ? { paths: block.paths as string[] } : {}),
        ...(typeof block.match === 'string' ? { match: block.match } : {}),
      });
      const where = `pr.diff ${String(block.id)} (${repo}#${number})`;
      if (patch === '') throw new Error(`${where}: paths and match select nothing`);
      const bytes = Buffer.byteLength(patch);
      if (bytes > MAX_PATCH_BYTES) {
        throw new Error(`${where}: selected patch is ${bytes} bytes, over the ${MAX_PATCH_BYTES}-byte cap; narrow paths or match`);
      }
      return { ...block, patch, fetched_at: rec.fetchedAt };
    }
    default:
      throw new Error(`${String(block.type)} ${String(block.id)}: not a pr pack block type`);
  }
}

// resolve fills every pr.* block in a board document, a block array, or one block.
export function resolve(node: unknown, refresh: boolean): unknown {
  if (Array.isArray(node)) return node.map((n) => resolve(n, refresh));
  if (node === null || typeof node !== 'object') return node;
  const obj = Object.fromEntries(Object.entries(node).map(([k, v]) => [k, resolve(v, refresh)]));
  return typeof obj.type === 'string' && obj.type.startsWith('pr.') ? resolveBlock(obj, refresh) : obj;
}

if (import.meta.main) {
  const args = process.argv.slice(2);
  const refresh = args.includes('--refresh');
  const write = args.includes('--write');
  const [file, ...extra] = args.filter((a) => !a.startsWith('--'));
  if (!file || extra.length > 0 || (write && file === '-')) {
    console.error('usage: bun scripts/resolve.ts [--refresh] [--write] <file|->');
    process.exit(2);
  }
  const input = readFileSync(file === '-' ? 0 : file, 'utf8');
  const out = `${JSON.stringify(resolve(JSON.parse(input), refresh), null, 2)}\n`;
  if (write) writeFileSync(file, out);
  else process.stdout.write(out);
}
