// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { StepDots } from './StepDots';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import { revisionStore } from '../revision';
import type { Interactions, PresentState, Revising, Verdict, WireFrame } from '../events';
import type { Approval, Block } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const approval = (id: string, prompt: string): Approval => ({ id, type: 'approval', prompt });
const empty = (): Interactions => emptyState().interactions;
const withVerdict = (id: string, verdict: Verdict): Interactions => ({
  ...empty(),
  decisions: { [id]: { verdict } },
});
const packInteractive = new Set<string>();

function docState(ids: string[], revising: Revising = { blockIds: [] }): PresentState {
  const base = emptyState();
  return { ...base, doc: { ...base.doc, blocks: ids.map((id) => ({ id, type: 'markdown', md: id }) as Block) }, revising };
}
const upsertFrame = (id: string): WireFrame => ({ schemaVersion: 1, type: 'block.upserted', block: { id, type: 'markdown', md: id } });
const revisingFrame = (ids: string[]): WireFrame => ({ schemaVersion: 1, type: 'revising.changed', blockIds: ids });

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

function render(blocks: Block[], interactions = empty(), index = 0, onJump = vi.fn()) {
  const steps = focusSteps(blocks, packInteractive);
  act(() =>
    root.render(
      <StepDots
        steps={steps}
        index={index}
        interactions={interactions}
        packInteractive={packInteractive}
        onJump={onJump}
      />,
    ),
  );
  return { steps, onJump };
}

describe('StepDots', () => {
  it('preserves dot status, labels, current state, and jump behavior', () => {
    const onJump = vi.fn();
    render([approval('a1', 'Ship it'), approval('a2', 'Hold it')], withVerdict('a1', 'approved'), 0, onJump);
    const dots = container.querySelectorAll('.focus-dot');
    expect(dots.length).toBe(2);
    expect(dots[0]!.className).toBe('focus-dot current approved');
    expect(dots[0]!.getAttribute('aria-label')).toBe('Step 1: Ship it, approved');
    expect(dots[0]!.getAttribute('aria-current')).toBe('true');
    expect(dots[1]!.classList).toContain('undecided');
    act(() => (dots[1] as HTMLButtonElement).click());
    expect(onJump).toHaveBeenCalledWith('a2');
  });

  it('collapses more than ten steps to the segmented rail', () => {
    const blocks = Array.from({ length: 11 }, (_, i) => approval(`a${i}`, `Ship ${i}`));
    const { steps } = render(blocks);
    expect(steps.length).toBe(11);
    expect(container.querySelectorAll('.focus-dot').length).toBe(0);
    expect(container.querySelectorAll('.focus-strip-seg').length).toBe(11);
  });

  it('carries step state and the jump target through a segment past the rail cap', () => {
    const onJump = vi.fn();
    const blocks = Array.from({ length: 12 }, (_, i) => approval(`a${i}`, `Ship ${i}`));
    const { steps } = render(blocks, withVerdict('a0', 'approved'), 0, onJump);
    expect(steps.length).toBe(12);
    expect(container.querySelectorAll('.focus-dot').length).toBe(0);
    const segs = container.querySelectorAll('.focus-strip-seg');
    expect(segs.length).toBe(12);
    expect(segs[0]!.classList.contains('approved')).toBe(true);
    expect(segs[0]!.classList.contains('current')).toBe(true);
    act(() => (segs[3] as HTMLButtonElement).click());
    expect(onJump).toHaveBeenCalledWith('a3');
  });

  it('layers the dot states with priority revising > added > changed', () => {
    revisionStore.markLive();
    revisionStore.ingest(revisingFrame(['a1']), docState(['a1', 'a2', 'a3'], { blockIds: ['a1'] }), { blockIds: ['a1'] });
    revisionStore.ingest(upsertFrame('a2'), docState(['a1', 'a3'], { blockIds: ['a1'] }), { blockIds: ['a1'] });
    revisionStore.ingest(upsertFrame('a3'), docState(['a1', 'a2', 'a3'], { blockIds: ['a1'] }), { blockIds: ['a1'] });

    render([approval('a1', 'One'), approval('a2', 'Two'), approval('a3', 'Three')]);

    const dots = container.querySelectorAll('.focus-dot');
    expect(dots[0]!.className).toContain('revising');
    expect(dots[1]!.className).toContain('added');
    expect(dots[2]!.className).toContain('changed');
  });
});
