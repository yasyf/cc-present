import { describe, expect, test } from 'bun:test';
import { project, toMarkdown, validate } from '../src/arch-map.js';
import type { ArchDoc } from '../src/arch-map.js';
import { validate as validateMap } from '../src/system-map.js';
import shop from './fixtures/arch-shop.json';

const doc = shop as ArchDoc;
const focuses = (d: ArchDoc): (string | null)[] => {
  const out: (string | null)[] = [null];
  const walk = (list: ArchDoc['nodes']) => list.forEach((n) => {
    if ((n.nodes || []).length) out.push(n.id);
    walk(n.nodes || []);
  });
  walk(d.nodes);
  return out;
};

describe('validate', () => {
  test('accepts the shop fixture', () => {
    expect(validate(doc)).toEqual([]);
  });

  test('rejects an id used twice across levels', () => {
    const bad = structuredClone(doc);
    bad.nodes[1]!.nodes![0]!.id = 'api';
    expect(validate(bad)).toContain('id "api" is used by a node and a node; ids are unique across the doc');
  });

  test('requires a medium on a nested store', () => {
    const bad = structuredClone(doc);
    delete bad.nodes[3]!.nodes![0]!.medium;
    expect(validate(bad).some((p) => p.startsWith('node "orders-table": a store node needs "medium"'))).toBe(true);
  });

  test('names an edge end that is not a node', () => {
    const bad = structuredClone(doc);
    bad.edges!.push({ from: 'http', to: 'nowhere' });
    expect(validate(bad)).toContain(`edge ${bad.edges!.length} "to" names "nowhere", which is not a node`);
  });

  test('refuses a note on an edge the doc does not draw', () => {
    const bad = structuredClone(doc);
    bad.notes!.push({ edge: { from: 'shopper', to: 'index' }, text: 'never drawn' });
    expect(validate(bad)).toContain(`note ${bad.notes!.length}: no edge joins "shopper" and "index"`);
  });
});

describe('project', () => {
  test('every level projects to a valid system map', () => {
    for (const f of focuses(doc)) expect([f, validateMap(project(doc, f), { derived: true })]).toEqual([f, []]);
  });

  test('the top level shows top nodes as blocks and their children as parts', () => {
    const map = project(doc, null);
    expect(map.blocks.map((b) => b.id)).toEqual(['shopper', 'web', 'api', 'db', 'relay', 'search', 'payments']);
    expect(map.blocks.find((b) => b.id === 'api')!.parts!.map((p) => p.id)).toEqual(['http', 'orders', 'outbox-writer']);
  });

  test('deep edges roll up to the visible part', () => {
    const map = project(doc, null);
    expect(map.links).toContainEqual({ from: 'orders', to: 'payments', label: 'charge the card', style: 'sync' });
    expect(map.links!.some((l) => l.from === 'validate' || l.to === 'price')).toBe(false);
  });

  test('zooming in turns outside ends into outside blocks', () => {
    const map = project(doc, 'api');
    expect(map.title).toBe('Orders API: Checks, charges, then records');
    expect(map.blocks.map((b) => [b.id, b.kind])).toEqual([
      ['http', 'service'],
      ['orders', 'service'],
      ['outbox-writer', 'service'],
      ['web', 'external'],
      ['payments', 'external'],
      ['db', 'external'],
    ]);
    expect(map.blocks.find((b) => b.id === 'orders')!.parts!.map((p) => p.id)).toEqual(['validate', 'price', 'reserve']);
    expect(map.links).toContainEqual({ from: 'web', to: 'http', label: 'GET /cart', style: 'sync' });
  });

  test('a level drops what happens entirely outside it', () => {
    const map = project(doc, 'relay');
    expect(map.stories!.map((s) => s.id)).toEqual(['search-catches-up']);
    expect(map.path).toBeUndefined();
  });

  test('a run of steps inside one box becomes one step on the box, and waitsFor follows its step', () => {
    const map = project(doc, null);
    expect(map.path!.steps.map((s) => [s.from, s.to, s.say, s.waitsFor ?? null])).toEqual([
      ['shopper', 'checkout-ui', 'tap Buy', null],
      ['checkout-ui', 'http', 'POST /orders', null],
      ['api', 'api', 'check the cart (and 1 more)', null],
      ['orders', 'payments', 'charge the card', null],
      ['orders', 'orders-table', 'write the order', null],
      ['http', 'checkout-ui', '201 Created', 5],
    ]);
  });

  test('zoomed in, the same run shows step by step', () => {
    expect(project(doc, 'api').path!.steps.map((s) => [s.from, s.to])).toEqual([
      ['web', 'http'],
      ['http', 'validate'],
      ['validate', 'price'],
      ['reserve', 'payments'],
      ['reserve', 'db'],
      ['http', 'web'],
    ]);
  });

  test('notes show only where their node or edge is drawn', () => {
    expect(project(doc, null).notes).toEqual([
      { on: 'outbox', text: 'events kept for 7 days' },
      { link: { from: 'publisher', to: 'indexer' }, text: 'about 2 s behind' },
    ]);
    expect(project(doc, 'api').notes).toEqual([{ on: 'reserve', text: 'stock held for 15 minutes' }]);
  });

  test('only the groups a level draws come along', () => {
    expect(project(doc, 'search').groups).toBeUndefined();
    expect(project(doc, null).groups!.map((g) => g.id)).toEqual(['phone', 'cloud']);
  });
});

describe('toMarkdown', () => {
  test('lists every node with its depth, edge and step', () => {
    const md = toMarkdown(doc);
    expect(md).toContain('# Bookshop');
    expect(md).toContain('    - **Reserver** (service): Holds stock and charges');
    expect(md).toContain('- Publisher to Indexer: order event (hands off without waiting)');
    expect(md).toContain('7. 201 Created (HTTP handler to Checkout), waits for step 6');
    expect(md).toContain('- Publisher to Indexer: about 2 s behind');
  });
});
