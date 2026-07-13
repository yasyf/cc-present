// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { ImageView } from './ImageView';
import { resetTokenForTest } from '../token';
import type { Image as ImageBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// jsdom implements neither native <dialog> nor button keyboard activation;
// emulate both, as other suites stub ResizeObserver.
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
  const t = e.target as HTMLElement;
  if (e.key === 'Escape') (document.querySelector('dialog[open]') as HTMLDialogElement | null)?.close();
  else if ((e.key === 'Enter' || e.key === ' ') && t instanceof HTMLButtonElement) t.click();
});

const imageBlock = (src: string): ImageBlock => ({ id: 'img', type: 'image', src, alt: 'diagram' }) as ImageBlock;

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
  window.history.replaceState(null, '', '/');
  resetTokenForTest();
});

function srcOf(block: ImageBlock): string {
  act(() => root.render(<ImageView block={block} />));
  const img = container.querySelector('img');
  if (!img) throw new Error('no img rendered');
  return img.getAttribute('src') ?? '';
}

describe('ImageView asset resolution', () => {
  it('routes an asset: src through the page token when one is set', () => {
    window.history.replaceState(null, '', '/?token=deadbeef');
    resetTokenForTest();
    expect(srcOf(imageBlock('asset:abc123'))).toBe('/assets/abc123?token=deadbeef');
  });

  it('leaves the asset URL byte-identical when there is no token', () => {
    resetTokenForTest();
    expect(srcOf(imageBlock('asset:abc123'))).toBe('/assets/abc123');
  });

  it('passes https: and data: URIs through unchanged even with a token set', () => {
    window.history.replaceState(null, '', '/?token=deadbeef');
    resetTokenForTest();
    expect(srcOf(imageBlock('https://example.com/x.png'))).toBe('https://example.com/x.png');
    expect(srcOf(imageBlock('data:image/png;base64,AAAA'))).toBe('data:image/png;base64,AAAA');
  });
});

function renderView(block: ImageBlock): void {
  act(() => root.render(<ImageView block={block} />));
}

const trigger = (): HTMLButtonElement => container.querySelector('.image-trigger') as HTMLButtonElement;
const dialog = (): HTMLDialogElement => container.querySelector('dialog') as HTMLDialogElement;

describe('ImageView lightbox trigger', () => {
  it('wraps the lazy image in a dialog-popup button', () => {
    renderView(imageBlock('https://example.com/x.png'));
    expect(trigger().getAttribute('aria-haspopup')).toBe('dialog');
    expect(trigger().querySelector('img')?.getAttribute('loading')).toBe('lazy');
    expect(dialog().open).toBe(false);
  });

  it('opens the lightbox on click', () => {
    renderView(imageBlock('https://example.com/x.png'));
    act(() => trigger().click());
    expect(dialog().open).toBe(true);
  });

  it('opens the lightbox on Enter', () => {
    renderView(imageBlock('https://example.com/x.png'));
    act(() => trigger().focus());
    act(() => trigger().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(dialog().open).toBe(true);
  });

  it('restores focus to the trigger after the lightbox closes', () => {
    renderView(imageBlock('https://example.com/x.png'));
    act(() => trigger().focus());
    act(() => trigger().click());
    expect(document.activeElement).not.toBe(trigger());
    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })));
    expect(dialog().open).toBe(false);
    expect(document.activeElement).toBe(trigger());
  });
});
