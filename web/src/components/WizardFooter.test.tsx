// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { WizardFooter } from './WizardFooter';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import type { Approval } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const approval = (id: string, prompt: string): Approval => ({ id, type: 'approval', prompt });
const steps = focusSteps([approval('a1', 'One'), approval('a2', 'Two')], new Set());
const interactions = emptyState().interactions;
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

function render(index: number, advancing = false, onPrev = vi.fn(), onNext = vi.fn(), onJump = vi.fn()) {
  act(() =>
    root.render(
      <WizardFooter
        steps={steps}
        index={index}
        total={steps.length}
        advancing={advancing}
        interactions={interactions}
        packInteractive={packInteractive}
        onJump={onJump}
        onPrev={onPrev}
        onNext={onNext}
      />,
    ),
  );
  return { onPrev, onNext, onJump };
}

describe('WizardFooter', () => {
  it('renders Back, centered step progress, and Next', () => {
    render(0);
    const buttons = container.querySelectorAll('.wizard-bar > .btn');
    expect((buttons[0] as HTMLButtonElement).disabled).toBe(true);
    expect(buttons[0]!.textContent).toBe('‹ Back');
    const dots = container.querySelectorAll('.focus-dot');
    expect(dots.length).toBe(2);
    expect(container.querySelector('.focus-dot.current')).toBe(dots[0]);
    expect(buttons[1]!.textContent).toBe('Next ›');
  });

  it('renders the review label and auto-advance cue on the final step', () => {
    const { onPrev, onNext } = render(1, true);
    const back = container.querySelector('.btn-ghost') as HTMLButtonElement;
    const next = container.querySelector('.btn-primary') as HTMLButtonElement;
    expect(next.classList).toContain('advancing');
    expect(next.textContent).toContain('Review');
    expect(next.querySelector('.focus-advance-fill')).not.toBeNull();
    expect(next.querySelector('.focus-advance-text')?.textContent).toBe('next in a moment');
    act(() => back.click());
    act(() => next.click());
    expect(onPrev).toHaveBeenCalledOnce();
    expect(onNext).toHaveBeenCalledOnce();
  });

  it('disables Next on the summary, past the last current dot', () => {
    render(steps.length);
    const next = container.querySelector('.btn-primary') as HTMLButtonElement;
    expect(next.disabled).toBe(true);
    expect(container.querySelector('.focus-dot.current')).toBeNull();
  });
});
