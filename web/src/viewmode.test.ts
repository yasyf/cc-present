import { describe, expect, it } from 'vitest';
import { resolveMode, viewKey } from './viewmode';
import type { ViewMode } from './viewmode';
import type { FocusStep } from './focus';

const step = (kind: FocusStep['kind']): FocusStep => ({
  id: kind,
  kind,
  context: [],
  block: { id: kind, type: 'markdown', md: '' },
  decidables: [],
  swipeable: false,
});

const decision = [step('decision')];
const contentOnly = [step('context')];

describe('resolveMode', () => {
  const cases: {
    name: string;
    presentation: ViewMode | undefined;
    override: ViewMode | null;
    steps: FocusStep[];
    expected: ViewMode;
  }[] = [
    { name: 'override wins over the hint and the default', presentation: 'focus', override: 'board', steps: decision, expected: 'board' },
    { name: 'the doc hint wins over the default', presentation: 'board', override: null, steps: decision, expected: 'board' },
    { name: 'defaults to focus when a step decides', presentation: undefined, override: null, steps: decision, expected: 'focus' },
    { name: 'defaults to board for a content-only doc', presentation: undefined, override: null, steps: contentOnly, expected: 'board' },
    { name: 'defaults to board for an empty doc', presentation: undefined, override: null, steps: [], expected: 'board' },
    { name: 'override focus beats a board hint', presentation: 'board', override: 'focus', steps: contentOnly, expected: 'focus' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(resolveMode(c.presentation, c.override, c.steps)).toBe(c.expected);
    });
  }
});

describe('viewKey', () => {
  it('namespaces the subject ref', () => {
    expect(viewKey('demo-board')).toBe('cc-present:view:demo-board');
  });
});
