// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { FocusDeck } from './FocusDeck';
import { SubmitBar } from './SubmitBar';
import { focusSteps } from '../focus';
import { loadView, saveView } from '../viewmode';
import { emptyState } from '../reduce';
import type { Interactions, Verdict } from '../events';
import type { Approval, Block, Doc } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;
window.matchMedia = ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener() {},
  removeEventListener() {},
  addListener() {},
  removeListener() {},
  dispatchEvent: () => false,
})) as typeof window.matchMedia;
Element.prototype.scrollIntoView = () => {};

class LocalStorageStub {
  private store = new Map<string, string>();
  getItem(key: string): string | null {
    return this.store.has(key) ? this.store.get(key)! : null;
  }
  setItem(key: string, value: string): void {
    this.store.set(key, String(value));
  }
  removeItem(key: string): void {
    this.store.delete(key);
  }
  clear(): void {
    this.store.clear();
  }
}
(globalThis as { localStorage?: unknown }).localStorage = new LocalStorageStub();

const AUTO_ADVANCE_MS = 450;

const approval = (id: string, prompt: string): Approval => ({ id, type: 'approval', prompt });
const empty = (): Interactions => emptyState().interactions;
const withVerdict = (id: string, verdict: Verdict): Interactions => ({
  ...empty(),
  decisions: { [id]: { verdict } },
});
const withVerdicts = (over: Record<string, Verdict>): Interactions => ({
  ...empty(),
  decisions: Object.fromEntries(Object.entries(over).map(([id, verdict]) => [id, { verdict }])),
});

const three = [approval('a1', 'Ship one'), approval('a2', 'Ship two'), approval('a3', 'Ship three')];

function Deck({
  blocks,
  interactions,
  round = 1,
  closed = false,
}: {
  blocks: Block[];
  interactions: Interactions;
  round?: number;
  closed?: boolean;
}) {
  const steps = focusSteps(blocks, new Set());
  const present: PresentApi = { post: async () => true, closed, currentRound: round };
  return (
    <PresentContext.Provider value={present}>
      <KeyboardProvider blocks={blocks} interactions={interactions} closed={closed} round={round}>
        <FocusDeck key={round} steps={steps} interactions={interactions} round={round} closed={closed} />
      </KeyboardProvider>
    </PresentContext.Provider>
  );
}

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
  vi.useRealTimers();
});

type DeckProps = Parameters<typeof Deck>[0];

function render(props: DeckProps): void {
  act(() => root.render(<Deck {...props} />));
}

const currentPrompt = (): string | undefined =>
  container.querySelector('.focus-card .approval-prompt')?.textContent ?? undefined;
const peekTitle = (): string | undefined => container.querySelector('.focus-peek-title')?.textContent ?? undefined;
function clickNext(): void {
  act(() => (container.querySelector('.focus-nav-btn.primary') as HTMLButtonElement).click());
}
function clickDot(i: number): void {
  act(() => (container.querySelectorAll('.focus-dot')[i] as HTMLButtonElement).click());
}
function pressKey(key: string): void {
  act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })));
}

describe('FocusDeck navigation', () => {
  it('shows the first step with the next one peeking behind', () => {
    render({ blocks: three, interactions: empty() });
    expect(currentPrompt()).toBe('Ship one');
    expect(peekTitle()).toBe('Ship two');
  });

  it('the peek is a facade — it never mounts the next block', () => {
    render({ blocks: three, interactions: empty() });
    expect(container.querySelector('.focus-peek-title')).not.toBeNull();
    expect(container.querySelector('.focus-peek .approval')).toBeNull();
  });

  it('advances to the next step on Next', () => {
    render({ blocks: three, interactions: empty() });
    clickNext();
    expect(currentPrompt()).toBe('Ship two');
  });

  it('routes the j key to the deck instead of the ring', () => {
    render({ blocks: three, interactions: empty() });
    pressKey('j');
    expect(currentPrompt()).toBe('Ship two');
    pressKey('k');
    expect(currentPrompt()).toBe('Ship one');
  });

  it('jumps to a step when its dot is tapped', () => {
    render({ blocks: three, interactions: empty() });
    act(() => (container.querySelectorAll('.focus-dot')[2] as HTMLButtonElement).click());
    expect(currentPrompt()).toBe('Ship three');
  });

  it('lands on the review summary past the last step', () => {
    render({ blocks: [approval('a1', 'Ship one')], interactions: empty() });
    clickNext();
    expect(container.querySelector('.focus-card')).toBeNull();
    expect(container.querySelector('.focus-step-count')?.textContent).toBe('Review');
    expect(container.querySelector('.focus-summary-head')?.textContent).toBe('Review');
    expect(container.querySelector('.focus-receipt-title')?.textContent).toBe('Ship one');
  });
});

