// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { CollapsedGroup } from '@cc-interact/react';
import { draftStore } from '../drafts';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { Choice } from './Choice';
import { FocusStageContext } from './focusStep';
import type { FocusStageValue } from './focusStep';
import { ThreadHostContext } from './threadHost';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { Choice as ChoiceBlock, OptionVisual } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
// OptionStrip's useScrollEdges constructs a ResizeObserver; jsdom ships none, and
// the strip nav drives scroll APIs the layout-less DOM lacks.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;
Element.prototype.scrollIntoView = () => {};
Element.prototype.scrollBy = () => {};
Element.prototype.scrollTo = () => {};
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

const empty = (): Interactions => emptyState().interactions;
const choice = (id: string, labels: string[], multi = false): ChoiceBlock => ({
  id,
  type: 'choice',
  multi,
  options: labels.map((label, i) => ({ id: `o${i}`, label })),
});

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
  vi.restoreAllMocks();
  draftStore.reset();
});

function render(block: ChoiceBlock, interactions: Interactions, closed = false): void {
  const present: PresentApi = { post: async () => true, closed, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[block]} interactions={interactions} closed={closed} round={1}>
          <Choice block={block} interactions={interactions} />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

function renderRail(block: ChoiceBlock, interactions: Interactions): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <ThreadHostContext.Provider value="rail">
          <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
            <Choice block={block} interactions={interactions} />
          </KeyboardProvider>
        </ThreadHostContext.Provider>
      </PresentContext.Provider>,
    ),
  );
}

// renderCapturing renders like render but returns the array every post() lands in,
// so a test can assert whether a click did or did not select an option.
function renderCapturing(block: ChoiceBlock, interactions: Interactions): unknown[] {
  const posted: unknown[] = [];
  const present: PresentApi = {
    post: async (ev) => {
      posted.push(ev);
      return true;
    },
    closed: false,
    currentRound: 1,
  };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
          <Choice block={block} interactions={interactions} />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
  return posted;
}

function renderPending(block: ChoiceBlock): {
  posted: unknown[];
  settle: ((ok: boolean) => void)[];
  show: (interactions: Interactions) => void;
} {
  const posted: unknown[] = [];
  const settle: ((ok: boolean) => void)[] = [];
  const present: PresentApi = {
    post: (ev) => {
      posted.push(ev);
      return new Promise((resolve) => settle.push(resolve));
    },
    closed: false,
    currentRound: 1,
  };
  const show = (interactions: Interactions): void =>
    act(() =>
      root.render(
        <PresentContext.Provider value={present}>
          <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
            <Choice block={block} interactions={interactions} />
          </KeyboardProvider>
        </PresentContext.Provider>,
      ),
    );
  return { posted, settle, show };
}

function remount(): void {
  act(() => root.unmount());
  root = createRoot(container);
}

// renderWithStage mounts the choice under a FocusStageContext, so `stage` is
// non-null and the component is in focus mode — the stage owns option visuals.
function renderWithStage(
  block: ChoiceBlock,
  interactions: Interactions,
  setVisual: FocusStageValue['setVisual'] = () => {},
  staged = true,
): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <FocusStageContext.Provider value={{ staged, setVisual }}>
          <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
            <Choice block={block} interactions={interactions} />
          </KeyboardProvider>
        </FocusStageContext.Provider>
      </PresentContext.Provider>,
    ),
  );
}

// Landing the cursor on the block: focusing an option bubbles a focusin the
// provider maps to the choice's decidable handle.
function cursorOntoChoice(): void {
  const opt = container.querySelector('.option') as HTMLElement;
  act(() => opt.dispatchEvent(new FocusEvent('focusin', { bubbles: true })));
}

function press(key: string): void {
  act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })));
}

function otherCard(): HTMLElement {
  return container.querySelector('.option-other') as HTMLElement;
}

function otherField(): HTMLTextAreaElement {
  return container.querySelector('.option-other textarea') as HTMLTextAreaElement;
}

