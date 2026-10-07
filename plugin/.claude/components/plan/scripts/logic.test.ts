import { describe, expect, test } from 'bun:test';
import {
  arrowHead,
  callCounts,
  callsProblems,
  flattenCalls,
  layoutMachine,
  mockCsp,
  mockDocument,
  neutralize,
  pinSpots,
  setComment,
  struckBy,
  toggleStruck,
  validateMachine,
} from '../src/logic';
import type { CallRow, MachineSpec } from '../src/logic';
import machineExample from '../examples/machine.json';

const tree: CallRow[] = [
  {
    call: 'root()',
    mark: '~',
    calls: [
      { id: 'a', call: 'a()', mark: '+', calls: [{ call: 'a1()', mark: '+' }] },
      { call: 'b()', mark: '-' },
    ],
  },
  { call: 'other()', mark: '?' },
];

describe('flattenCalls', () => {
  test('keys rows by id, else by index path, with tree glyphs', () => {
    expect(flattenCalls(tree).map((f) => [f.key, f.glyph, f.root, f.parent])).toEqual([
      ['0', '', 0, null],
      ['a', '├ ', 0, '0'],
      ['0.0.0', '│ └ ', 0, 'a'],
      ['0.1', '└ ', 0, '0'],
      ['1', '', 1, null],
    ]);
  });

  test('names duplicate keys', () => {
    expect(callsProblems([{ id: 'x', call: 'x', mark: '+' }, { id: 'x', call: 'y', mark: '+' }])).toEqual(['duplicate row id "x"']);
    expect(callsProblems(tree)).toEqual([]);
  });

  test('counts rows by mark', () => {
    expect(callCounts(tree)).toEqual({ added: 2, removed: 1, changed: 1, proposed: 1 });
  });
});

describe('striking', () => {
  test('a struck row takes its subtree', () => {
    const flat = flattenCalls(tree);
    expect([...struckBy(flat, new Set(['a']))]).toEqual([['0.0.0', 'a']]);
    expect([...struckBy(flat, new Set(['0'])).keys()]).toEqual(['a', '0.0.0', '0.1']);
  });

  test('toggles a key in and out', () => {
    expect(toggleStruck([], 'a')).toEqual(['a']);
    expect(toggleStruck(['a', 'b'], 'a')).toEqual(['b']);
  });
});

const base: MachineSpec = {
  states: [{ id: 'idle' }, { id: 'busy' }, { id: 'done', final: true }],
  transitions: [
    { from: 'idle', event: 'go', to: 'busy' },
    { from: 'busy', event: 'ok', to: 'done' },
  ],
};

describe('validateMachine', () => {
  test('accepts the example and a plain machine', () => {
    expect(validateMachine(machineExample as MachineSpec)).toEqual([]);
    expect(validateMachine(base)).toEqual([]);
  });

  test('names unknown ids before graph checks', () => {
    expect(validateMachine({ ...base, initial: 'nope', transitions: [{ from: 'idle', event: 'x', to: 'ghost' }] })).toEqual([
      'initial state "nope" is not declared',
      'transition 1 (idle -x-> ghost): unknown state "ghost"',
    ]);
  });

  test('names unreachable states and non-final dead ends', () => {
    const m: MachineSpec = { ...base, states: [...base.states, { id: 'lost' }], transitions: [base.transitions[0]!] };
    expect(validateMachine(m)).toEqual([
      'state "busy" has no way out and is not final',
      'state "done" is unreachable from "idle"',
      'state "lost" is unreachable from "idle"',
      'state "lost" has no way out and is not final',
    ]);
  });

  test('a removed transition does not count and a removed state is exempt', () => {
    const m: MachineSpec = {
      states: [{ id: 'idle' }, { id: 'old', mark: '-' }, { id: 'done', final: true }],
      transitions: [
        { from: 'idle', event: 'go', to: 'done' },
        { from: 'idle', event: 'legacy', to: 'old', mark: '-' },
      ],
    };
    expect(validateMachine(m)).toEqual([]);
    expect(validateMachine({ ...m, transitions: [{ ...m.transitions[0]!, mark: '-' }, m.transitions[1]!] })).toEqual([
      'state "idle" has no way out and is not final',
      'state "done" is unreachable from "idle"',
    ]);
  });

  test('requires the grid to place every state exactly once', () => {
    expect(validateMachine({ ...base, grid: [['idle', 'busy', 'idle'], ['ghost', null]] })).toEqual([
      'grid: state "idle" appears twice',
      'grid: unknown state "ghost"',
      'grid: state "done" has no cell',
    ]);
  });
});

describe('layoutMachine', () => {
  const measure = (s: string) => s.length * 7;

  test('places states on their grid cells', () => {
    const lay = layoutMachine(machineExample as MachineSpec, measure);
    const { scheduled, sending, failed, cancelled } = lay.boxes;
    expect(scheduled!.y).toBe(sending!.y);
    expect(cancelled!.x).toBe(scheduled!.x);
    expect(failed!.x).toBe(sending!.x);
    expect(failed!.y).toBeGreaterThan(sending!.y);
  });

  test('offsets a two-way pair so the arrows do not overlap', () => {
    const lay = layoutMachine(machineExample as MachineSpec, measure);
    const fail = lay.routes[2]!.pts;
    const retry = lay.routes[5]!.pts;
    expect(fail[0]![0]).not.toBe(retry[0]![0]);
    expect(Math.abs(fail[0]![0] - retry[retry.length - 1]![0])).toBe(14);
  });

  test('layers states by distance from the initial state without a grid', () => {
    const lay = layoutMachine(base, measure);
    expect(lay.boxes.idle!.x).toBeLessThan(lay.boxes.busy!.x);
    expect(lay.boxes.busy!.x).toBeLessThan(lay.boxes.done!.x);
    expect(lay.routes[0]!.pts).toHaveLength(2);
  });

  test('routes a same-row edge around a state in the way', () => {
    const m: MachineSpec = {
      ...base,
      grid: [['idle', 'busy', 'done']],
      transitions: [...base.transitions, { from: 'idle', event: 'skip', to: 'done' }],
    };
    const lay = layoutMachine(m, measure);
    const skip = lay.routes[2]!;
    expect(skip.pts).toHaveLength(4);
    expect(skip.pts[1]![1]).toBeLessThan(lay.boxes.idle!.y);
  });

  test('draws an arrow head at the end point', () => {
    expect(arrowHead([0, 0], [10, 0], 4)).toBe('M10,0L6,2L6,-2Z');
  });
});

