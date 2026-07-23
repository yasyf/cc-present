// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext, usePresent } from './present';
import type { PresentApi } from './present';
import { KeyboardProvider, useKeyboardApi } from './keyboard';
import { ActiveBlockProvider, useActiveBlock } from './activeBlock';
import { emptyState } from './reduce';
import type { Approval } from './schema';

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
Element.prototype.scrollIntoView = () => {};

const blocks: Approval[] = [
  { id: 'a1', type: 'approval' },
  { id: 'a2', type: 'approval' },
  { id: 'a3', type: 'approval' },
];

function Probe() {
  const { post } = usePresent();
  const kbd = useKeyboardApi();
  const { activeId, panelOpen, pinnedOpen, pin, closePanel, requestCompose, setComposing } = useActiveBlock();
  return (
    <div>
      <div data-testid="active">{activeId ?? 'none'}</div>
      <div data-testid="panel-open">{String(panelOpen)}</div>
      <div data-testid="pinned-open">{String(pinnedOpen)}</div>
      <button data-testid="post-a1" onClick={() => void post({ type: 'decision.created', blockId: 'a1', verdict: 'approved' })} />
      <button data-testid="cursor-a2" onClick={() => kbd.setCursor('a2')} />
      <button data-testid="cursor-a3" onClick={() => kbd.setCursor('a3')} />
      <button data-testid="pin-a1" onClick={() => pin('a1')} />
      <button data-testid="request-compose" onClick={requestCompose} />
      <button data-testid="close-panel" onClick={closePanel} />
      <button data-testid="compose-on" onClick={() => setComposing(true)} />
      <button data-testid="compose-off" onClick={() => setComposing(false)} />
    </div>
  );
}

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

function render(): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={blocks} interactions={emptyState().interactions} closed={false} round={1}>
          <ActiveBlockProvider>
            <Probe />
          </ActiveBlockProvider>
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

const active = (): string | null => container.querySelector('[data-testid="active"]')!.textContent;
const click = (id: string): void => act(() => (container.querySelector(`[data-testid="${id}"]`) as HTMLButtonElement).click());

describe('ActiveBlockProvider resolution', () => {
  it('starts with nothing active', () => {
    render();
    expect(active()).toBe('none');
  });

  it('records the last interacted block through the wrapped post', () => {
    render();
    click('post-a1');
    expect(active()).toBe('a1');
  });

  it('prefers the cursor over the last interaction', () => {
    render();
    click('post-a1');
    click('cursor-a2');
    expect(active()).toBe('a2');
  });

  it('prefers an explicit pin over the cursor, then yields on a cursor move', () => {
    render();
    click('cursor-a2');
    click('pin-a1');
    expect(active()).toBe('a1');
    // Moving the cursor supersedes the pin so the rail follows navigation.
    click('cursor-a3');
    expect(active()).toBe('a3');
  });
});

describe('ActiveBlockProvider composing latch', () => {
  it('freezes the active id while the composer holds a draft', () => {
    render();
    click('cursor-a2');
    expect(active()).toBe('a2');
    click('compose-on');
    // A background cursor move must not swap the pinned thread mid-type.
    click('cursor-a3');
    expect(active()).toBe('a2');
    // Releasing the composer lets the active id catch up to the cursor.
    click('compose-off');
    expect(active()).toBe('a3');
  });
});

describe('ActiveBlockProvider panel state', () => {
  it('clears the desktop pin latch when closing the panel', () => {
    render();
    click('request-compose');
    expect(container.querySelector('[data-testid="panel-open"]')!.textContent).toBe('true');
    expect(container.querySelector('[data-testid="pinned-open"]')!.textContent).toBe('true');

    click('close-panel');
    expect(container.querySelector('[data-testid="panel-open"]')!.textContent).toBe('false');
    expect(container.querySelector('[data-testid="pinned-open"]')!.textContent).toBe('false');
  });
});