describe('FocusDeck live churn', () => {
  it('keeps its place on the same step when an earlier block is removed', () => {
    render({ blocks: three, interactions: empty() });
    clickNext();
    expect(currentPrompt()).toBe('Ship two');
    render({ blocks: [three[1]!, three[2]!], interactions: empty() });
    expect(currentPrompt()).toBe('Ship two');
  });

  it('clamps to the same position when the current anchor vanishes', () => {
    render({ blocks: three, interactions: empty() });
    clickNext();
    expect(currentPrompt()).toBe('Ship two');
    render({ blocks: [three[0]!, three[2]!], interactions: empty() });
    expect(currentPrompt()).toBe('Ship three');
  });

  it('resets to the first step when the round changes', () => {
    render({ blocks: three, interactions: empty() });
    clickNext();
    expect(currentPrompt()).toBe('Ship two');
    render({ blocks: three, interactions: empty(), round: 2 });
    expect(currentPrompt()).toBe('Ship one');
  });
});

describe('FocusDeck auto-advance', () => {
  it('advances an approval 450ms after its verdict lands', () => {
    vi.useFakeTimers();
    render({ blocks: three, interactions: empty() });
    render({ blocks: three, interactions: withVerdict('a1', 'approved') });
    act(() => vi.advanceTimersByTime(AUTO_ADVANCE_MS));
    expect(currentPrompt()).toBe('Ship two');
  });

  it('survives the echo re-render and still advances', () => {
    vi.useFakeTimers();
    render({ blocks: three, interactions: empty() });
    // The optimistic patch arms the timer; the SSE echo re-renders with a fresh
    // interactions object carrying the same verdict before 450ms elapses.
    render({ blocks: three, interactions: withVerdict('a1', 'approved') });
    render({ blocks: three, interactions: withVerdict('a1', 'approved') });
    act(() => vi.advanceTimersByTime(AUTO_ADVANCE_MS));
    expect(currentPrompt()).toBe('Ship two');
  });

  it('cancels the auto-advance while feedback is composing', () => {
    vi.useFakeTimers();
    render({ blocks: three, interactions: empty() });
    act(() => (container.querySelector('.focus-card .feedback-affordance .link-btn') as HTMLButtonElement).click());
    expect(container.querySelector('[data-composing]')).not.toBeNull();
    render({ blocks: three, interactions: withVerdict('a1', 'approved') });
    act(() => vi.advanceTimersByTime(AUTO_ADVANCE_MS));
    expect(currentPrompt()).toBe('Ship one');
  });

  it('does not re-arm when re-entering an already-decided approval', () => {
    vi.useFakeTimers();
    render({ blocks: three, interactions: withVerdict('a1', 'approved') });
    clickDot(1);
    expect(currentPrompt()).toBe('Ship two');
    clickDot(0);
    expect(currentPrompt()).toBe('Ship one');
    act(() => vi.advanceTimersByTime(AUTO_ADVANCE_MS));
    expect(currentPrompt()).toBe('Ship one');
  });

  it('clears a pending timer when navigating away before it fires', () => {
    vi.useFakeTimers();
    render({ blocks: three, interactions: empty() });
    render({ blocks: three, interactions: withVerdict('a1', 'approved') });
    act(() => vi.advanceTimersByTime(200));
    clickNext();
    expect(currentPrompt()).toBe('Ship two');
    act(() => vi.advanceTimersByTime(AUTO_ADVANCE_MS));
    expect(currentPrompt()).toBe('Ship two');
  });

  it('clears a pending timer on unmount with no post-unmount update', () => {
    vi.useFakeTimers();
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const c = document.createElement('div');
    document.body.appendChild(c);
    const r = createRoot(c);
    act(() => r.render(<Deck blocks={three} interactions={empty()} />));
    act(() => r.render(<Deck blocks={three} interactions={withVerdict('a1', 'approved')} />));
    act(() => r.unmount());
    act(() => vi.advanceTimersByTime(AUTO_ADVANCE_MS));
    expect(errorSpy).not.toHaveBeenCalled();
    errorSpy.mockRestore();
    c.remove();
  });
});

