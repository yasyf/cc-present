import { describe, expect, test } from 'bun:test';
import example from '../examples/system.json';
import { artifactDocument, isTimestamp, panelProblems, splitMermaid, stepCount, wrapLabel } from '../src/logic';
import { validate } from '../src/system-map.js';
import type { SystemMap } from '../src/system-map.js';
import dense from './fixtures/dense.json';
import sessions from './fixtures/sessions.json';
import storage from './fixtures/storage.json';
import tiny from './fixtures/tiny.json';
import web from './fixtures/web.json';

describe('splitMermaid', () => {
  test('splits prose around mermaid fences', () => {
    const md = 'Intro\n\n```mermaid\nflowchart LR\n  a --> b\n```\n\nOutro\n';
    expect(splitMermaid(md)).toEqual([
      { kind: 'md', text: 'Intro\n\n' },
      { kind: 'mermaid', source: 'flowchart LR\n  a --> b' },
      { kind: 'md', text: '\n\nOutro\n' },
    ]);
  });

  test('leaves other fences to markdown', () => {
    const md = '```ts\nconst a = 1;\n```';
    expect(splitMermaid(md)).toEqual([{ kind: 'md', text: md }]);
  });

  test('handles adjacent diagrams with no prose', () => {
    const md = '```mermaid\ngraph TD\n  a\n```\n```mermaid\ngraph TD\n  b\n```';
    expect(splitMermaid(md)).toEqual([
      { kind: 'mermaid', source: 'graph TD\n  a' },
      { kind: 'mermaid', source: 'graph TD\n  b' },
    ]);
  });
});

describe('panelProblems', () => {
  const actors = [
    { id: 'api', label: 'api' },
    { id: 'ex', label: 'executor' },
  ];

  test('accepts known actors and notes without a target', () => {
    expect(panelProblems({ actors, steps: [{ from: 'api', to: 'ex', label: 'start' }, { from: 'ex', label: 'run' }] }, 0)).toEqual([]);
  });

  test('names unknown and duplicate actors', () => {
    const panel = { actors: [...actors, { id: 'api', label: 'again' }], steps: [{ from: 'api', to: 'db', label: 'x' }] };
    expect(panelProblems(panel, 1)).toEqual(['panel 2: duplicate actor "api"', 'panel 2 step 1: unknown actor "db"']);
  });
});

test('stepCount is the longest panel', () => {
  const step = { from: 'a', label: 'x' };
  const actors = [{ id: 'a', label: 'a' }];
  expect(stepCount([{ actors, steps: [step] }, { actors, steps: [step, step, step] }])).toBe(3);
});

describe('artifactDocument', () => {
  test('appends a height reporter keyed by the frame id to html', () => {
    const doc = artifactDocument('html', '<p>hi</p>', 'blk"1');
    expect(doc.startsWith('<p>hi</p><script>')).toBe(true);
    expect(doc).toContain('ccPresentArtifact:"blk\\"1"');
    expect(doc).toContain('new ResizeObserver(post)');
  });

  test('wraps svg in a page that scales it', () => {
    const doc = artifactDocument('svg', '<svg viewBox="0 0 1 1"></svg>', 'a');
    expect(doc.startsWith('<!doctype html>')).toBe(true);
    expect(doc).toContain('<body><svg viewBox="0 0 1 1"></svg><script>');
    expect(doc.endsWith('</body></html>')).toBe(true);
  });
});

describe('wrapLabel', () => {
  const measure = (s: string) => s.length * 10;

  test('keeps a label that fits on one line', () => {
    expect(wrapLabel('Database', 100, measure)).toEqual({ lines: ['Database'], clipped: false });
  });

  test('wraps words onto further lines', () => {
    expect(wrapLabel('Run assignment', 100, measure)).toEqual({ lines: ['Run', 'assignment'], clipped: false });
  });

  test('hyphenates a word wider than the line', () => {
    expect(wrapLabel('Database', 60, measure)).toEqual({ lines: ['Datab-', 'ase'], clipped: false });
  });

  test('leaves at least two letters after a hyphen', () => {
    const narrowHyphen = (s: string) => s.replaceAll('-', '').length * 10 + (s.includes('-') ? 5 : 0);
    expect(wrapLabel('Database', 78, narrowHyphen)).toEqual({ lines: ['Databa-', 'se'], clipped: false });
  });

  test('clips overflow past the last line with an ellipsis', () => {
    expect(wrapLabel('Read the executor binding now', 100, measure, 2)).toEqual({ lines: ['Read the', 'executor…'], clipped: true });
  });

  test('clips a word too narrow to hyphenate', () => {
    expect(wrapLabel('Supercalifragilistic', 30, measure, 1)).toEqual({ lines: ['Su…'], clipped: true });
  });
});

