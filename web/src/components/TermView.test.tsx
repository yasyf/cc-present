// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { TermView } from './TermView';
import type { Term } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const ESC = String.fromCharCode(27);
const GREEN = `${ESC}[32mgreen${ESC}[0m plain`;

const term = (output: string, extra?: Partial<Term>): Term => ({
  id: 't1',
  type: 'term',
  output,
  ...extra,
});

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

async function render(block: Term): Promise<void> {
  await act(async () => {
    root.render(<TermView block={block} />);
  });
}

async function waitFor(predicate: () => boolean): Promise<void> {
  for (let i = 0; i < 100; i++) {
    if (predicate()) return;
    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });
  }
  throw new Error('condition never met');
}

describe('TermView', () => {
  it('first paints the stripped output as plain text, no escape bytes', async () => {
    await render(term(GREEN));
    const plain = container.querySelector('.term-plain');
    expect(plain).not.toBeNull();
    expect(plain?.textContent).toBe('green plain');
    expect(plain?.textContent).not.toContain(ESC);
    expect(container.querySelector('.shiki-wrap')).toBeNull();
  });

  it('swaps in ANSI-colored HTML once real Shiki resolves', async () => {
    await render(term(GREEN));
    await waitFor(() => container.querySelector('.shiki-wrap') != null);
    const wrap = container.querySelector('.shiki-wrap');
    expect(wrap?.textContent).toContain('green');
    const green = Array.from(wrap?.querySelectorAll('span') ?? []).find(
      (s) => s.textContent === 'green',
    );
    expect(green).not.toBeUndefined();
    // The 32m SGR resolves to shiki's github-light green with a dual-theme dark var.
    const style = (green as HTMLElement).getAttribute('style') ?? '';
    expect(style).toMatch(/color:\s*#28a745/i);
    expect(style).toContain('--shiki-dark');
    expect(container.querySelector('.term-plain')).toBeNull();
  });

  it('copies the stripped output, not the raw ANSI stream', async () => {
    await render(term(GREEN, { command: 'make build' }));
    const button = container.querySelector('button.copy-button') as HTMLButtonElement;
    await act(async () => {
      button.click();
    });
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('green plain');
  });

  it('renders the command row with a dim prompt glyph and an optional title', async () => {
    await render(term('done', { command: 'npm test', title: 'Test run' }));
    expect(container.querySelector('.term-title')?.textContent).toBe('Test run');
    const command = container.querySelector('.term-command');
    expect(command?.textContent).toContain('npm test');
    expect(container.querySelector('.term-prompt')?.textContent).toBe('❯');
  });

  it('omits the command row when no command is set', async () => {
    await render(term('bare output'));
    expect(container.querySelector('.term-command')).toBeNull();
    expect(container.querySelector('.term-title')).toBeNull();
  });

  it('shows only the current output after a prop change, never the prior highlight', async () => {
    await render(term(GREEN));
    await waitFor(() => container.querySelector('.shiki-wrap') != null);
    expect(container.textContent).toContain('green');

    // The html is paired with the output it was computed from, so a change never leaves
    // the previous highlight under the new text — the stripped new output shows at once.
    await render(term(`${ESC}[31mred${ESC}[0m done`));
    await waitFor(() => container.textContent?.includes('red done') ?? false);
    expect(container.textContent).not.toContain('green');
  });
});
