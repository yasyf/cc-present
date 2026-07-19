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
import { DECAY_MS, revisionStore } from '../revision';
import type { Interaction, PresentState, Revising, WireFrame } from '../events';
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
const choice = (id: string, prompt: string, labels: string[]): Block => ({
  id,
  type: 'choice',
  prompt,
  options: labels.map((label, i) => ({ id: `${id}o${i}`, label })),
});
const markdown = (id: string, md: string): Block => ({ id, type: 'markdown', md });
const code = (id: string): Block => ({ id, type: 'code', lang: 'go', code: 'package main' });
const card = (id: string, title: string, children: Block[]): Block => ({
  id,
  type: 'card',
  title,
  children: children as never,
});

// renderStep renders the step focusSteps derives at `index` from a block run, so a
// step's demoted lead-in context (the run before its anchor) can be exercised.
function renderStep(blocks: Block[], index = 0): void {
  const step = focusSteps(blocks, new Set())[index]!;
  const interactions = emptyState().interactions;
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <LazyMotion features={domMax} strict>
        <MotionConfig reducedMotion="user">
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={blocks} interactions={interactions} closed={false} round={1}>
              <FocusCard step={step} interactions={interactions} />
            </KeyboardProvider>
          </PresentContext.Provider>
        </MotionConfig>
      </LazyMotion>,
    ),
  );
}

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
  vi.useRealTimers();
  revisionStore.reset();
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
    // The prompt is hoisted into the pinned question, not the in-body approval prompt.
    expect(container.querySelector('.focus-question')?.textContent).toBe('Ship it');
    expect(container.querySelector('.approval-prompt')).toBeNull();
    expect(container.querySelector('.swipe-label.approve')).not.toBeNull();
    expect(container.querySelector('.swipe-label.reject')).not.toBeNull();
  });

  it('omits the swipe labels on a non-swipeable step', () => {
    renderCard(input('i1'));
    expect(container.querySelector('.focus-card')).not.toBeNull();
    expect(container.querySelector('.swipe-label')).toBeNull();
  });

  it('never renders the deleted focus-tier eyebrow', () => {
    renderCard(approval('a1', 'Ship it'));
    expect(container.querySelector('.focus-tier')).toBeNull();
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

describe('FocusCard question-first anatomy', () => {
  it('hoists a lone choice prompt into the question and suppresses the in-body copy', () => {
    renderStep([choice('c1', 'Which transport?', ['A', 'B'])]);
    const q = container.querySelector('.focus-question');
    expect(q?.textContent).toBe('Which transport?');
    expect(container.querySelector('.choice-prompt')).toBeNull();
    // a11y survives the suppression: the options group carries the prompt.
    expect(container.querySelector('.options')?.getAttribute('aria-label')).toBe('Which transport?');
    // The card is labelled by the hoisted heading.
    expect(container.querySelector('.focus-card')?.getAttribute('aria-labelledby')).toBe(q?.id);
  });

  it('reserves the empty revision and media landing slots', () => {
    renderStep([choice('c1', 'Which transport?', ['A', 'B'])]);
    expect(container.querySelector('.focus-revision')).not.toBeNull();
    expect(container.querySelector('.focus-media')).not.toBeNull();
  });

  it('heads a single-decidable card with the prompt and demotes the title to the meta eyebrow', () => {
    renderStep([card('k1', 'Transport choice', [choice('c1', 'Which transport?', ['A', 'B'])])]);
    expect(container.querySelector('.focus-question')?.textContent).toBe('Which transport?');
    expect(container.querySelector('.focus-meta-eyebrow')?.textContent).toBe('Transport choice');
    // The in-body card head is hoisted out, not duplicated.
    expect(container.querySelector('.card-head')).toBeNull();
    expect(container.querySelector('.choice-prompt')).toBeNull();
  });

  it('heads a multi-decidable card with its title and keeps the child prompts inline', () => {
    renderStep([
      card('k1', 'Two calls', [choice('c1', 'First?', ['A', 'B']), choice('c2', 'Second?', ['C', 'D'])]),
    ]);
    expect(container.querySelector('.focus-question')?.textContent).toBe('Two calls');
    expect(container.querySelector('.focus-meta-eyebrow')).toBeNull();
    expect(container.querySelector('.card-head')).toBeNull();
    const prompts = [...container.querySelectorAll('.choice-prompt')].map((p) => p.textContent);
    expect(prompts).toEqual(['First?', 'Second?']);
  });

  it('mounts the active option visual in the focus-media stage', () => {
    const withVisual: Block = {
      id: 'c1',
      type: 'choice',
      prompt: 'Which transport?',
      options: [
        { id: 'o0', label: 'A', visual: { id: 'o0v', type: 'code', lang: 'go', code: 'package main' } },
        { id: 'o1', label: 'B' },
      ],
    } as Block;
    renderStep([withVisual]);
    // The first option is active at rest, so its visual mounts in the stage.
    expect(container.querySelector('.focus-media .code-block')).not.toBeNull();
  });

  it('collapses the option-visual stage to a titled disclosure at the compact breakpoint', () => {
    const original = window.matchMedia;
    window.matchMedia = ((query: string) => ({
      matches: query.includes('440px'),
      media: query,
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    })) as typeof window.matchMedia;
    try {
      const withVisual: Block = {
        id: 'c1',
        type: 'choice',
        prompt: 'Which transport?',
        options: [
          { id: 'o0', label: 'A', visual: { id: 'o0v', type: 'code', lang: 'go', code: 'package main' } },
          { id: 'o1', label: 'B' },
        ],
      } as Block;
      renderStep([withVisual]);
      const media = container.querySelector('.focus-media');
      expect(media?.tagName).toBe('DETAILS');
      expect(media?.querySelector('summary.focus-media-summary')?.textContent).toBe('go');
      // The visual still mounts, now inside the disclosure body.
      expect(container.querySelector('.focus-media .code-block')).not.toBeNull();
    } finally {
      window.matchMedia = original;
    }
  });

  it('demotes markdown lead-in to a clamped block and heavy blocks to titled disclosures', () => {
    renderStep([markdown('m1', 'lead-in prose'), code('code1'), approval('a1', 'Ship it?')], 0);
    const context = container.querySelector('.focus-context');
    expect(context).not.toBeNull();
    expect(context?.querySelector('.markdown-block.focus-context-md')).not.toBeNull();
    // The code block collapses behind a disclosure titled by its language.
    const header = context?.querySelector('.cc-group-header');
    expect(header?.textContent).toContain('go');
    expect(container.querySelector('.focus-question')?.textContent).toBe('Ship it?');
  });
});

function docState(ids: string[], revising: Revising = { blockIds: [] }): PresentState {
  const base = emptyState();
  return { ...base, doc: { ...base.doc, blocks: ids.map((id) => markdown(id, id)) }, revising };
}
const upsertFrame = (id: string): WireFrame => ({ type: 'block.upserted', block: markdown(id, id) });
const revisingFrame = (ids: string[], note?: string): WireFrame => ({
  type: 'revising.changed',
  blockIds: ids,
  ...(note !== undefined ? { note } : {}),
});
const revisionLine = (): { cls: string; text: string } => {
  const el = container.querySelector('.focus-revision-line');
  if (!el) throw new Error('no .focus-revision-line rendered');
  return { cls: el.className, text: el.textContent ?? '' };
};

describe('FocusCard live-revision callout', () => {
  it('warns while the step is being revised (controls stay live)', () => {
    revisionStore.ingest(revisingFrame(['b1'], 'reworking per your pick'), docState(['b1']), {
      blockIds: ['b1'],
      note: 'reworking per your pick',
    });
    renderCard(markdown('b1', 'body'));
    const l = revisionLine();
    expect(l.cls).toContain('revising');
    expect(l.text).toBe('Claude is rewriting this step — reworking per your pick');
  });

  it('drops the banner to a passive line after the 120s decay', () => {
    vi.useFakeTimers();
    revisionStore.ingest(revisingFrame(['b1'], 'reworking'), docState(['b1']), { blockIds: ['b1'], note: 'reworking' });
    renderCard(markdown('b1', 'body'));
    expect(revisionLine().cls).toContain('revising');
    act(() => vi.advanceTimersByTime(DECAY_MS));
    const l = revisionLine();
    expect(l.cls).toContain('passive');
    expect(l.text).toBe('Claude may still be revising this step');
  });

  it('shows the revised callout on arrival at a changed step', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsertFrame('b1'), docState(['b1'], { blockIds: ['b1'], note: 'updated for step 1' }), {
      blockIds: [],
    });
    renderCard(markdown('b1', 'body'));
    const l = revisionLine();
    expect(l.cls).toContain('callout');
    expect(l.text).toBe('Updated after your earlier pick — updated for step 1');
  });

  it('shows the added callout, note omitted gracefully', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsertFrame('b2'), docState(['b1']), { blockIds: [] });
    renderCard(markdown('b2', 'body'));
    const l = revisionLine();
    expect(l.cls).toContain('callout');
    expect(l.text).toBe('Claude added this step');
  });

  it('marks a step seen when the human leaves it', () => {
    revisionStore.markLive();
    revisionStore.ingest(upsertFrame('b1'), docState(['b1']), { blockIds: [] });
    renderCard(markdown('b1', 'body'));
    expect(revisionStore.unseenChange('b1')).not.toBeNull();
    renderCard(markdown('b2', 'body'));
    expect(revisionStore.unseenChange('b1')).toBeNull();
  });
});