function typeOther(value: string): void {
  const field = otherField();
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
  act(() => {
    setter.call(field, value);
    field.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function blurOther(): void {
  act(() => otherField().dispatchEvent(new FocusEvent('focusout', { bubbles: true })));
}

function modEnter(target: HTMLElement, mod: KeyboardEventInit): KeyboardEvent {
  const ev = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true, ...mod });
  act(() => {
    target.dispatchEvent(ev);
  });
  return ev;
}

describe('Choice keyboard index tags', () => {
  it('renders no index tags until the cursor rests on the block', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    expect(container.querySelectorAll('.option-index').length).toBe(0);
  });

  it('tags each option 1..n once the choice is cursored', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    cursorOntoChoice();
    // The write-in card rides index n+1, so scope to the authored option cards.
    const tags = [...container.querySelectorAll('.option:not(.option-other) .option-index')];
    expect(tags.map((t) => t.textContent)).toEqual(['1', '2', '3']);
    // The chrome write-in card takes the next index (4) so a key lands the composer.
    expect(container.querySelector('.option-other .option-index')?.textContent).toBe('4');
  });

  it('lands digit n+1 in the write-in row under the strip', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    cursorOntoChoice();
    press('4');
    expect(document.activeElement).toBe(otherField());
  });

  it('caps the index tags at the 1-9 keymap', () => {
    render(
      choice(
        'c1',
        Array.from({ length: 11 }, (_, i) => `opt ${i}`),
      ),
      empty(),
    );
    cursorOntoChoice();
    expect(container.querySelectorAll('.option-index').length).toBe(9);
  });
});

describe('Choice selection mark', () => {
  it('draws a ring on the selected single-select option and a check on a multi one', () => {
    render(choice('c1', ['A', 'B']), { ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    expect(container.querySelector('.option.selected .option-indicator .mark-ring')).not.toBeNull();

    render(choice('c2', ['A', 'B'], true), { ...empty(), choices: { c2: { optionIds: ['o1'] } } });
    expect(container.querySelector('.option.selected .option-indicator .mark-check')).not.toBeNull();
  });
});

describe('Choice facts cluster', () => {
  it('renders each fact value with its optional label and tone class', () => {
    const block: ChoiceBlock = {
      id: 'c1',
      type: 'choice',
      multi: false,
      options: [{ id: 'o0', label: 'A', facts: [{ label: 'cost', value: '$5', tone: 'good' }, { value: '9ms' }] }],
    };
    render(block, empty());
    const facts = container.querySelectorAll('.option-facts .fact');
    expect(facts.length).toBe(2);
    const first = facts[0] as HTMLElement;
    const second = facts[1] as HTMLElement;
    expect(first.classList.contains('fact-good')).toBe(true);
    expect(first.querySelector('.fact-value')?.textContent).toBe('$5');
    expect(first.querySelector('.fact-label')?.textContent).toBe('cost');
    expect(second.classList.contains('fact-default')).toBe(true);
    expect(second.querySelector('.fact-label')).toBeNull();
  });

  it('renders fact value and label as literal text, not markdown', () => {
    const block: ChoiceBlock = {
      id: 'c1',
      type: 'choice',
      multi: false,
      options: [{ id: 'o0', label: 'A', facts: [{ label: '*load*', value: '*fast*' }] }],
    };
    render(block, empty());
    const value = container.querySelector('.fact-value') as HTMLElement;
    const label = container.querySelector('.fact-label') as HTMLElement;
    expect(value.textContent).toBe('*fast*');
    expect(value.querySelector('em')).toBeNull();
    expect(label.textContent).toBe('*load*');
    expect(label.querySelector('em')).toBeNull();
  });
});

describe('Choice detail disclosure', () => {
  const withDetail = (detail: ChoiceBlock['options'][number]['detail']): ChoiceBlock => ({
    id: 'c1',
    type: 'choice',
    multi: false,
    options: [{ id: 'o0', label: 'A', detail }],
  });

  it('reveals pros, cons, and unclamped md when the inline disclosure expands', () => {
    render(withDetail({ pros: ['fast'], cons: ['pricey'], md: 'the *why*' }), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    expect(detail.querySelector('.cc-group-header')?.textContent).toContain('Details');
    expect(detail.querySelector('.detail-body')).toBeNull();

    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelector('.detail-pros')?.textContent).toContain('fast');
    expect(detail.querySelector('.detail-cons')?.textContent).toContain('pricey');
    expect(detail.querySelector('.detail-md.prose')?.innerHTML).toContain('<em>why</em>');
    expect(detail.querySelector('.clamped')).toBeNull();
  });

  it('renders pros and cons as literal text, not markdown', () => {
    render(withDetail({ pros: ['*fast*'], cons: ['*pricey*'] }), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    const pro = detail.querySelector('.detail-pros .detail-text') as HTMLElement;
    const con = detail.querySelector('.detail-cons .detail-text') as HTMLElement;
    expect(pro.textContent).toBe('*fast*');
    expect(pro.querySelector('em')).toBeNull();
    expect(con.textContent).toBe('*pricey*');
    expect(con.querySelector('em')).toBeNull();
  });

  it('renders detail.md as block markdown, so lists and paragraphs form', () => {
    render(withDetail({ md: '- one\n- two' }), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelectorAll('.detail-md.prose li').length).toBe(2);
  });

  it('never selects the option when its Details trigger is clicked', () => {
    const posted = renderCapturing(withDetail({ pros: ['fast'] }), empty());
    act(() => (container.querySelector('.cc-group-header') as HTMLElement).click());
    expect(posted).toEqual([]);
    act(() => (container.querySelector('.option') as HTMLElement).click());
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: ['o0'] }]);
  });

  it('leaves mod+Enter on the Details control to the global submit path, and keeps plain Enter', () => {
    const heard: string[] = [];
    const listen = (e: KeyboardEvent): void => {
      heard.push(e.metaKey || e.ctrlKey ? 'chord' : 'plain');
    };
    document.addEventListener('keydown', listen);
    for (const [mode, control] of [
      ['inline', '.cc-group-header'],
      ['modal', '.detail-trigger'],
    ] as const) {
      render(withDetail({ md: 'why', mode }), empty());
      const target = container.querySelector(`.option-detail ${control}`) as HTMLElement;
      modEnter(target, { metaKey: true });
      modEnter(target, { ctrlKey: true });
      modEnter(target, {});
    }
    document.removeEventListener('keydown', listen);

    expect(heard).toEqual(['chord', 'chord', 'chord', 'chord']);
  });

  it('renders a modal detail as a trigger button over a dialog, not a CollapsedGroup', () => {
    render(withDetail({ md: 'why', mode: 'modal' }), empty());
    expect(container.querySelector('.option-detail .detail-trigger')).not.toBeNull();
    expect(container.querySelector('.option-detail dialog.detail-modal')).not.toBeNull();
    expect(container.querySelector('.option-detail .cc-group')).toBeNull();
  });
});

