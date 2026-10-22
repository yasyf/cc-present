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
import { ThreadHostContext } from './threadHost';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { Choice as ChoiceBlock, OptionVisual } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
// Clamped constructs a ResizeObserver and the keyboard reveal scrolls; jsdom ships
// neither.
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver = ResizeObserverStub;
Element.prototype.scrollIntoView = () => {};
let narrowViewport = false;
window.matchMedia = ((query: string) => ({
  matches: narrowViewport && query.includes('720px'),
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
  narrowViewport = false;
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

  it('lands digit n+1 in the write-in row', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    cursorOntoChoice();
    press('4');
    expect(document.activeElement).toBe(otherField());
  });

  it('scrolls the option a digit key picks into view', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    cursorOntoChoice();
    const reveal = vi.spyOn(Element.prototype, 'scrollIntoView');
    press('3');
    expect(reveal.mock.contexts.at(-1)).toBe(container.querySelectorAll('.option-row')[2]);
    expect(reveal).toHaveBeenLastCalledWith({ block: 'nearest' });
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

  it('renders aligned facts as a compare grid instead of per-option trays', () => {
    render(twoWithFacts(), empty());
    expect(container.querySelector('.choice-grid')).not.toBeNull();
    expect(container.querySelector('.option-facts')).toBeNull();
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

describe('Choice list shape', () => {
  it('lists every option as a full-width row with the Other write-in last, inside one group', () => {
    render(choice('c1', ['A', 'B', 'C', 'D', 'E']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.getAttribute('role')).toBe('radiogroup');
    expect(options.querySelectorAll(':scope > .option-row > .option').length).toBe(5);
    expect(options.lastElementChild?.classList.contains('option-other')).toBe(true);
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

  it('keeps the rail comment chip in its own row after the options', () => {
    renderRail(choice('c1', ['A', 'B', 'C']), empty());
    const row = container.querySelector('.choice > .comment-chip-row');
    expect(row?.querySelector('.comment-chip')?.textContent).toContain('Note on this decision');
    expect(row?.previousElementSibling?.classList.contains('options')).toBe(true);
  });

  it('names the options group after an inline prompt', () => {
    render({ ...choice('c1', ['A', 'B', 'C', 'D']), prompt: 'Pick a plan' }, empty());
    expect(container.querySelector('.choice-prompt')?.textContent).toBe('Pick a plan');
    expect(container.querySelector('[role="radiogroup"]')?.getAttribute('aria-label')).toBe('Pick a plan');
  });
});

describe('Choice compare grid', () => {
  const plans = (n = 3, multi = false): ChoiceBlock => ({
    id: 'c1',
    type: 'choice',
    multi,
    prompt: 'Pick a plan',
    options: Array.from({ length: n }, (_, i) => ({
      id: `o${i}`,
      label: `Plan ${i}`,
      recommended: i === 1,
      facts: [
        { label: 'Cost', value: `$${i + 1}`, tone: i === 0 ? ('good' as const) : undefined },
        { label: 'Latency', value: `${(i + 1) * 10}ms` },
      ],
    })),
  });
  const cellText = (selector: string) => [...container.querySelectorAll(selector)].map((el) => el.textContent);

  it('lays options out as columns with one row per fact axis and the picks along the foot', () => {
    render(plans(), empty());
    const grid = container.querySelector('.choice-grid') as HTMLElement;
    expect(grid.style.getPropertyValue('--grid-n')).toBe('3');
    expect(cellText('.grid-head .option-label')).toEqual(['Plan 0', 'Plan 1', 'Plan 2']);
    expect(container.querySelectorAll('.grid-head')[1]!.querySelector('.option-reco')).not.toBeNull();
    expect(cellText('.grid-axis:not(.grid-pick-axis)')).toEqual(['Cost', 'Latency']);
    expect(cellText('.grid-cell.fact .fact-value')).toEqual(['$1', '$2', '$3', '10ms', '20ms', '30ms']);
    expect(container.querySelector('.grid-cell.fact-good .fact-value')?.textContent).toBe('$1');
    expect(container.querySelectorAll('.grid-pick[role="radio"]').length).toBe(3);
  });

  it('keeps the Other write-in under the grid inside the same named group', () => {
    render(plans(), empty());
    const group = container.querySelector('.choice-compare') as HTMLElement;
    expect(group.getAttribute('role')).toBe('radiogroup');
    expect(group.getAttribute('aria-label')).toBe('Pick a plan');
    expect([...group.children].map((child) => child.classList[0])).toEqual(['choice-grid', 'option']);
    expect(group.lastElementChild?.classList.contains('option-other')).toBe(true);
  });

  it('posts the same choice.selected from a grid pick as from the list row', () => {
    const fromGrid = renderCapturing(plans(), empty());
    act(() => container.querySelectorAll<HTMLElement>('.grid-pick')[2]!.click());
    narrowViewport = true;
    remount();
    const fromList = renderCapturing(plans(), empty());
    expect(container.querySelector('.choice-grid')).toBeNull();
    act(() => container.querySelectorAll<HTMLElement>('.options .option')[2]!.click());
    expect(fromGrid).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: ['o2'] }]);
    expect(fromList).toEqual(fromGrid);
  });

  it('marks the picked column and checks its pick cell', () => {
    render(plans(3, true), { ...empty(), choices: { c1: { optionIds: ['o1'] } } });
    const picks = container.querySelectorAll('.grid-pick[role="checkbox"]');
    expect([...picks].map((p) => p.getAttribute('aria-checked'))).toEqual(['false', 'true', 'false']);
    expect(picks[1]!.querySelector('.mark-check')).not.toBeNull();
    expect(container.querySelectorAll('.grid-head')[1]!.classList.contains('selected')).toBe(true);
  });

  it('picks a column on its digit key once the cursor rests on the grid', () => {
    const posted = renderCapturing(plans(), empty());
    act(() => container.querySelector('.grid-pick')!.dispatchEvent(new FocusEvent('focusin', { bubbles: true })));
    expect(cellText('.grid-pick .option-index')).toEqual(['1', '2', '3']);
    press('2');
    expect(posted).toEqual([{ type: 'choice.selected', blockId: 'c1', optionIds: ['o1'] }]);
  });

  it('scrolls the pick cell of the column a digit key picks into view', () => {
    render(plans(), empty());
    act(() => container.querySelector('.grid-pick')!.dispatchEvent(new FocusEvent('focusin', { bubbles: true })));
    const reveal = vi.spyOn(Element.prototype, 'scrollIntoView');
    press('3');
    expect(reveal.mock.contexts.at(-1)).toBe(container.querySelectorAll('.grid-pick')[2]);
  });

  it('falls back to the list shape on a narrow viewport', () => {
    narrowViewport = true;
    render(plans(), empty());
    expect(container.querySelector('.choice-grid')).toBeNull();
    expect(container.querySelectorAll('.options .option-facts').length).toBe(3);
  });

  it('shows the hovered or picked column visual full-width under the grid, and nothing at rest', () => {
    const block = plans(3);
    block.options[0]!.visual = { id: 'v0', type: 'image', src: 'data:,x', alt: 'chart-0' };
    block.options[2]!.visual = { id: 'v2', type: 'image', src: 'data:,x', alt: 'chart-2' };
    const shown = () => container.querySelector<HTMLImageElement>('.choice-compare > .option-visual img')?.alt ?? null;
    const at = (selector: string, i: number) => container.querySelectorAll<HTMLElement>(selector)[i]!;
    const pointer = (el: HTMLElement, type: string, clientX: number) =>
      act(() => void el.dispatchEvent(new MouseEvent(type, { bubbles: true, clientX, clientY: 10 })));

    render(block, { ...empty(), choices: { c1: { optionIds: ['o2'] } } });
    expect(container.querySelector('.choice-grid img')).toBeNull();
    expect(shown()).toBe('chart-2');

    pointer(at('.grid-cell.fact', 0), 'mousemove', 50);
    expect(shown()).toBe('chart-0');
    pointer(at('.grid-head', 1), 'mousemove', 60);
    expect(shown()).toBeNull();
    pointer(at('.grid-head', 1), 'mouseout', 400);
    expect(shown()).toBe('chart-2');

    render(block, empty());
    expect(shown()).toBeNull();
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

describe('Choice inline option visual', () => {
  const visual = (n: number): OptionVisual => ({ id: `v${n}`, type: 'image', src: 'data:,x', alt: `chart-${n}` });
  const withVisuals = (multi = false): ChoiceBlock => ({
    id: 'c1',
    type: 'choice',
    multi,
    options: [0, 1, 2].map((i) => ({ id: `o${i}`, label: `Option ${i}`, visual: visual(i) })),
  });
  const row = (i: number) => container.querySelectorAll<HTMLElement>('.option-row')[i]!;
  const fire = (i: number, event: Event) => act(() => void row(i).querySelector('.option')!.dispatchEvent(event));
  const pointer = (type: string, i: number, clientX: number) =>
    fire(i, new MouseEvent(type, { bubbles: true, clientX, clientY: 10 }));
  const shown = () =>
    [...container.querySelectorAll('.option-row')].flatMap((r, i) =>
      r.querySelector(':scope > .option-visual img') ? [i] : [],
    );

  it('shows no visual and no placeholder before a hover or a pick', () => {
    render(withVisuals(), empty());
    expect(shown()).toEqual([]);
    expect(container.textContent).not.toContain('No preview');
  });

  it('renders nothing beneath an option without a visual', () => {
    const block = withVisuals();
    delete block.options[1]!.visual;
    render(block, empty());
    pointer('mousemove', 1, 50);
    expect(container.querySelector('.option-visual')).toBeNull();
  });

  it('shows a hovered option visual beneath it and drops it once the pointer leaves', () => {
    render(withVisuals(), empty());
    pointer('mousemove', 1, 50);
    expect(shown()).toEqual([1]);
    expect(row(1).querySelector<HTMLImageElement>('.option-visual img')?.alt).toBe('chart-1');
    pointer('mouseout', 1, 400);
    expect(shown()).toEqual([]);
  });

  it('returns to the selected option visual once the pointer or focus leaves', () => {
    render(withVisuals(), { ...empty(), choices: { c1: { optionIds: ['o2'] } } });
    expect(shown()).toEqual([2]);

    pointer('mousemove', 0, 50);
    expect(shown()).toEqual([0]);
    pointer('mouseout', 0, 400);
    expect(shown()).toEqual([2]);

    fire(1, new FocusEvent('focusin', { bubbles: true }));
    expect(shown()).toEqual([1]);
    fire(1, new FocusEvent('focusout', { bubbles: true }));
    expect(shown()).toEqual([2]);
  });

  it('holds the hovered visual when a layout shift slides a neighbour under a still pointer', () => {
    render(withVisuals(), { ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    pointer('mousemove', 1, 50);
    expect(shown()).toEqual([1]);

    pointer('mouseout', 1, 50);
    pointer('mouseover', 2, 50);
    pointer('mousemove', 2, 50);
    expect(shown()).toEqual([1]);

    pointer('mousemove', 2, 51);
    expect(shown()).toEqual([2]);
  });

  it('moves to the option a digit key picks', () => {
    render(withVisuals(), empty());
    cursorOntoChoice();
    expect(shown()).toEqual([0]);
    press('2');
    render(withVisuals(), { ...empty(), choices: { c1: { optionIds: ['o1'] } } });
    expect(shown()).toEqual([1]);
  });

  it('follows the most recently checked option still checked on a multi-select', () => {
    const block = withVisuals(true);
    render(block, { ...empty(), choices: { c1: { optionIds: ['o0', 'o2'] } } });
    expect(shown()).toEqual([2]);
    render(block, { ...empty(), choices: { c1: { optionIds: ['o0', 'o2', 'o1'] } } });
    expect(shown()).toEqual([1]);
  });

  it('keeps detail content in the drill-down and the visual out of it', () => {
    const block = withVisuals();
    block.options[0]!.detail = { pros: ['fast'] };
    render(block, { ...empty(), choices: { c1: { optionIds: ['o0'] } } });
    const detail = container.querySelector('.option-detail') as HTMLElement;
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelector('.detail-pros')?.textContent).toContain('fast');
    expect(detail.querySelector('img')).toBeNull();
    expect(row(0).querySelector(':scope > .option-visual img')).not.toBeNull();
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