describe('system map validate', () => {
  const base = (): SystemMap => ({
    title: 'Orders reach disk first',
    groups: [{ id: 'vm', label: 'Orders VM', kind: 'machine' }],
    blocks: [
      { id: 'app', kind: 'app', name: 'Shop app', role: 'places orders' },
      { id: 'api', kind: 'service', name: 'Orders API', role: 'checks then replies', group: 'vm', parts: [{ id: 'edge', shape: 'api', name: 'Edge', role: 'takes requests' }] },
      { id: 'log', kind: 'log', name: 'Order log', role: 'keeps every order', medium: 'disk', group: 'vm', critical: true },
    ],
    links: [
      { from: 'app', to: 'edge', label: 'POST' },
      { from: 'edge', to: 'log', label: 'append' },
    ],
    path: {
      title: 'An order',
      steps: [
        { from: 'app', to: 'api', say: 'send' },
        { from: 'api', to: 'log', say: 'append' },
        { from: 'api', to: 'app', say: 'reply', waitsFor: 2 },
      ],
    },
    stories: [{ id: 'retry', label: 'A retry', steps: [{ from: 'app', to: 'api', say: 'again', mark: 'risk' }] }],
  });
  const ten = Array.from({ length: 10 }, (_, i) => ({ id: `b${i}`, kind: 'service' as const, name: `Box ${i}`, role: 'does one thing' }));
  const pairs = ten.flatMap((a, i) => ten.slice(i + 1).map((b) => ({ from: a.id, to: b.id }))).slice(0, 13);

  test('accepts the example and every board fixture', () => {
    for (const map of [example, tiny, storage, web, dense, sessions, base()]) expect(validate(map)).toEqual([]);
  });

  const broken: [string, (m: SystemMap) => void, string][] = [
    ['ids are unique', (m) => m.stories?.push({ id: 'app', label: 'Twice', steps: [{ from: 'app', to: 'api', say: 'x' }] }), 'id "app" is used twice; ids are unique across groups, blocks, parts and stories'],
    ['a process group runs on a machine', (m) => m.groups?.push({ id: 'proc', label: 'Worker', kind: 'process' }), 'group "proc": a process group needs "in" naming the machine group it runs on'],
    ['only a process group takes in', (m) => m.groups?.push({ id: 'zone', label: 'Zone', in: 'vm' }), 'group "zone": only a process group takes "in"'],
    ['link ends name known ids', (m) => m.links?.splice(1, 1, { from: 'edge', to: 'walz' }), 'link 2: "to" names "walz", which is not a block or part'],
    ['start.block has parts', (m) => { m.start = { block: 'app' }; }, 'start: block "app" has no parts to open'],
    ['medium only on stores', (m) => { m.blocks[0] = { ...m.blocks[0]!, medium: 'disk' }; }, 'block "app": only store and log blocks take "medium"'],
    ['every part is linked', (m) => m.blocks[1]?.parts?.push({ id: 'cache', shape: 'cache', name: 'Cache', role: 'recent orders' }), 'part "cache" in block "api" is in no link or step'],
    ['no link joins a block to its own part', (m) => m.links?.push({ from: 'api', to: 'edge' }), 'link 3: joins block "api" to one of its own parts'],
    ['at most 12 arrows', (m) => Object.assign(m, { blocks: ten, groups: [], links: pairs, path: undefined, stories: [] }), 'the links roll up to 13 arrows between blocks; keep it to 12'],
    ['waitsFor names an earlier step', (m) => m.path?.steps.splice(0, 1, { from: 'app', to: 'api', say: 'send', waitsFor: 1 }), 'path step 1: "waitsFor" is 1, which is not an earlier step'],
    ['waitsFor sits on an arrow between blocks', (m) => m.path?.steps.splice(2, 1, { from: 'api', to: 'edge', say: 'reply', waitsFor: 2 }), 'path step 3: "waitsFor" needs an arrow to sit on, but this step stays inside block "api"'],
    ['mark only on stories', (m) => m.path?.steps.splice(0, 1, { from: 'app', to: 'api', say: 'send', mark: 'stop' }), 'path step 1: "mark" belongs only on story steps'],
    ['story labels are unique', (m) => m.stories?.push({ id: 'again', label: 'A retry', steps: [{ from: 'app', to: 'api', say: 'x' }] }), 'story label "A retry" is used twice'],
  ];
  for (const [rule, breakIt, sentence] of broken) {
    test(rule, () => {
      const m = base();
      breakIt(m);
      expect(validate(m)).toEqual([sentence]);
    });
  }
});

describe('isTimestamp', () => {
  test.each(['Sep 12', 'Oct 3, 2026', '2026-09-12 14:03 PT', '9/12', '14:03', 'March 4'])('reads %p as a timestamp', (when) => {
    expect(isTimestamp(when)).toBe(true);
  });

  test.each(['Release 6', 'Step 2', 'Marketing 3', 'v0.52'])('reads %p as a label', (when) => {
    expect(isTimestamp(when)).toBe(false);
  });
});
