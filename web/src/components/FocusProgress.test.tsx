// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { FocusProgress } from './FocusProgress';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import type { Interactions, Verdict } from '../events';
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
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
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
