// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { FocusProgress } from './FocusProgress';
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
  act(() => {
    root.render(
      <FocusProgress
        steps={steps}
        index={index}
        interactions={interactions}
        packInteractive={packInteractive}
        onJump={onJump}
      />,
    );
  });
  return { steps, onJump };
}

describe('FocusProgress dot rail', () => {
  it('renders trailing context as a muted tick alongside decision dots', () => {
    const blocks: Block[] = [
      { id: 'm1', type: 'markdown', md: 'Lead-in context' },
      approval('a1', 'Ship one'),
      approval('a2', 'Ship two'),
      { id: 'm2', type: 'markdown', md: 'Trailing context' },
    ];

    const { steps } = render(blocks);

    expect(steps.length).toBe(3);
    expect(container.querySelectorAll('.focus-dot').length).toBe(3);
    expect(container.querySelectorAll('.focus-dot.tick').length).toBe(1);
    expect(container.querySelector('.focus-step-count')?.textContent).toBe('Step 1 / 3');
  });

  it('renders an input-only step as a tick', () => {
    render([{ id: 'i1', type: 'input', label: 'Notes' }]);

    expect(container.querySelector('.focus-dot')?.classList).toContain('tick');
  });

  it('renders an approved approval as approved rather than a tick', () => {
    render([approval('a1', 'Ship it')], withVerdict('a1', 'approved'));

    const dot = container.querySelector('.focus-dot');
    expect(dot?.classList).toContain('approved');
    expect(dot?.classList).not.toContain('tick');
  });

  it('jumps to a tick step by id', () => {
    const onJump = vi.fn();
    render(
      [approval('a1', 'Ship it'), { id: 'm1', type: 'markdown', md: 'Trailing context' }],
      empty(),
      0,
      onJump,
    );

    act(() => (container.querySelector('.focus-dot.tick') as HTMLButtonElement).click());

    expect(onJump).toHaveBeenCalledWith('m1');
  });
});

function docState(ids: string[], revising: Revising = { blockIds: [] }): PresentState {
  const base = emptyState();
  return { ...base, doc: { ...base.doc, blocks: ids.map((id) => ({ id, type: 'markdown', md: id }) as Block) }, revising };
}
const upsertFrame = (id: string): WireFrame => ({ type: 'block.upserted', block: { id, type: 'markdown', md: id } });
const revisingFrame = (ids: string[], note?: string): WireFrame => ({
  type: 'revising.changed',
  blockIds: ids,
  ...(note !== undefined ? { note } : {}),
});

describe('FocusProgress revision states', () => {
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

  it('describes the revision state in the dot aria-label', () => {
    revisionStore.ingest(revisingFrame(['a1']), docState(['a1']), { blockIds: ['a1'] });
    render([approval('a1', 'One')]);
    expect(container.querySelector('.focus-dot')?.getAttribute('aria-label')).toContain('being revised');
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

describe('FocusProgress segmented strip', () => {
  it('collapses past the rail cap to tappable segments carrying step state', () => {
    const onJump = vi.fn();
    const blocks = Array.from({ length: 12 }, (_, i) => approval(`a${i}`, `Ship ${i}`));
    const { steps } = render(blocks, withVerdict('a0', 'approved'), 0, onJump);

    expect(steps.length).toBe(12);
    expect(container.querySelectorAll('.focus-dot').length).toBe(0);
    const segs = container.querySelectorAll('.focus-strip-seg');
    expect(segs.length).toBe(12);
    // The verdict badge rides the segment; it stays a jump button.
    expect(segs[0]!.classList.contains('approved')).toBe(true);
    expect(segs[0]!.classList.contains('current')).toBe(true);
    act(() => (segs[3] as HTMLButtonElement).click());
    expect(onJump).toHaveBeenCalledWith('a3');
  });
});
