// @vitest-environment jsdom
import { beforeAll, describe, expect, it, vi } from 'vitest';

const initialize = vi.fn();
const render = vi.fn().mockResolvedValue({ svg: '<svg></svg>' });
vi.mock('mermaid', () => ({ default: { initialize, render } }));

import { renderDiagram } from './mermaid';

// configure runs once via the lazy singleton; capture the config mermaid.initialize saw.
describe('mermaid configure', () => {
  let cfg: { htmlLabels?: unknown; flowchart?: { htmlLabels?: unknown }; themeVariables: Record<string, string> };

  beforeAll(async () => {
    await renderDiagram('graph LR\n  A[Client] --> B[Edge]');
    expect(initialize).toHaveBeenCalledTimes(1);
    cfg = initialize.mock.calls[0]![0];
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
