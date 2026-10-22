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
import type { FocusReveal } from './FocusCard';
import { ThreadHostContext } from './threadHost';
import type { ThreadHost } from './threadHost';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import { DECAY_MS, revisionStore } from '../revision';
import { FOCUS_DWELL_MS, viewedStore } from '../viewed';
import type { Interaction, Interactions, PresentState, Revising, WireFrame } from '../events';
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
const staged = (id: string): Block =>
  ({
    id,
    type: 'choice',
    prompt: 'Which transport?',
    options: [
      { id: `${id}o0`, label: 'A' },
      { id: `${id}o1`, label: 'B', visual: { id: `${id}o1v`, type: 'code', lang: 'go', code: 'package main' } },
    ],
  }) as Block;

// renderStep renders the step focusSteps derives at `index` from a block run, so a
// step's demoted lead-in context (the run before its anchor) can be exercised.
function renderStep(
  blocks: Block[],
  index = 0,
  host: ThreadHost = 'inline',
  { interactions = emptyState().interactions, reveal = null }: { interactions?: Interactions; reveal?: FocusReveal | null } = {},
): void {
  const step = focusSteps(blocks, new Set())[index]!;
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <LazyMotion features={domMax} strict>
        <MotionConfig reducedMotion="user">
          <PresentContext.Provider value={present}>
            <ThreadHostContext.Provider value={host}>
              <KeyboardProvider blocks={blocks} interactions={interactions} closed={false} round={1}>
                <FocusCard step={step} interactions={interactions} reveal={reveal} />
              </KeyboardProvider>
            </ThreadHostContext.Provider>
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
  viewedStore.reset();
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
              <FocusCard step={step} interactions={interactions} reveal={null} />
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

  it('reserves the empty revision landing slot', () => {
    renderStep([choice('c1', 'Which transport?', ['A', 'B'])]);
    expect(container.querySelector('.focus-revision')).not.toBeNull();
  });

  it('heads a titled single-decidable card with its title and keeps the prompt as the sub-question', () => {
    renderStep([card('k1', 'Transport choice', [choice('c1', 'Which transport?', ['A', 'B'])])]);
    expect(container.querySelector('.focus-question')?.textContent).toBe('Transport choice');
    expect(container.querySelector('.focus-meta-eyebrow')).toBeNull();
    expect(container.querySelector('.card-head')).toBeNull();
    expect(container.querySelector('.choice-prompt')?.textContent).toBe('Which transport?');
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

  it('keeps option visuals out of any preview column, with no placeholder', () => {
    renderStep([staged('c1')]);
    const body = container.querySelector('.focus-card-body')!;
    expect(body.hasAttribute('data-stage')).toBe(false);
    expect(container.querySelector('.focus-media')).toBeNull();
    expect(container.textContent).not.toContain('No preview');
  });

  it('orders the face as lede, decidable, note row, then one Background disclosure', () => {
    renderStep([markdown('m1', 'Lead-in'), code('code1'), card('k1', 'Transport choice', [staged('c1')])], 0, 'rail');
    const body = container.querySelector('.focus-card-body')!;
    expect([...body.children].map((child) => child.classList[0])).toEqual([
      'clamped',
      'card',
      'comment-chip-row',
      'focus-background',
    ]);
    expect(body.querySelector(':scope > .card .choice')).not.toBeNull();
  });

  it('clamps the lede to three lines', () => {
    renderStep([markdown('m1', 'lead-in prose'), approval('a1', 'Ship it?')]);
    const lede = container.querySelector('.focus-lede') as HTMLElement;
    expect(lede.textContent).toBe('lead-in prose');
    expect(lede.style.getPropertyValue('--clamp-lines')).toBe('3');
    expect(container.querySelector('.focus-background')).toBeNull();
  });

  it('leaves a lede carrying a table unclamped', () => {
    renderStep([markdown('m1', '| PR | Verdict |\n| --- | --- |\n| #1 | keep |'), approval('a1', 'Ship it?')]);
    const lede = container.querySelector('.focus-lede')!;
    expect(lede.querySelector('table')).not.toBeNull();
    expect(lede.closest('.clamped')).toBeNull();
  });

  it('leads with the card summary and moves the card context into Background', () => {
    const withSummary: Block = {
      id: 'k1',
      type: 'card',
      title: 'Transport choice',
      summary: 'Pick *one* transport',
      children: [markdown('m1', 'history') as never, choice('c1', 'Which?', ['A', 'B']) as never],
    };
    renderStep([withSummary]);
    expect(container.querySelector('.focus-lede')?.innerHTML).toContain('<em>one</em>');
    expect(container.querySelector('.card-summary')).toBeNull();
    expect(container.querySelector('.card .markdown-block')).toBeNull();
    const background = container.querySelector('.focus-background')!;
    act(() => (background.querySelector('.cc-group-header') as HTMLElement).click());
    expect(background.querySelector('.markdown-block')?.textContent).toBe('history');
  });

  it('collects the remaining context into one collapsed Background disclosure', () => {
    const table: Block = { id: 't1', type: 'table', columns: [{ key: 'pr', label: 'PR' }], rows: [] };
    renderStep([markdown('m1', 'lead'), table, code('code1'), approval('a1', 'Ship it?')]);
    const groups = container.querySelectorAll('.cc-group');
    expect(groups.length).toBe(1);
    const background = container.querySelector('.focus-background .cc-group')!;
    expect(background.querySelector('.cc-group-header')?.textContent).toContain('Background');
    expect(background.classList.contains('cc-group-expanded')).toBe(false);
    expect(background.querySelector('.code-block')).toBeNull();
    act(() => (background.querySelector('.cc-group-header') as HTMLElement).click());
    expect(background.querySelector('.table-block')).not.toBeNull();
    expect(background.querySelector('.code-block')).not.toBeNull();
    expect(container.querySelector('.focus-question')?.textContent).toBe('Ship it?');
  });

  it('opens Background and reveals the block a jump targets inside it', () => {
    const reveal = vi.spyOn(Element.prototype, 'scrollIntoView');
    renderStep([markdown('m1', 'lead'), code('k1'), approval('a1', 'Ship it?')], 0, 'inline', {
      reveal: { id: 'k1', seq: 1 },
    });
    const background = container.querySelector('.focus-background .cc-group')!;
    expect(background.classList.contains('cc-group-expanded')).toBe(true);
    const target = background.querySelector('[data-block-id="k1"]');
    expect(target?.querySelector('.code-block')).not.toBeNull();
    expect(reveal.mock.contexts).toContain(target);
    reveal.mockRestore();
  });

  it('leaves Background closed for a jump to a block outside it', () => {
    renderStep([markdown('m1', 'lead'), code('k1'), approval('a1', 'Ship it?')], 0, 'inline', {
      reveal: { id: 'a1', seq: 1 },
    });
    expect(container.querySelector('.focus-background .cc-group-expanded')).toBeNull();
  });

  it('counts notes on a card child moved into Background on the card chip', () => {
    const k = card('k1', 'Transport choice', [markdown('m1', 'history'), choice('c1', 'Which?', ['A', 'B'])]);
    renderStep([k], 0, 'rail', {
      interactions: { ...emptyState().interactions, feedback: { m1: [{ id: 'f1', text: 'why?' }] } },
    });
    expect(container.querySelector('.focus-card-body > .comment-chip-row .comment-chip')?.textContent).toBe('1 comment');
  });

  it('renders a context step in full, with no Background', () => {
    renderStep([markdown('m1', 'first'), code('code1')]);
    expect(container.querySelector('.focus-background')).toBeNull();
    expect(container.querySelector('.focus-lede')).toBeNull();
    expect(container.querySelector('.markdown-block')?.textContent).toBe('first');
    expect(container.querySelector('.code-block')).not.toBeNull();
  });
});

function docState(ids: string[], revising: Revising = { blockIds: [] }): PresentState {
  const base = emptyState();
  return { ...base, doc: { ...base.doc, blocks: ids.map((id) => markdown(id, id)) }, revising };
}
const upsertFrame = (id: string): WireFrame => ({ schemaVersion: 1, type: 'block.upserted', block: markdown(id, id) });
const revisingFrame = (ids: string[], note?: string): WireFrame => ({
  schemaVersion: 1,
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

describe('FocusCard viewed tracking', () => {
  it('marks a step and its card children viewed once it dwells', () => {
    vi.useFakeTimers();
    renderCard(card('c1', 'Pick', [approval('a1', 'Ship?')]));
    act(() => vi.advanceTimersByTime(FOCUS_DWELL_MS - 1));
    expect(viewedStore.viewed()).toEqual([]);
    act(() => vi.advanceTimersByTime(1));
    expect(viewedStore.viewed()).toEqual(['c1', 'a1']);
  });

  it('re-arms the dwell when a child joins the step it already marked', () => {
    vi.useFakeTimers();
    renderCard(card('c1', 'Pick', [approval('a1', 'Ship?')]));
    act(() => vi.advanceTimersByTime(FOCUS_DWELL_MS));
    renderCard(card('c1', 'Pick', [approval('a1', 'Ship?'), code('k1')]));
    act(() => vi.advanceTimersByTime(FOCUS_DWELL_MS - 1));
    expect(viewedStore.viewed()).toEqual(['c1', 'a1']);
    act(() => vi.advanceTimersByTime(1));
    expect(viewedStore.viewed()).toEqual(['c1', 'a1', 'k1']);
  });

  it('marks context held in a closed Background viewed once the step dwells', () => {
    vi.useFakeTimers();
    renderStep([markdown('m1', 'lead'), code('k1'), approval('a1', 'Ship?')]);
    act(() => vi.advanceTimersByTime(FOCUS_DWELL_MS));
    expect(container.querySelector('.focus-background .cc-group-expanded')).toBeNull();
    expect(viewedStore.viewed()).toEqual(['m1', 'k1', 'a1']);
  });

  it('never marks a step the human skips past before the dwell', () => {
    vi.useFakeTimers();
    renderCard(markdown('b1', 'body'));
    act(() => vi.advanceTimersByTime(FOCUS_DWELL_MS / 2));
    renderCard(markdown('b2', 'body'));
    act(() => vi.advanceTimersByTime(FOCUS_DWELL_MS));
    expect(viewedStore.viewed()).toEqual(['b2']);
  });
});