describe('Choice per-card facts', () => {
  const twoWithFacts = (): ChoiceBlock => ({
    id: 'c1',
    type: 'choice',
    multi: false,
    options: [
      { id: 'o0', label: 'A', facts: [{ label: 'Latency', value: '12ms', tone: 'good' }, { label: 'Cost', value: '$5' }] },
      { id: 'o1', label: 'B', facts: [{ label: 'Latency', value: '80ms' }, { label: 'Cost', value: '$2', tone: 'bad' }] },
    ],
  });

  it('renders each option facts as its own foot tray in factAxes order', () => {
    render(twoWithFacts(), empty());
    const options = container.querySelector('.options') as HTMLElement;
    // The subgrid alignment system is gone: no aligned attr, no shared axis header.
    expect(options.hasAttribute('data-facts-aligned')).toBe(false);
    expect(container.querySelector('.fact-axes')).toBeNull();
    expect(container.querySelector('.fact-cell')).toBeNull();
    // Each option carries its own tray with rows in the shared axis order.
    const trays = container.querySelectorAll('.option-facts');
    expect(trays.length).toBe(2);
    const firstRows = [...trays[0]!.querySelectorAll('.fact')].map((f) => [
      f.querySelector('.fact-value')?.textContent,
      f.querySelector('.fact-label')?.textContent,
    ]);
    expect(firstRows).toEqual([
      ['12ms', 'Latency'],
      ['$5', 'Cost'],
    ]);
    // Tone survives onto each row.
    expect(trays[0]!.querySelector('.fact-good .fact-value')?.textContent).toBe('12ms');
    expect(trays[1]!.querySelector('.fact-bad .fact-value')?.textContent).toBe('$2');
  });

  it('keeps each option facts in its own authored order on a label mismatch', () => {
    const mismatch: ChoiceBlock = {
      id: 'c1',
      type: 'choice',
      multi: false,
      options: [
        { id: 'o0', label: 'A', facts: [{ label: 'Latency', value: '12ms' }] },
        { id: 'o1', label: 'B', facts: [{ label: 'Speed', value: '80ms' }] },
      ],
    };
    render(mismatch, empty());
    const trays = container.querySelectorAll('.option-facts');
    expect(trays.length).toBe(2);
    expect(trays[0]!.querySelector('.fact .fact-value')?.textContent).toBe('12ms');
    expect(trays[0]!.querySelector('.fact .fact-label')?.textContent).toBe('Latency');
    expect(trays[1]!.querySelector('.fact .fact-label')?.textContent).toBe('Speed');
  });
});

