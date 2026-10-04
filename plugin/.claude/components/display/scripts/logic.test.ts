import { describe, expect, test } from 'bun:test';
import { artifactDocument, panelProblems, splitMermaid, stepCount, wrapLabel } from '../src/logic';

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
