// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from './present';
import type { PresentApi } from './present';
import { KeyboardProvider } from './keyboard';
import { Lightbox } from './components/Lightbox';
import { emptyState } from './reduce';
import type { Interactions } from './events';
import type { Approval, Block } from './schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
Element.prototype.scrollIntoView = () => {};
window.matchMedia = ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener() {},
  removeEventListener() {},
  addListener() {},
  removeListener() {},
  dispatchEvent: () => false,
})) as typeof window.matchMedia;

// jsdom implements no native <dialog>; emulate the open/close contract plus the
// native Escape-closes-modal behaviour the gate leans on.
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.setAttribute('open', '');
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  if (!this.open) return;
  this.removeAttribute('open');
  this.dispatchEvent(new Event('close'));
};
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') (document.querySelector('dialog[open]') as HTMLDialogElement | null)?.close();
});

const empty = (): Interactions => emptyState().interactions;
const approval = (id: string, prompt: string): Approval => ({ id, type: 'approval', prompt });
const blocks: Block[] = [approval('a1', 'Ship one')];

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

function render(onViewToggle: () => void, lightboxOpen: boolean, onClose: () => void): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider
          blocks={blocks}
          interactions={empty()}
          closed={false}
          round={1}
          onViewToggle={onViewToggle}
        >
          <Lightbox open={lightboxOpen} onClose={onClose} src="/x.png" alt="pic" />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

function press(key: string): void {
  act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })));
}

describe('keyboard global-shortcut gate behind a modal dialog', () => {
  it('fires a global shortcut when no dialog is open', () => {
    const toggle = vi.fn();
    render(toggle, false, () => {});
    press('v');
    expect(toggle).toHaveBeenCalledTimes(1);
  });

  it('suppresses v / j / ? behind an open lightbox and never stacks the help dialog', () => {
    const toggle = vi.fn();
    render(toggle, true, () => {});
    expect(document.querySelectorAll('dialog[open]').length).toBe(1);
    press('v');
    press('j');
    press('?');
    expect(toggle).not.toHaveBeenCalled();
    expect(document.querySelectorAll('dialog[open]').length).toBe(1);
  });

  it('lets Escape through to close the dialog, then restores shortcuts', () => {
    const toggle = vi.fn();
    const onClose = vi.fn();
    render(toggle, true, onClose);
    press('Escape');
    expect(onClose).toHaveBeenCalledTimes(1);
    render(toggle, false, onClose);
    press('v');
    expect(toggle).toHaveBeenCalledTimes(1);
  });
});
