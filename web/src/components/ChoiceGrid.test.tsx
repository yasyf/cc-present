import { describe, expect, it } from 'vitest';
import type { OptionVisual } from '../schema';
import { visualTitle } from './ChoiceGrid';

describe('visualTitle', () => {
  it('labels each visual type by authored title, else the iOS fallback name', () => {
    const cases: [OptionVisual, string][] = [
      [{ id: 'c', type: 'code', lang: 'go', code: '' }, 'go'],
      [{ id: 'c', type: 'code', lang: 'go', code: '', title: 'main.go' }, 'main.go'],
      [{ id: 'd', type: 'diagram', kind: 'mermaid', source: '' }, 'Diagram'],
      [{ id: 'i', type: 'image', src: 'x', alt: 'A chart' }, 'A chart'],
      [{ id: 'i', type: 'image', src: 'x', alt: 'A chart', caption: 'Q3' }, 'Q3'],
      [{ id: 'c', type: 'chart', kind: 'bar', categories: [], series: [] } as OptionVisual, 'Chart'],
      [{ id: 'c', type: 'chart', kind: 'bar', categories: [], series: [], title: 'Latency' } as OptionVisual, 'Latency'],
      [{ id: 't', type: 'term', output: 'x' } as OptionVisual, 'Terminal'],
      [{ id: 'f', type: 'filetree', entries: [] } as OptionVisual, 'Files'],
      [{ id: 'r', type: 'record', facts: [], title: 'AA123' } as OptionVisual, 'AA123'],
      [{ id: 'r', type: 'record', facts: [] } as OptionVisual, 'Record'],
    ];
    for (const [visual, title] of cases) expect(visualTitle(visual)).toBe(title);
  });
});