describe('Choice card strip', () => {
  it('opts the container into data-strip at three or more options, sized by its card count', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.hasAttribute('data-strip')).toBe(true);
    expect(options.style.getPropertyValue('--strip-n')).toBe('3');
    expect(options.parentElement?.classList.contains('strip-frame')).toBe(true);
    expect(container.querySelectorAll('.strip-nav .strip-dot').length).toBe(3);
    expect(container.querySelectorAll('.strip-arrow').length).toBe(0);
  });

  it('mirrors the scroll edges onto the strip container and arrows only the hidden edges', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    Object.defineProperties(options, {
      clientWidth: { configurable: true, value: 300 },
      scrollWidth: { configurable: true, value: 900 },
      scrollLeft: { configurable: true, writable: true, value: 0 },
    });
    const arrows = () => [...container.querySelectorAll('.strip-frame > .strip-arrow')].map((a) => a.getAttribute('aria-label'));

    act(() => options.dispatchEvent(new Event('scroll')));
    expect(options.hasAttribute('data-edge-start')).toBe(true);
    expect(options.hasAttribute('data-edge-end')).toBe(false);
    expect(arrows()).toEqual(['Next options']);

    options.scrollLeft = 300;
    act(() => options.dispatchEvent(new Event('scroll')));
    expect(options.hasAttribute('data-edge-start')).toBe(false);
    expect(options.hasAttribute('data-edge-end')).toBe(false);
    expect(arrows()).toEqual(['Previous options', 'Next options']);

    options.scrollLeft = 600;
    act(() => options.dispatchEvent(new Event('scroll')));
    expect(options.hasAttribute('data-edge-start')).toBe(false);
    expect(options.hasAttribute('data-edge-end')).toBe(true);
    expect(arrows()).toEqual(['Previous options']);
  });

  it('clamps an option description at eight lines', () => {
    const block = choice('c1', ['A', 'B', 'C']);
    block.options[0]!.md = 'A long description.';
    render(block, empty());
    const md = container.querySelector('.option-md') as HTMLElement;
    expect(md.style.getPropertyValue('--clamp-lines')).toBe('8');
  });

  it('marks the choice closed so its locked cards keep full opacity', () => {
    render(choice('c1', ['A', 'B', 'C']), empty(), true);
    expect(container.querySelector('.choice')?.hasAttribute('data-closed')).toBe(true);
    expect(container.querySelector('.option')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('places the rail comment chip between the strip dots and arrows', () => {
    renderRail(choice('c1', ['A', 'B', 'C']), empty());
    const nav = container.querySelector('.strip-nav') as HTMLElement;
    expect(nav.querySelector(':scope > .comment-chip-row .comment-chip')?.textContent).toContain('Note on this decision');
    expect([...nav.children].map((child) => child.className)).toEqual(['strip-dots', 'comment-chip-row']);
    expect(container.querySelector('.choice > .comment-chip-row')).toBeNull();
  });

  it('keeps every option in the vertical stack beside a focus stage', () => {
    renderWithStage(choice('c1', ['A', 'B', 'C', 'D']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.hasAttribute('data-strip')).toBe(false);
    expect(options.querySelectorAll('.option:not(.option-other)').length).toBe(4);
    expect(container.querySelector('.strip-nav')).toBeNull();
    expect(container.querySelector('.strip-arrow')).toBeNull();
  });

  it('keeps the strip in a focus step without a stage', () => {
    renderWithStage(choice('c1', ['A', 'B', 'C']), empty(), () => {}, false);
    expect(container.querySelector('.options')?.hasAttribute('data-strip')).toBe(true);
  });

  it('keeps the vertical stack with no nav at two or fewer options', () => {
    render(choice('c1', ['A', 'B']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.hasAttribute('data-strip')).toBe(false);
    expect(container.querySelector('.strip-nav')).toBeNull();
  });

  it('keeps the rail comment chip in its own row for the vertical stack', () => {
    renderRail(choice('c1', ['A', 'B']), empty());
    expect(container.querySelector('.strip-nav')).toBeNull();
    const row = container.querySelector('.choice > .comment-chip-row');
    expect(row?.querySelector('.comment-chip')?.textContent).toContain('Note on this decision');
    expect(row?.previousElementSibling?.classList.contains('options')).toBe(true);
  });

  it('places the Other write-in as a row under the strip, inside the options group', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    expect(container.querySelectorAll('.options > .option').length).toBe(3);
    expect(container.querySelector('.options .option-other')).toBeNull();
    const group = container.querySelector('.strip-group') as HTMLElement;
    expect(group.getAttribute('role')).toBe('radiogroup');
    expect(container.querySelector('.options')?.hasAttribute('role')).toBe(false);
    expect([...group.children].map((child) => child.classList.contains('option-other'))).toEqual([false, true]);
  });

  it('keeps the Other write-in inside the vertical stack at two or fewer options', () => {
    render(choice('c1', ['A', 'B']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.getAttribute('role')).toBe('radiogroup');
    expect(options.lastElementChild?.classList.contains('option-other')).toBe(true);
  });

  it('names the options group after an inline prompt in both layouts', () => {
    render({ ...choice('c1', ['A', 'B']), prompt: 'Pick a plan' }, empty());
    expect(container.querySelector('.choice-prompt')?.textContent).toBe('Pick a plan');
    expect(container.querySelector('[role="radiogroup"]')?.getAttribute('aria-label')).toBe('Pick a plan');
    render({ ...choice('c1', ['A', 'B', 'C', 'D']), prompt: 'Pick a plan' }, empty());
    expect(container.querySelector('.strip-group')?.getAttribute('aria-label')).toBe('Pick a plan');
  });

  it('reveals a clicked card, the Other card included', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    const reveal = vi.spyOn(Element.prototype, 'scrollIntoView');
    const cards = container.querySelectorAll<HTMLElement>('.options > .option');
    const other = container.querySelector<HTMLElement>('.option-other')!;

    act(() => cards[1]!.click());
    expect(reveal).toHaveBeenCalledTimes(1);
    expect(reveal.mock.contexts[0]).toBe(cards[1]);
    expect(reveal).toHaveBeenLastCalledWith({ inline: 'nearest', block: 'nearest', behavior: 'smooth' });

    act(() => other.click());
    expect(reveal).toHaveBeenCalledTimes(2);
    expect(reveal.mock.contexts[1]).toBe(other);
  });

  it('marks the dot of each selected card, clearing it for an uncommitted write-in', () => {
    render(choice('c1', ['A', 'B', 'C']), { ...empty(), choices: { c1: { optionIds: ['o1'] } } });
    const pickedDots = () => [...container.querySelectorAll('.strip-dot')].map((dot) => dot.classList.contains('picked'));
    expect(pickedDots()).toEqual([false, true, false]);
    const dots = container.querySelectorAll('.strip-dot');
    expect(dots[0]!.getAttribute('aria-label')).toBe('Go to option 1');
    expect(dots[1]!.getAttribute('aria-label')).toBe('Go to option 2, selected');
    expect(container.querySelector('.strip-dot[aria-current]')).toBeNull();

    typeOther('roll our own');
    expect(pickedDots()).toEqual([false, false, false]);
  });
});

describe('Choice universal escape hatch', () => {
  it('always renders a write-in row with an inline field', () => {
    render(choice('c1', ['A', 'B']), empty());
    const row = container.querySelector('.option-other');
    expect(row).not.toBeNull();
    expect(row?.querySelector('.option-label')?.textContent).toBe('Other');
    expect(container.querySelector('.focus-card .choice input, .choice .option-other textarea')).not.toBeNull();
  });

  it('commits a single-select write-in as the sole answer, clearing authored picks', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), {
      ...empty(),
      choices: { c1: { optionIds: ['o0'] } },
    });
    typeOther('roll our own');
    blurOther();
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'roll our own' }]);
  });

  it('renders a decision note affordance whose feedback posts against the choice', () => {
    render(choice('c1', ['A', 'B']), empty());
    const link = container.querySelector('.feedback-affordance .link-btn');
    expect(link?.textContent).toBe('Note on this decision');
  });
});

