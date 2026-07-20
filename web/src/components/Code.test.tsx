// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';

vi.mock('../highlight', () => ({
  resolveLang: (lang: string) => (lang === 'go' ? 'go' : null),
  highlight: (code: string) => Promise.resolve(`<pre class="shiki"><code>${code}</code></pre>`),
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
