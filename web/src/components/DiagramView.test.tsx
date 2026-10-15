// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import type { Diagram } from '../schema';

vi.mock('../mermaid', () => ({ renderDiagram: vi.fn() }));
import { renderDiagram } from '../mermaid';
import { DiagramView } from './DiagramView';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

// A controllable prefers-color-scheme stub: `useResolvedTheme` reads `.matches` and
// subscribes to `change`, and `flipSystemDark` drives an OS-level flip in system mode.
type MqlListener = (e: { matches: boolean }) => void;
let darkMatches = false;
const mqlListeners = new Set<MqlListener>();
window.matchMedia = ((query: string) => ({
  get matches() {
    return darkMatches;
  },
  media: query,
  onchange: null,
  addEventListener: (_: string, l: MqlListener) => mqlListeners.add(l),
  removeEventListener: (_: string, l: MqlListener) => mqlListeners.delete(l),
  addListener: (l: MqlListener) => mqlListeners.add(l),
  removeListener: (l: MqlListener) => mqlListeners.delete(l),
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

function flipSystemDark(next: boolean): void {
  darkMatches = next;
  for (const l of mqlListeners) l({ matches: next });
}

// jsdom lays nothing out and ships no ResizeObserver or native <dialog>: the frame
// reports `frameWidth`, `resize` replays the observer, and the dialog emulates
// showModal/close as the Lightbox suite does.
let frameWidth = 0;
Object.defineProperty(HTMLElement.prototype, 'clientWidth', {
  configurable: true,
  get() {
    return frameWidth;
  },
});
let resize: ResizeObserverCallback | null = null;
class ResizeObserverStub {
  constructor(callback: ResizeObserverCallback) {
    resize = callback;
  }

  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.setAttribute('open', '');
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  if (!this.open) return;
  this.removeAttribute('open');
  this.dispatchEvent(new Event('close'));
};

const mockRender = vi.mocked(renderDiagram);
const diagram = (source: string, title?: string): Diagram => ({ id: 'd1', type: 'diagram', kind: 'mermaid', source, title });

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  mockRender.mockReset();
  frameWidth = 0;
  resize = null;
  darkMatches = false;
  mqlListeners.clear();
  document.documentElement.removeAttribute('data-theme');
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  document.documentElement.removeAttribute('data-theme');
});

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('DiagramView', () => {
  it('injects the sanitized SVG on a successful render, dropping the source', async () => {
    mockRender.mockResolvedValue('<svg data-mmd="1" viewBox="0 0 200 100"><g></g></svg>');
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A --> B', 'Flow')} />);
    });
    await flush();
    expect(container.querySelector('.diagram-title')?.textContent).toBe('Flow');
    expect(container.querySelector('.diagram-svg svg')).not.toBeNull();
    expect(container.querySelector('.diagram-source')).toBeNull();
    expect(container.querySelector('.diagram-error')).toBeNull();
  });

  it('shows an error banner over the raw source when rendering throws', async () => {
    mockRender.mockRejectedValue(new Error('parse error'));
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A -->')} />);
    });
    await flush();
    expect(container.querySelector('.diagram-error')).not.toBeNull();
    expect(container.querySelector('.diagram-source')?.textContent).toContain('graph LR');
    expect(container.querySelector('.diagram-svg')).toBeNull();
  });

  it('preserves SVG <text> labels and the label-color <style> through the sanitize pass', async () => {
    mockRender.mockResolvedValue(
      '<svg id="mmd-1" viewBox="0 0 200 100"><style>.label text{fill:rgb(31, 36, 48);}</style>' +
        '<g class="node"><rect class="basic label-container"></rect>' +
        '<g class="label"><text class="nodeLabel"><tspan class="text-inner-tspan">Client</tspan></text></g>' +
        '</g></svg>',
    );
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A[Client]')} />);
    });
    await flush();
    const svg = container.querySelector('.diagram-svg');
    expect(svg?.querySelector('style')).not.toBeNull();
    expect(svg?.querySelector('.nodeLabel')?.textContent).toBe('Client');
    expect(svg?.querySelector('style')?.textContent).toContain('.label text');
  });

  it('strips <foreignObject> HTML labels — why node labels must render as SVG text', async () => {
    mockRender.mockResolvedValue(
      '<svg id="mmd-1" viewBox="0 0 200 100"><g class="node"><g class="label">' +
        '<foreignObject><div xmlns="http://www.w3.org/1999/xhtml"><span class="nodeLabel"><p>Client</p></span></div></foreignObject>' +
        '</g></g></svg>',
    );
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A[Client]')} />);
    });
    await flush();
    const svg = container.querySelector('.diagram-svg');
    expect(svg?.querySelector('foreignObject')).toBeNull();
    expect(svg?.textContent).not.toContain('Client');
  });

  it('shows a skeleton until the renderer resolves', async () => {
    let resolve: (svg: string) => void = () => {};
    mockRender.mockReturnValue(new Promise((r) => (resolve = r)));
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A --> B')} />);
    });
    expect(container.querySelector('.diagram-skeleton')).not.toBeNull();
    await act(async () => {
      resolve('<svg viewBox="0 0 200 100"></svg>');
      await Promise.resolve();
    });
    await flush();
    expect(container.querySelector('.diagram-skeleton')).toBeNull();
    expect(container.querySelector('.diagram-svg svg')).not.toBeNull();
  });

  it('re-renders a mounted diagram when the data-theme attribute flips', async () => {
    mockRender.mockResolvedValue('<svg data-mmd="1" viewBox="0 0 200 100"></svg>');
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A --> B')} />);
    });
    await flush();
    expect(mockRender).toHaveBeenCalledTimes(1);
    // A ThemeToggle click mutates data-theme; the observer re-runs the render effect
    // so the SVG's baked palette follows the new surface.
    await act(async () => {
      document.documentElement.dataset.theme = 'dark';
      await Promise.resolve();
    });
    await flush();
    expect(mockRender).toHaveBeenCalledTimes(2);
  });

  it('re-renders on an OS dark-mode flip while in system mode', async () => {
    mockRender.mockResolvedValue('<svg viewBox="0 0 200 100"></svg>');
    await act(async () => {
      root.render(<DiagramView block={diagram('graph TD\n  A')} />);
    });
    await flush();
    expect(mockRender).toHaveBeenCalledTimes(1);
    // No explicit data-theme, so the OS preference drives the palette.
    await act(async () => {
      flipSystemDark(true);
      await Promise.resolve();
    });
    await flush();
    expect(mockRender).toHaveBeenCalledTimes(2);
  });

  it('renders a narrow diagram at its natural width', async () => {
    frameWidth = 400;
    await renderSvg(300);
    expect(fitVar()).toBe('300px');
  });

  it('shrinks a slightly wide diagram to the frame', async () => {
    frameWidth = 400;
    await renderSvg(450);
    expect(fitVar()).toBe('400px');
  });

  it('holds a wide diagram at 0.8 scale so the frame scrolls', async () => {
    frameWidth = 400;
    await renderSvg(1000);
    expect(fitVar()).toBe('800px');
  });

  it('re-fits when the frame resizes', async () => {
    frameWidth = 400;
    await renderSvg(1000);
    frameWidth = 900;
    act(() => resize?.([], {} as ResizeObserver));
    expect(fitVar()).toBe('900px');
  });

  it('re-fits when a theme flip swaps in a new SVG', async () => {
    frameWidth = 400;
    mockRender.mockResolvedValueOnce(svgOf(1000)).mockResolvedValueOnce(svgOf(300));
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A --> B')} />);
    });
    await flush();
    expect(fitVar()).toBe('800px');
    await act(async () => {
      document.documentElement.dataset.theme = 'dark';
      await Promise.resolve();
    });
    await flush();
    expect(fitVar()).toBe('300px');
  });

  it('opens the SVG at natural size in a dialog from the Expand button', async () => {
    frameWidth = 400;
    await renderSvg(1000, 'Flow');
    const dialog = container.querySelector('dialog') as HTMLDialogElement;
    expect(dialog.open).toBe(false);
    expect(dialog.querySelector('svg')).toBeNull();
    const expand = container.querySelector('.diagram-expand') as HTMLButtonElement;
    expect(expand.getAttribute('aria-haspopup')).toBe('dialog');
    act(() => expand.click());
    expect(dialog.open).toBe(true);
    expect(dialog.querySelector('.diagram-expanded svg')?.getAttribute('viewBox')).toBe('0 0 1000 100');
    expect(dialog.querySelector('.lightbox-caption')?.textContent).toBe('Flow');
    expect(figure().style.getPropertyValue('--diagram-natural')).toBe('1000px');
    act(() => (container.querySelector('.lightbox-close') as HTMLButtonElement).click());
    expect(dialog.open).toBe(false);
    expect(dialog.querySelector('svg')).toBeNull();
  });
});

function svgOf(width: number): string {
  return `<svg viewBox="0 0 ${width} 100"><g></g></svg>`;
}

async function renderSvg(width: number, title?: string): Promise<void> {
  mockRender.mockResolvedValue(svgOf(width));
  await act(async () => {
    root.render(<DiagramView block={diagram('graph LR\n  A --> B', title)} />);
  });
  await flush();
}

const figure = (): HTMLElement => container.querySelector('.diagram-block') as HTMLElement;
const fitVar = (): string => figure().style.getPropertyValue('--diagram-fit');