describe('Choice write-in selection semantics', () => {
  it('gives the single-select Other card radio semantics, unchecked and focusable when empty', () => {
    render(choice('c1', ['A', 'B']), empty());
    const other = container.querySelector('.option-other') as HTMLElement;
    expect(other.getAttribute('role')).toBe('radio');
    expect(other.getAttribute('aria-checked')).toBe('false');
    expect(other.getAttribute('aria-disabled')).toBe('false');
    expect(other.getAttribute('tabindex')).toBe('0');
  });

  it('gives the multi-select Other card checkbox semantics', () => {
    render(choice('c1', ['A', 'B'], true), empty());
    const other = container.querySelector('.option-other') as HTMLElement;
    expect(other.getAttribute('role')).toBe('checkbox');
    expect(other.getAttribute('aria-checked')).toBe('false');
  });

  it('marks the Other card checked once a write-in value selects it', () => {
    render(choice('c1', ['A', 'B']), { ...empty(), choices: { c1: { optionIds: [], other: 'roll our own' } } });
    const other = container.querySelector('.option-other') as HTMLElement;
    expect(other.getAttribute('aria-checked')).toBe('true');
  });

  it('disables the Other card when the block is closed', () => {
    render(choice('c1', ['A', 'B']), empty(), true);
    const other = container.querySelector('.option-other') as HTMLElement;
    expect(other.getAttribute('aria-disabled')).toBe('true');
    expect(other.getAttribute('tabindex')).toBe('-1');
  });

  it('checks Other and unchecks the authored pick as soon as the user types, posting nothing', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), { ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    typeOther('roll our own');
    expect(otherCard().getAttribute('aria-checked')).toBe('true');
    expect(otherCard().querySelector('.option-indicator .mark-ring')).not.toBeNull();
    expect(container.querySelector('.option')?.getAttribute('aria-checked')).toBe('false');
    expect(posted).toEqual([]);
  });

  it('posts a typed write-in once on Enter', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), empty());
    typeOther('roll our own');
    const field = otherField();
    act(() => field.focus());
    const enter = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true });
    act(() => {
      field.dispatchEvent(enter);
    });
    expect(enter.defaultPrevented).toBe(true);
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'roll our own' }]);
  });

  it('keeps the text when an authored pick takes the answer, and re-posts it when Other is clicked', () => {
    const block = choice('c1', ['A', 'B']);
    const posted = renderCapturing(block, empty());
    typeOther('roll our own');
    blurOther();
    act(() => (container.querySelector('.option') as HTMLElement).click());
    expect(posted[1]).toEqual({ type: 'choice.selected', blockId: 'c1', optionIds: ['o0'] });
    expect(otherField().value).toBe('roll our own');
    expect(otherCard().getAttribute('aria-checked')).toBe('false');

    act(() => otherCard().click());
    expect(posted[2]).toEqual({ type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'roll our own' });
    expect(posted.length).toBe(3);
    expect(otherCard().getAttribute('aria-checked')).toBe('true');
    expect(document.activeElement).toBe(otherField());
  });

  it('keeps the draft through a blur while the interactions still carry the old answer', () => {
    const block = choice('c1', ['A', 'B']);
    const posted = renderCapturing(block, empty());
    typeOther('keep me');
    blurOther();
    expect(otherField().value).toBe('keep me');

    act(() => (otherCard().querySelector('.option-label') as HTMLElement).click());
    expect(otherField().value).toBe('keep me');
    expect(otherCard().getAttribute('aria-checked')).toBe('true');
    expect(posted.length).toBe(1);

    render(block, empty());
    expect(otherField().value).toBe('keep me');
  });

  it('re-selects the kept write-in on its digit key', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), empty());
    typeOther('roll our own');
    blurOther();
    act(() => (container.querySelector('.option') as HTMLElement).click());
    cursorOntoChoice();
    press('3');
    expect(posted[2]).toEqual({ type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'roll our own' });
    expect(posted.length).toBe(3);
    expect(otherCard().getAttribute('aria-checked')).toBe('true');
  });

  it('unchecks a multi-select Other on a chrome click, keeping the text unselected', () => {
    const block = choice('c1', ['A', 'B'], true);
    const posted = renderCapturing(block, { ...empty(), choices: { c1: { optionIds: ['o0'], other: 'mine' } } });
    act(() => otherField().click());
    expect(posted).toEqual([]);

    act(() => otherCard().click());
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: ['o0'] }]);

    const later = renderCapturing(block, { ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    expect(otherField().value).toBe('mine');
    expect(otherCard().getAttribute('aria-checked')).toBe('false');
    blurOther();
    expect(later).toEqual([]);
    expect(otherCard().getAttribute('aria-checked')).toBe('false');
  });

  it('deselects Other when the box is emptied, posting the empty answer on blur', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), { ...empty(), choices: { c1: { optionIds: [], other: 'mine' } } });
    typeOther('');
    expect(otherCard().getAttribute('aria-checked')).toBe('false');
    expect(posted).toEqual([]);
    blurOther();
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: [] }]);
  });

  it('adopts a newly committed write-in into an idle clean box, but never a cleared one', () => {
    const block = choice('c1', ['A', 'B']);
    render(block, { ...empty(), choices: { c1: { optionIds: [], other: 'first' } } });
    expect(otherField().value).toBe('first');

    render(block, { ...empty(), choices: { c1: { optionIds: [], other: 'second' } } });
    expect(otherField().value).toBe('second');

    render(block, { ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    expect(otherField().value).toBe('second');
    expect(otherCard().getAttribute('aria-checked')).toBe('false');
  });

  it('keeps an edited write-in typed and checked when its POST fails and the answer rolls back', async () => {
    const answered = (other: string): Interactions => ({ ...empty(), choices: { c1: { optionIds: [], other } } });
    const { posted, settle, show } = renderPending(choice('c1', ['A', 'B']));

    show(answered('first'));
    typeOther('first, revised');
    blurOther();
    show(answered('first, revised'));
    show(answered('first'));
    await act(async () => settle[0]!(false));

    expect(otherField().value).toBe('first, revised');
    expect(otherCard().getAttribute('aria-checked')).toBe('true');

    blurOther();
    expect(posted).toEqual([
      { type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'first, revised' },
      { type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'first, revised' },
    ]);
  });

  it('leaves an authored pick standing when an earlier write-in POST fails after it', async () => {
    const { posted, settle, show } = renderPending(choice('c1', ['A', 'B']));

    show(empty());
    typeOther('mine');
    blurOther();
    act(() => (container.querySelector('.option') as HTMLElement).click());
    show({ ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    await act(async () => settle[0]!(false));

    expect(otherCard().getAttribute('aria-checked')).toBe('false');
    expect(container.querySelector('.option')?.getAttribute('aria-checked')).toBe('true');
    expect(otherField().value).toBe('mine');

    blurOther();
    expect(posted).toEqual([
      { type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'mine' },
      { type: 'choice.selected', blockId: 'c1', optionIds: ['o0'] },
    ]);
  });
});

describe('Choice kept write-in', () => {
  const block = choice('c1', ['A', 'B']);
  const picked = (): Interactions => ({ ...empty(), choices: { c1: { optionIds: ['o0'] } } });

  it('shows the kept text again after the choice remounts, still unselected', () => {
    render(block, empty());
    typeOther('roll our own');
    blurOther();
    act(() => (container.querySelector('.option') as HTMLElement).click());
    remount();

    const posted = renderCapturing(block, picked());
    expect(otherField().value).toBe('roll our own');
    expect(otherCard().getAttribute('aria-checked')).toBe('false');
    expect(container.querySelector('.option')?.getAttribute('aria-checked')).toBe('true');

    act(() => otherCard().click());
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'roll our own' }]);
  });

  it('forgets the text once the box is emptied', () => {
    render(block, empty());
    typeOther('roll our own');
    expect(draftStore.get('c1', 'other')).toBe('roll our own');
    typeOther('');
    expect(draftStore.get('c1', 'other')).toBeUndefined();
    remount();

    render(block, empty());
    expect(otherField().value).toBe('');
  });

  it('shows a committed write-in over the kept text', () => {
    draftStore.set('c1', 'other', 'never sent');
    render(block, { ...empty(), choices: { c1: { optionIds: [], other: 'sent' } } });
    expect(otherField().value).toBe('sent');
    expect(otherCard().getAttribute('aria-checked')).toBe('true');
  });

  it('neither shows nor replaces the kept text in a read-only round', () => {
    const present: PresentApi = { post: async () => true, closed: false, currentRound: 2 };
    const frozen = (interactions: Interactions): void =>
      act(() =>
        root.render(
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={[]} interactions={interactions} closed={false} round={2}>
              <CollapsedGroup defaultExpanded readOnly header="Round 1">
                <Choice block={block} interactions={interactions} />
              </CollapsedGroup>
            </KeyboardProvider>
          </PresentContext.Provider>,
        ),
      );
    draftStore.set('c1', 'other', 'this round');

    frozen(picked());
    expect(otherField().value).toBe('');
    remount();
    frozen({ ...empty(), choices: { c1: { optionIds: [], other: 'last round' } } });

    expect(otherField().value).toBe('last round');
    expect(draftStore.get('c1', 'other')).toBe('this round');
  });
});

