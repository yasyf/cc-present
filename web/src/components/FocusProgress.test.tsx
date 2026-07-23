// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { FocusProgress } from './FocusProgress';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import { revisionStore } from '../revision';
import type { PresentState, Revising, WireFrame } from '../events';
import type { Approval, Block } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const approval = (id: string, prompt: string): Approval => ({ id, type: 'approval', prompt });
const packInteractive = new Set<string>();

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  revisionStore.reset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  revisionStore.reset();
});

function render(blocks: Block[], index = 0): void {
  const steps = focusSteps(blocks, packInteractive);
  act(() => {
    root.render(<FocusProgress steps={steps} index={index} />);
  });
}

function docState(ids: string[], revising: Revising = { blockIds: [] }): PresentState {
  const base = emptyState();
  return { ...base, doc: { ...base.doc, blocks: ids.map((id) => ({ id, type: 'markdown', md: id }) as Block) }, revising };
}
const revisingFrame = (ids: string[], note?: string): WireFrame => ({
  schemaVersion: 1,
  type: 'revising.changed',
  blockIds: ids,
  ...(note !== undefined ? { note } : {}),
});

describe('FocusProgress header', () => {
  it('renders the step counter in the header', () => {
    render([approval('a1', 'One'), approval('a2', 'Two')]);
    expect(container.querySelector('.focus-step-count')?.textContent).toBe('Step 1 / 2');
  });

  it('renders the doc-level drafting one-liner in the header', () => {
    revisionStore.ingest(revisingFrame([], 'drafting a comparison step'), docState(['a1']), {
      blockIds: [],
      note: 'drafting a comparison step',
    });
    render([approval('a1', 'One')]);
    expect(container.querySelector('.focus-drafting')?.textContent).toBe('Claude is drafting — drafting a comparison step');
  });

  it('announces deck growth on a polite live region', () => {
    render([approval('a1', 'One')]);
    render([approval('a1', 'One'), approval('a2', 'Two')]);
    expect(container.querySelector('.focus-deck-live')?.textContent).toBe('Deck grew to 2 steps');
  });
});