describe('mockDocument', () => {
  const html = '<p data-ref="x">hi</p><meta http-equiv="refresh" content="0;url=https://example.com"><script>alert(1)</script>';
  const doc = mockDocument({ html, css: 'p{color:red}</style><meta http-equiv="refresh">' }, 'N0NCE', 'T0KEN');
  const host = doc.slice(doc.indexOf('<script nonce='), doc.lastIndexOf('</script>'));

  test('pins a nonce CSP and ships author html and css only as escaped data', () => {
    expect(mockCsp('N0NCE')).toBe(
      "default-src 'none'; style-src 'unsafe-inline'; img-src data:; script-src 'nonce-N0NCE'; base-uri 'none'; form-action 'none'",
    );
    expect(doc).not.toContain('<meta http-equiv="refresh"');
    expect(doc).not.toContain('<p data-ref');
    expect(doc).not.toContain('</style><meta');
    expect(host.slice(host.indexOf('>') + 1)).not.toContain('<');
    expect(host).toContain('\\u003cp data-ref');
  });

  test('only the host script carries the nonce, and its measurements carry the token', () => {
    expect(doc.match(/nonce="N0NCE"/g)).toHaveLength(1);
    expect(host).toContain('"token":"T0KEN"');
    expect(host).toContain('ccPresentMock:p.token');
    expect(host).toContain('(d)');
  });

  test('adds terminal helpers only for the terminal frame', () => {
    expect(doc).not.toContain('.dim{');
    expect(mockDocument({ html: 'x', frame: 'terminal' }, 'n', 'k')).toContain('.dim{');
  });
});

class FakeAttr {
  constructor(readonly localName: string, readonly value: string) {}
}

class FakeEl {
  parent: FakeEl | null = null;
  attributes: FakeAttr[];
  children: FakeEl[] = [];
  constructor(readonly tag: string, attrs: Record<string, string> = {}, children: FakeEl[] = []) {
    this.attributes = Object.entries(attrs).map(([k, v]) => new FakeAttr(k, v));
    for (const c of children) {
      c.parent = this;
      this.children.push(c);
    }
  }
  all(): FakeEl[] {
    return this.children.flatMap((c) => [c, ...c.all()]);
  }
  querySelectorAll(selector: string): FakeEl[] {
    const tags = selector.split(',');
    return this.all().filter((e) => tags.includes('*') || tags.includes(e.tag));
  }
  getAttribute(name: string): string | null {
    return this.attributes.find((a) => a.localName === name)?.value ?? null;
  }
  removeAttributeNode(attr: FakeAttr): void {
    this.attributes = this.attributes.filter((a) => a !== attr);
  }
  remove(): void {
    this.parent!.children = this.parent!.children.filter((c) => c !== this);
  }
  shape(): string {
    const attrs = this.attributes.map((a) => ` ${a.localName}`).join('');
    return `<${this.tag}${attrs}>${this.children.map((c) => c.shape()).join('')}`;
  }
}

describe('neutralize', () => {
  test('drops navigation-capable elements and attributes, keeping the rest', () => {
    const root = new FakeEl('body', {}, [
      new FakeEl('meta', { 'http-equiv': 'refresh' }),
      new FakeEl('base', { href: 'https://example.com' }),
      new FakeEl('iframe', { srcdoc: 'x' }),
      new FakeEl('a', { href: 'https://example.com', target: '_self', class: 'btn', 'data-ref': 'later' }, [new FakeEl('span')]),
      new FakeEl('svg', {}, [new FakeEl('a', { href: '#x' }, [new FakeEl('set', { attributeName: 'href', to: 'https://example.com' })])]),
      new FakeEl('form', { action: 'https://example.com' }, [new FakeEl('button', { formaction: 'https://example.com' })]),
      new FakeEl('set', { attributeName: 'fill' }),
    ]);
    neutralize(root as unknown as ParentNode);
    expect(root.shape()).toBe('<body><a class data-ref><span><svg><a><form><button><set attributeName>');
  });
});

describe('pins', () => {
  test('places a pin on the scaled top-right corner and skips unknown or clipped refs', () => {
    const rects = { a: [10, 20, 100, 30] as [number, number, number, number], b: [0, 900, 10, 10] as [number, number, number, number] };
    expect(pinSpots(['a', 'gone', 'b'], rects, 0.5, 300, 200)).toEqual([{ ref: 'a', n: 1, x: 55, y: 11 }]);
  });

  test('a blank comment removes the entry', () => {
    expect(setComment({ a: 'x' }, 'b', '  hi ')).toEqual({ a: 'x', b: 'hi' });
    expect(setComment({ a: 'x' }, 'a', '  ')).toEqual({});
  });
});
