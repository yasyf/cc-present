// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { LazyMotion, MotionConfig, domMax } from 'motion/react';
import type { PanInfo } from 'motion/react';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { FocusCard, NO_DRAG, resolveDragEnd, swipeCommit } from './FocusCard';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import type { Interaction } from '../events';
import type { Block } from '../schema';

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

describe('swipeCommit', () => {
  const decision = (verdict: string): Interaction => ({ type: 'decision.created', blockId: 'a1', verdict } as Interaction);
  const cases: { name: string; offset: number; velocity: number; posted: Interaction | null; committed: boolean }[] = [
    { name: 'a right swipe past threshold posts approved', offset: 200, velocity: 0, posted: decision('approved'), committed: true },
    { name: 'a left swipe past threshold posts rejected', offset: -200, velocity: 0, posted: decision('rejected'), committed: true },
    { name: 'a rightward flick alone posts approved', offset: 0, velocity: 700, posted: decision('approved'), committed: true },
    { name: 'a sub-threshold drag posts nothing', offset: 50, velocity: 100, posted: null, committed: false },
  ];
  for (const c of cases) {
    it(c.name, () => {
      const post = vi.fn<(i: Interaction) => boolean>(() => true);
      const committed = swipeCommit(c.offset, c.velocity, 'a1', post);
      expect(committed).toBe(c.committed);
      if (c.posted) {
        expect(post).toHaveBeenCalledTimes(1);
        expect(post).toHaveBeenCalledWith(c.posted);
      } else {
        expect(post).not.toHaveBeenCalled();
      }
    });
  }
});

const approval = (id: string, prompt: string): Block => ({ id, type: 'approval', prompt });
const input = (id: string): Block => ({ id, type: 'input', label: id });

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

function renderCard(block: Block): void {
  const step = focusSteps([block], new Set())[0]!;
  const interactions = emptyState().interactions;
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <LazyMotion features={domMax} strict>
        <MotionConfig reducedMotion="user">
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
              <FocusCard step={step} interactions={interactions} />
            </KeyboardProvider>
          </PresentContext.Provider>
        </MotionConfig>
      </LazyMotion>,
    ),
  );
}

describe('FocusCard swipe affordances', () => {
  it('renders the swipe labels on a lone approval', () => {
    renderCard(approval('a1', 'Ship it'));
    expect(container.querySelector('.focus-card')).not.toBeNull();
    expect(container.querySelector('.approval-prompt')?.textContent).toBe('Ship it');
    expect(container.querySelector('.swipe-label.approve')).not.toBeNull();
    expect(container.querySelector('.swipe-label.reject')).not.toBeNull();
  });

  it('omits the swipe labels on a non-swipeable step', () => {
    renderCard(input('i1'));
    expect(container.querySelector('.focus-card')).not.toBeNull();
    expect(container.querySelector('.swipe-label')).toBeNull();
  });
});

const panInfo = (offsetX: number, velocityX = 0): PanInfo => ({
  point: { x: 0, y: 0 },
  delta: { x: 0, y: 0 },
  offset: { x: offsetX, y: 0 },
  velocity: { x: velocityX, y: 0 },
});

describe('resolveDragEnd release guards', () => {
  const open = { closed: false, present: true, reduced: false, primaryId: 'a1', dragPrimary: 'a1' };

  it('posts a committing swipe against a present, open, unchanged primary', () => {
    const post = vi.fn<(i: Interaction) => boolean>(() => true);
    expect(resolveDragEnd(panInfo(200), open, { set: vi.fn() }, post)).toBe(true);
    expect(post).toHaveBeenCalledWith({ type: 'decision.created', blockId: 'a1', verdict: 'approved' });
  });

  it('does not post against a card that has departed mid-drag', () => {
    const post = vi.fn<(i: Interaction) => boolean>(() => true);
    expect(resolveDragEnd(panInfo(200), { ...open, present: false }, { set: vi.fn() }, post)).toBe(false);
    expect(post).not.toHaveBeenCalled();
  });

  it('does not post once the round has closed', () => {
    const post = vi.fn<(i: Interaction) => boolean>(() => true);
    expect(resolveDragEnd(panInfo(200), { ...open, closed: true }, { set: vi.fn() }, post)).toBe(false);
    expect(post).not.toHaveBeenCalled();
  });

  it('does not post when a same-id upsert swapped the primary under the gesture', () => {
    const post = vi.fn<(i: Interaction) => boolean>(() => true);
    expect(resolveDragEnd(panInfo(200), { ...open, primaryId: 'a2', dragPrimary: 'a1' }, { set: vi.fn() }, post)).toBe(false);
    expect(post).not.toHaveBeenCalled();
  });

  it('resets x on release under reduced motion, and skips it under normal motion', () => {
    const reducedX = { set: vi.fn() };
    resolveDragEnd(panInfo(20), { ...open, reduced: true }, reducedX, vi.fn(() => true));
    expect(reducedX.set).toHaveBeenCalledWith(0);

    const normalX = { set: vi.fn() };
    resolveDragEnd(panInfo(20), open, normalX, vi.fn(() => true));
    expect(normalX.set).not.toHaveBeenCalled();
  });
});

describe('NO_DRAG selector', () => {
  const el = (html: string): HTMLElement => {
    const host = document.createElement('div');
    host.innerHTML = html;
    return host.firstElementChild as HTMLElement;
  };

  it('blocks ARIA and focusable pack targets a native selector would miss', () => {
    expect(el('<span role="button"><b>go</b></span>').querySelector('b')!.closest(NO_DRAG)).not.toBeNull();
    expect(el('<span role="link">go</span>').matches(NO_DRAG)).toBe(true);
    expect(el('<span tabindex="0">go</span>').matches(NO_DRAG)).toBe(true);
  });

  it('never blocks the card root, whose own tabindex is -1', () => {
    expect(el('<div tabindex="-1">card</div>').matches(NO_DRAG)).toBe(false);
    expect(el('<p>plain content</p>').matches(NO_DRAG)).toBe(false);
  });
});
