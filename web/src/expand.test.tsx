// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { ExpandAllProvider, useExpandAll } from './expand';
import { Clamped } from './components/Clamped';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;

// jsdom reports zero for both dimensions, so Clamped would never see overflow;
// force content taller than its clamp so the per-block toggle renders.
Object.defineProperty(HTMLElement.prototype, 'scrollHeight', { configurable: true, get: () => 1000 });
Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 100 });

function Harness() {
  const { toggle } = useExpandAll();
  return (
    <>
      <button type="button" className="global-toggle" onClick={() => toggle()}>
        toggle
      </button>
      <Clamped html="<p>alpha</p>" lines={2} />
      <Clamped html="<p>beta</p>" lines={2} />
    </>
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
  act(() =>
    root.render(
      <ExpandAllProvider>
        <Harness />
      </ExpandAllProvider>,
    ),
  );
}

const contents = (): HTMLElement[] => [...container.querySelectorAll<HTMLElement>('.clamped-content')];
const clamp = (el: HTMLElement): boolean => el.classList.contains('is-clamped');
const globalToggle = (): void => act(() => (container.querySelector('.global-toggle') as HTMLElement).click());

describe('ExpandAllProvider', () => {
  it('a global toggle expands then collapses every Clamped', () => {
    render();
    expect(contents().every(clamp)).toBe(true);
    globalToggle();
    expect(contents().every((c) => !clamp(c))).toBe(true);
    globalToggle();
    expect(contents().every(clamp)).toBe(true);
  });

  it('a per-block toggle after a global expand wins until the next epoch', () => {
    render();
    globalToggle();
    const firstToggle = container.querySelectorAll<HTMLElement>('.clamp-toggle')[0]!;
    act(() => firstToggle.click());
    expect(clamp(contents()[0]!)).toBe(true);
    expect(clamp(contents()[1]!)).toBe(false);
    globalToggle();
    expect(contents().every(clamp)).toBe(true);
  });
});

describe('Clamped resync on content change', () => {
  function Provided({ html }: { html: string }) {
    const { toggle } = useExpandAll();
    return (
      <>
        <button type="button" className="global-toggle" onClick={() => toggle()}>
          toggle
        </button>
        <Clamped html={html} lines={2} />
      </>
    );
  }

  it('keeps an expand-all block expanded across a content update', () => {
    const show = (html: string): void =>
      act(() =>
        root.render(
          <ExpandAllProvider>
            <Provided html={html} />
          </ExpandAllProvider>,
        ),
      );
    show('<p>alpha</p>');
    expect(clamp(contents()[0]!)).toBe(true);
    globalToggle();
    expect(clamp(contents()[0]!)).toBe(false);
    show('<p>alpha revised</p>');
    expect(clamp(contents()[0]!)).toBe(false);
  });

  it('re-clamps an expanded block on a content update with no provider', () => {
    const show = (html: string): void => act(() => root.render(<Clamped html={html} lines={2} />));
    show('<p>alpha</p>');
    act(() => (container.querySelector('.clamp-toggle') as HTMLElement).click());
    expect(clamp(contents()[0]!)).toBe(false);
    show('<p>alpha revised</p>');
    expect(clamp(contents()[0]!)).toBe(true);
  });
});
