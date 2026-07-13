// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { CopyButton } from './CopyButton';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;
let writeText: ReturnType<typeof vi.fn>;

beforeEach(() => {
  writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.useRealTimers();
});

async function click() {
  await act(async () => {
    container.querySelector('button')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  await act(async () => {
    await Promise.resolve();
  });
}

describe('CopyButton', () => {
  it('writes the text and swaps the glyph to a drawn check', async () => {
    await act(async () => root.render(<CopyButton text="hello world" />));
    expect(container.querySelector('.copy-glyph')).not.toBeNull();
    expect(container.querySelector('.mark-check')).toBeNull();

    await click();

    expect(writeText).toHaveBeenCalledWith('hello world');
    expect(container.querySelector('.mark-check')).not.toBeNull();
    expect(container.querySelector('.copy-glyph')).toBeNull();
  });

  it('reverts to the copy glyph after the hold', async () => {
    vi.useFakeTimers();
    await act(async () => root.render(<CopyButton text="x" />));
    await act(async () => {
      container.querySelector('button')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await act(async () => {
      await Promise.resolve();
    });
    expect(container.querySelector('.mark-check')).not.toBeNull();
    await act(async () => {
      vi.advanceTimersByTime(1300);
    });
    expect(container.querySelector('.copy-glyph')).not.toBeNull();
  });

  it('exposes an accessible label', async () => {
    await act(async () => root.render(<CopyButton text="x" />));
    expect(container.querySelector('button')?.getAttribute('aria-label')).toBe('Copy code');
  });

  it('renders nothing when the Clipboard API is unavailable', async () => {
    Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });
    await act(async () => root.render(<CopyButton text="x" />));
    expect(container.querySelector('button')).toBeNull();
  });

  it('draws a cross when the write is denied', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'));
    await act(async () => root.render(<CopyButton text="x" />));
    await click();
    expect(writeText).toHaveBeenCalledWith('x');
    expect(container.querySelector('.mark-cross')).not.toBeNull();
    expect(container.querySelector('.mark-check')).toBeNull();
    expect(container.querySelector('.copy-glyph')).toBeNull();
  });
});
