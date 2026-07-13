// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { Lightbox } from './Lightbox';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// jsdom implements no native <dialog>; emulate the contract the component leans
// on, as other suites stub ResizeObserver.
let restoreFocus: Element | null = null;
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  restoreFocus = document.activeElement;
  this.setAttribute('open', '');
  this.setAttribute('tabindex', '-1');
  this.focus();
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  if (!this.open) return;
  this.removeAttribute('open');
  this.dispatchEvent(new Event('close'));
  if (restoreFocus instanceof HTMLElement) restoreFocus.focus();
};
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') (document.querySelector('dialog[open]') as HTMLDialogElement | null)?.close();
});

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function render(open: boolean, onClose: () => void, caption?: string): void {
  act(() =>
    root.render(<Lightbox open={open} onClose={onClose} src="/img.png" alt="pic" caption={caption} />),
  );
}

const dialog = (): HTMLDialogElement => container.querySelector('dialog') as HTMLDialogElement;

describe('Lightbox', () => {
  it('shows the modal when the open prop is set and renders the caption', () => {
    render(true, () => {}, 'a diagram');
    expect(dialog().open).toBe(true);
    expect(container.querySelector('.lightbox-caption')?.textContent).toBe('a diagram');
  });

  it('closes and fires onClose when the open prop flips false', () => {
    const onClose = vi.fn();
    render(true, onClose);
    render(false, onClose);
    expect(dialog().open).toBe(false);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes on Escape', () => {
    const onClose = vi.fn();
    render(true, onClose);
    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })));
    expect(dialog().open).toBe(false);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes on a backdrop click', () => {
    const onClose = vi.fn();
    render(true, onClose);
    act(() => dialog().dispatchEvent(new MouseEvent('click', { bubbles: true })));
    expect(dialog().open).toBe(false);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('stays open when the image itself is clicked', () => {
    const onClose = vi.fn();
    render(true, onClose);
    const img = container.querySelector('.lightbox-img') as HTMLElement;
    act(() => img.dispatchEvent(new MouseEvent('click', { bubbles: true })));
    expect(dialog().open).toBe(true);
    expect(onClose).not.toHaveBeenCalled();
  });
});