describe('Choice submit chord', () => {
  it('leaves mod+Enter in the write-in to the global submit path', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), empty());
    typeOther('roll our own');
    expect(modEnter(otherField(), { metaKey: true }).defaultPrevented).toBe(false);
    expect(modEnter(otherField(), { ctrlKey: true }).defaultPrevented).toBe(false);
    expect(posted).toEqual([]);
  });

  it('leaves mod+Enter on a focused card to the global submit path', () => {
    const posted = renderCapturing(choice('c1', ['A', 'B']), empty());
    expect(modEnter(container.querySelector('.option') as HTMLElement, { metaKey: true }).defaultPrevented).toBe(false);
    expect(modEnter(otherCard(), { ctrlKey: true }).defaultPrevented).toBe(false);
    expect(posted).toEqual([]);
  });
});

describe('Choice stage preview', () => {
  const visual = (n: number): OptionVisual => ({ id: `v${n}`, type: 'image', src: 'data:,x', alt: `chart-${n}` });
  const visuals = [visual(0), visual(1), visual(2)];
  const withVisuals = (multi = false): ChoiceBlock => ({
    id: 'c1',
    type: 'choice',
    multi,
    options: visuals.map((v, i) => ({ id: `o${i}`, label: `Option ${i}`, visual: v })),
  });
  const card = (i: number) => container.querySelectorAll<HTMLElement>('.option')[i]!;
  const fire = (i: number, event: Event) => act(() => void card(i).dispatchEvent(event));
  const pointer = (type: string, i: number, clientX: number) =>
    fire(i, new MouseEvent(type, { bubbles: true, clientX, clientY: 10 }));

  it('stages the first option before any hover, then holds the last hovered card after the pointer leaves', () => {
    const setVisual = vi.fn<FocusStageValue['setVisual']>();
    renderWithStage(withVisuals(), empty(), setVisual);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[0]);

    pointer('mousemove', 1, 50);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);
    pointer('mouseout', 1, 400);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);
  });

  it('returns to the selected option once the pointer or focus rests', () => {
    const setVisual = vi.fn<FocusStageValue['setVisual']>();
    renderWithStage(withVisuals(), { ...empty(), choices: { c1: { optionIds: ['o2'] } } }, setVisual);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[2]);

    pointer('mousemove', 0, 50);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[0]);
    pointer('mouseout', 0, 400);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[2]);
    pointer('mousemove', 0, 50);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[0]);
    pointer('mouseout', 0, 400);

    fire(1, new FocusEvent('focusin', { bubbles: true }));
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);
    fire(1, new FocusEvent('focusout', { bubbles: true }));
    expect(setVisual).toHaveBeenLastCalledWith(visuals[2]);
  });

  it('keeps the picked card staged when a scroll slides a neighbour under a still pointer', () => {
    const setVisual = vi.fn<FocusStageValue['setVisual']>();
    renderWithStage(withVisuals(), { ...empty(), choices: { c1: { optionIds: ['o1'] } } }, setVisual);
    pointer('mousemove', 1, 50);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);

    pointer('mouseout', 1, 50);
    pointer('mouseover', 0, 50);
    pointer('mousemove', 0, 50);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);

    pointer('mousemove', 0, 51);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[0]);
  });

  it('moves to the option a digit key picks', () => {
    const setVisual = vi.fn<FocusStageValue['setVisual']>();
    renderWithStage(withVisuals(), empty(), setVisual);
    cursorOntoChoice();
    expect(setVisual).toHaveBeenLastCalledWith(visuals[0]);
    press('2');
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);
  });

  it('follows the most recently checked option still checked on a multi-select', () => {
    const setVisual = vi.fn<FocusStageValue['setVisual']>();
    const block = withVisuals(true);
    renderWithStage(block, { ...empty(), choices: { c1: { optionIds: ['o0', 'o2'] } } }, setVisual);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[2]);

    renderWithStage(block, { ...empty(), choices: { c1: { optionIds: ['o0', 'o2', 'o1'] } } }, setVisual);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[1]);

    renderWithStage(block, { ...empty(), choices: { c1: { optionIds: ['o0', 'o2'] } } }, setVisual);
    expect(setVisual).toHaveBeenLastCalledWith(visuals[2]);
  });
});

