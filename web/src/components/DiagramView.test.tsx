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

const mockRender = vi.mocked(renderDiagram);
const diagram = (source: string, title?: string): Diagram => ({ id: 'd1', type: 'diagram', kind: 'mermaid', source, title });

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  mockRender.mockReset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('DiagramView', () => {
  it('injects the sanitized SVG on a successful render, dropping the source', async () => {
    mockRender.mockResolvedValue('<svg data-mmd="1"><g></g></svg>');
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

  it('shows a skeleton until the renderer resolves', async () => {
    let resolve: (svg: string) => void = () => {};
    mockRender.mockReturnValue(new Promise((r) => (resolve = r)));
    await act(async () => {
      root.render(<DiagramView block={diagram('graph LR\n  A --> B')} />);
    });
    expect(container.querySelector('.diagram-skeleton')).not.toBeNull();
    await act(async () => {
      resolve('<svg></svg>');
      await Promise.resolve();
    });
    await flush();
    expect(container.querySelector('.diagram-skeleton')).toBeNull();
    expect(container.querySelector('.diagram-svg svg')).not.toBeNull();
  });
});
