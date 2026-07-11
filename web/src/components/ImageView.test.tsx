// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { ImageView } from './ImageView';
import { resetTokenForTest } from '../token';
import type { Image as ImageBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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
