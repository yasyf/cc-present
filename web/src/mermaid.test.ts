// @vitest-environment jsdom
import { beforeAll, describe, expect, it, vi } from 'vitest';

const initialize = vi.fn();
const render = vi.fn().mockResolvedValue({ svg: '<svg></svg>' });
vi.mock('mermaid', () => ({ default: { initialize, render } }));
const fontLoad = vi.fn().mockResolvedValue([]);
Object.defineProperty(document, 'fonts', { configurable: true, value: { load: fontLoad, ready: Promise.resolve() } });

import { renderDiagram } from './mermaid';

// configure runs once via the lazy singleton; capture the config mermaid.initialize saw.
describe('mermaid configure', () => {
  let cfg: { htmlLabels?: unknown; flowchart?: { htmlLabels?: unknown }; themeVariables: Record<string, string> };

  beforeAll(async () => {
    await renderDiagram('graph LR\n  A[Client] --> B[Edge]');
    expect(initialize).toHaveBeenCalledTimes(1);
    cfg = initialize.mock.calls[0]![0];
  });

  it('loads the Barlow prose face before mermaid first measures a label', () => {
    expect(fontLoad).toHaveBeenCalledWith('400 1em Barlow');
    expect(fontLoad.mock.invocationCallOrder[0]!).toBeLessThan(initialize.mock.invocationCallOrder[0]!);
    expect(initialize.mock.invocationCallOrder[0]!).toBeLessThan(render.mock.invocationCallOrder[0]!);
  });

  it('disables HTML labels at the top level, not the ignored flowchart.htmlLabels', () => {
    // Top-level is the flag mermaid honors. The nested form emits <foreignObject> HTML
    // labels that DiagramView's SVG-profile sanitize strips, leaving empty node boxes.
    expect(cfg.htmlLabels).toBe(false);
    expect(cfg.flowchart?.htmlLabels).toBeUndefined();
  });

  it('resolves node text ink distinct from the surface and background fills', () => {
    const tv = cfg.themeVariables;
    expect(tv.primaryTextColor).not.toBe(tv.primaryColor);
    expect(tv.textColor).not.toBe(tv.background);
    expect(tv.textColor).not.toBe(tv.mainBkg);
  });
});