describe('Choice board-mode option visual', () => {
  const imageVisual: OptionVisual = { id: 'v0', type: 'image', src: 'data:,x', alt: 'chart-a' };
  const withVisual = (detail?: ChoiceBlock['options'][number]['detail']): ChoiceBlock => ({
    id: 'c1',
    type: 'choice',
    multi: false,
    options: [{ id: 'o0', label: 'A', visual: imageVisual, detail }],
  });

  it('renders the active option visual full-width above the options in board mode', () => {
    render(withVisual(), empty());
    expect(container.querySelector('.option-detail')).toBeNull();
    const visual = container.querySelector('.choice > .choice-visual') as HTMLElement;
    expect(visual.nextElementSibling?.classList.contains('options')).toBe(true);
    expect(visual.querySelector<HTMLImageElement>('.image-block img')?.alt).toBe('chart-a');
  });

  it('keeps detail content in the drill-down and the visual above the options', () => {
    render(withVisual({ pros: ['fast'] }), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelector('.detail-pros')?.textContent).toContain('fast');
    expect(detail.querySelector('.detail-visual')).toBeNull();
    expect(container.querySelector('.choice-visual .image-block')).not.toBeNull();
  });

  it('leaves a visual-only option to the stage in focus mode — no disclosure', () => {
    renderWithStage(withVisual(), empty());
    expect(container.querySelector('.option-detail')).toBeNull();
    expect(container.querySelector('.choice-visual')).toBeNull();
  });

  it('shows only detail in the focus-mode disclosure, never double-rendering the visual', () => {
    renderWithStage(withVisual({ pros: ['fast'] }), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    expect(detail).not.toBeNull();
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelector('.detail-pros')?.textContent).toContain('fast');
    expect(detail.querySelector('.detail-visual')).toBeNull();
  });
});

describe('Choice recommended badge', () => {
  it('stamps the recommended option and marks the row', () => {
    const block: ChoiceBlock = {
      id: 'c1',
      type: 'choice',
      multi: false,
      options: [
        { id: 'o0', label: 'A', recommended: true },
        { id: 'o1', label: 'B' },
      ],
    };
    render(block, empty());
    const rows = container.querySelectorAll('.option');
    expect(rows[0]!.classList.contains('recommended')).toBe(true);
    expect(rows[1]!.classList.contains('recommended')).toBe(false);
    const reco = rows[0]!.querySelector('.option-label .option-reco');
    expect(reco).not.toBeNull();
    expect(rows[1]!.querySelector('.option-reco')).toBeNull();
  });
});