describe('FocusDeck next-undecided wrapping', () => {
  it('wraps to an earlier undecided step rather than landing on Review', () => {
    render({ blocks: three, interactions: withVerdicts({ a2: 'approved', a3: 'approved' }) });
    clickDot(2);
    expect(currentPrompt()).toBe('Ship three');
    pressKey('n');
    expect(currentPrompt()).toBe('Ship one');
    expect(container.querySelector('.focus-summary')).toBeNull();
  });

  it('lands on Review only when every step is decided', () => {
    render({ blocks: three, interactions: withVerdicts({ a1: 'approved', a2: 'approved', a3: 'approved' }) });
    pressKey('n');
    expect(container.querySelector('.focus-card')).toBeNull();
    expect(container.querySelector('.focus-summary-head')?.textContent).toBe('Review');
  });
});

describe('FocusDeck nested-decidable jump', () => {
  it('lands the cursor on a nested decidable when a SubmitBar dot jumps to it', () => {
    const cardBlock: Block = {
      id: 'c1',
      type: 'card',
      children: [
        approval('c1a', 'Approve me'),
        { id: 'c1c', type: 'choice', options: [{ id: 'o1', label: 'one' }] },
      ] as never,
    };
    const blocks = [cardBlock];
    const steps = focusSteps(blocks, new Set<string>());
    const interactions = empty();
    const doc: Doc = { version: 1, title: '', blocks };
    const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
    const queryClient = new QueryClient();
    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={blocks} interactions={interactions} closed={false} round={1}>
              <FocusDeck steps={steps} interactions={interactions} round={1} closed={false} />
              <SubmitBar blocks={blocks} doc={doc} interactions={interactions} subject="s" hasHistory={false} />
            </KeyboardProvider>
          </PresentContext.Provider>
        </QueryClientProvider>,
      );
    });
    // The cursor enters on the step's primary (the approval).
    expect(container.querySelector('.approval[data-kbd-cursor]')).not.toBeNull();
    expect(container.querySelector('.choice[data-kbd-cursor]')).toBeNull();
    // The SubmitBar dots map to [c1a, c1c]; tapping the second jumps to the nested choice.
    const dots = container.querySelectorAll('.submit-dots .dot');
    expect(dots.length).toBe(2);
    act(() => (dots[1] as HTMLButtonElement).click());
    expect(container.querySelector('.choice[data-kbd-cursor]')).not.toBeNull();
    expect(container.querySelector('.approval[data-kbd-cursor]')).toBeNull();
    queryClient.clear();
  });
});

describe('FocusDeck final-step removal', () => {
  it('clamps to the surviving neighbour rather than the summary', () => {
    render({ blocks: three, interactions: empty() });
    clickDot(2);
    expect(currentPrompt()).toBe('Ship three');
    render({ blocks: [three[0]!, three[1]!], interactions: empty() });
    expect(currentPrompt()).toBe('Ship two');
    expect(container.querySelector('.focus-summary')).toBeNull();
  });
});

describe('FocusDeck step-change focus and announce', () => {
  it('moves DOM focus into the new card and announces the step', () => {
    render({ blocks: three, interactions: empty() });
    clickNext();
    const card = container.querySelector('.focus-card');
    expect(card).not.toBeNull();
    expect(card!.contains(document.activeElement)).toBe(true);
    const live = container.querySelector('.sr-only[role="status"]');
    expect(live?.textContent).toBe('Step 2 of 3 — Ship two');
  });

  it('announces the review summary on reaching it', () => {
    render({ blocks: [approval('a1', 'Ship one')], interactions: empty() });
    clickNext();
    expect(container.querySelector('.focus-summary')).not.toBeNull();
    const live = container.querySelector('.sr-only[role="status"]');
    expect(live?.textContent).toBe('Review');
  });
});

describe('FocusDeck closed', () => {
  it('disables the verdict controls', () => {
    render({ blocks: three, interactions: empty(), closed: true });
    const buttons = [...container.querySelectorAll('.focus-card .verdict')] as HTMLButtonElement[];
    expect(buttons.length).toBeGreaterThan(0);
    expect(buttons.every((b) => b.disabled)).toBe(true);
  });
});

describe('view persistence', () => {
  it('round-trips a per-subject override', () => {
    saveView('subject-x', 'board');
    expect(loadView('subject-x')).toBe('board');
    saveView('subject-x', 'focus');
    expect(loadView('subject-x')).toBe('focus');
  });
});
