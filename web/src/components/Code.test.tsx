// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';

vi.mock('../highlight', () => ({
  resolveLang: (lang: string) => (lang === 'go' ? 'go' : null),
  highlight: (code: string) => Promise.resolve(`<pre class="shiki"><code>${code}</code></pre>`),
  tokenizeLines: (code: string) =>
    Promise.resolve(code.split('\n').map((line) => [{ content: line, htmlStyle: { color: '#123456' } }])),
}));

import { Code } from './Code';
import type { Code as CodeBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

const codeBlock = (lang: string, title?: string): CodeBlock => ({
  id: 'c',
  type: 'code',
  lang,
  code: 'package main',
  title,
});

async function renderCode(block: CodeBlock) {
  await act(async () => {
    root.render(<Code block={block} />);
  });
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('Code header', () => {
  it('names the plain-text fallback in the hold tone for an uncurated language', async () => {
    await renderCode(codeBlock('brainfuck'));
    const tag = container.querySelector('.code-lang');
    expect(tag?.classList.contains('code-lang-plain')).toBe(true);
    expect(tag?.textContent).toContain('brainfuck');
    expect(tag?.textContent).toContain('plain text');
    expect(container.querySelector('.shiki-wrap')).toBeNull();
  });

  it('shows just the language and highlights a curated language', async () => {
    await renderCode(codeBlock('go'));
    const tag = container.querySelector('.code-lang');
    expect(tag?.classList.contains('code-lang-plain')).toBe(false);
    expect(tag?.textContent).toBe('go');
    expect(container.querySelector('.shiki-wrap')).not.toBeNull();
  });

  it('renders a copy button in the header strip', async () => {
    await renderCode(codeBlock('go', 'main.go'));
    expect(container.querySelector('.code-title')?.textContent).toBe('main.go');
    expect(container.querySelector('button.copy-button')).not.toBeNull();
  });

  it('shows the current code after a prop change, never the prior highlight', async () => {
    await renderCode({ id: 'c', type: 'code', lang: 'go', code: 'AAA' });
    expect(container.querySelector('.shiki-wrap')?.textContent).toContain('AAA');
    await renderCode({ id: 'c', type: 'code', lang: 'go', code: 'BBB' });
    expect(container.textContent).toContain('BBB');
    expect(container.textContent).not.toContain('AAA');
  });
});

describe('Code grounded in a file', () => {
  const grounded: CodeBlock = {
    id: 'g',
    type: 'code',
    lang: 'go',
    code: 'func main() {\n\tprintln("hi")\n}',
    src: 'cmd/main.go',
    lines: '3-5',
    start: 3,
    highlight: '4-5',
    sha: 'abc1234+wt',
    pins: [
      { line: 4, title: 'prints', body: 'to stderr', tone: 'warn' },
      { line: 4, title: 'again' },
    ],
  };

  it('heads the block with path:lines @ sha', async () => {
    await renderCode(grounded);
    expect(container.querySelector('.code-src')?.textContent).toBe('cmd/main.go:3-5 @ abc1234+wt');
  });

  it('numbers the gutter from start and tints highlighted lines', async () => {
    await renderCode(grounded);
    const rows = [...container.querySelectorAll('.code-row')];
    expect(rows.map((r) => r.querySelector('.code-gutter')?.textContent)).toEqual(['3', '4', '5']);
    expect(rows.map((r) => r.classList.contains('code-lit'))).toEqual([false, true, true]);
    expect(rows[1]?.querySelector('.code-tok')?.textContent).toBe('\tprintln("hi")');
    expect(container.querySelector('.shiki-wrap')).toBeNull();
  });

  it('marks pinned lines and lists numbered pins', async () => {
    await renderCode(grounded);
    const rows = [...container.querySelectorAll('.code-row')];
    const marks = rows[1]?.querySelectorAll('.code-marks .code-pin-mark') ?? [];
    expect([...marks].map((m) => m.textContent)).toEqual(['1', '2']);
    expect(rows[0]?.querySelector('.code-marks .code-pin-mark')).toBeNull();
    const notes = [...container.querySelectorAll('.code-pins .code-pin')];
    expect(notes).toHaveLength(2);
    expect(notes[0]?.classList.contains('code-pin-warn')).toBe(true);
    expect(notes[0]?.textContent).toBe('1L4printsto stderr');
    expect(notes[1]?.classList.contains('code-pin-default')).toBe(true);
  });

  it('renders plain rows for an uncurated language', async () => {
    await renderCode({ ...grounded, lang: 'text', pins: undefined, highlight: undefined });
    expect(container.querySelector('.code-lang')?.textContent).toBe('text · plain text');
    expect(container.querySelector('.code-tok')).toBeNull();
    expect(container.querySelectorAll('.code-row')[2]?.textContent).toBe('5}');
  });
});
