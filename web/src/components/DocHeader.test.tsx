// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { emptyState } from '../reduce';
import { DocHeader, wireState } from './DocHeader';
import type { DocHeaderProps } from './DocHeader';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
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
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.setAttribute('open', '');
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  if (!this.open) return;
  this.removeAttribute('open');
  this.dispatchEvent(new Event('close'));
};

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

const title = 'Durable agents: replan after the Oct 20 incident, round one of two';

function render(over: Partial<DocHeaderProps> = {}): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[]} interactions={emptyState().interactions} closed={false} round={1}>
          <DocHeader
            doc={{ version: 1, title, blocks: [] }}
            round={1}
            connected
            peerPresent
            closed={false}
            mode="board"
            onSetView={() => {}}
            {...over}
          />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

const menu = () => container.querySelector('.header-menu') as HTMLDialogElement;
const items = () => [...menu().querySelectorAll('.header-menu-item')].map((i) => i.textContent);

describe('wireState', () => {
  const cases: { name: string; args: [boolean, boolean, boolean | null]; expected: ReturnType<typeof wireState> }[] = [
    { name: 'a closed board reads closed whatever the wire says', args: [true, false, true], expected: 'closed' },
    { name: 'a dropped wire reads down', args: [false, false, true], expected: 'down' },
    { name: 'a present agent on a live wire', args: [false, true, true], expected: 'present' },
    { name: 'an absent agent on a live wire', args: [false, true, false], expected: 'away' },
    { name: 'unknown presence on a live wire', args: [false, true, null], expected: 'live' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(wireState(...c.args)).toBe(c.expected);
    });
  }
});

describe('DocHeader', () => {
  it('keeps one status dot by the full title and nothing but the toggle and menu beside it', () => {
    render();
    expect(container.querySelector('.doc-title')!.getAttribute('title')).toBe(title);
    const dot = container.querySelector('.doc-heading [role="status"]')!;
    expect(dot.getAttribute('aria-label')).toBe('Agent on the line · live');
    const controls = container.querySelector('.doc-controls')!;
    expect([...controls.children].map((c) => c.className)).toEqual(['view-toggle', 'header-menu-toggle', 'header-menu']);
  });

  it('tucks shortcuts and theme behind the menu, adding notes only below the rail breakpoint', () => {
    render();
    expect(menu().hasAttribute('open')).toBe(false);
    act(() => (container.querySelector('.header-menu-toggle') as HTMLButtonElement).click());
    expect(menu().hasAttribute('open')).toBe(true);
    expect(items()).toEqual(['Shortcuts ?', 'Theme']);
    expect(menu().querySelectorAll('.theme-seg').length).toBe(3);

    const onOpenComments = vi.fn();
    render({ commentCount: 4, onOpenComments });
    expect(items()).toEqual(['Notes4', 'Shortcuts ?', 'Theme']);
    act(() => (menu().querySelector('.header-menu-item') as HTMLButtonElement).click());
    expect(onOpenComments).toHaveBeenCalledTimes(1);
    expect(menu().hasAttribute('open')).toBe(false);
  });
});
