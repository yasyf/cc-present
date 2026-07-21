// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { Choice } from './Choice';
import { FocusStageContext } from './focusStep';
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
});

function render(block: ChoiceBlock, interactions: Interactions): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
          <Choice block={block} interactions={interactions} />
        </KeyboardProvider>
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

// renderWithStage mounts the choice under a FocusStageContext, so `stage` is
// non-null and the component is in focus mode — the stage owns option visuals.
function renderWithStage(block: ChoiceBlock, interactions: Interactions): void {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <FocusStageContext.Provider value={{ setVisual: () => {} }}>
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
  it('opts the container into data-strip at three or more options, with dots and arrows', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.hasAttribute('data-strip')).toBe(true);
    // One dot per card, including the trailing Other card (3 + 1 = 4).
    expect(container.querySelectorAll('.strip-nav .strip-dot').length).toBe(4);
    // Prev/next arrows render as icon buttons.
    expect(container.querySelectorAll('.strip-nav .btn-icon').length).toBe(2);
  });

  it('keeps the vertical stack with no nav at two or fewer options', () => {
    render(choice('c1', ['A', 'B']), empty());
    const options = container.querySelector('.options') as HTMLElement;
    expect(options.hasAttribute('data-strip')).toBe(false);
    expect(container.querySelector('.strip-nav')).toBeNull();
  });

  it('places the Other write-in as the trailing card inside the strip', () => {
    render(choice('c1', ['A', 'B', 'C']), empty());
    const cards = container.querySelectorAll('.options > .option');
    expect(cards.length).toBe(4);
    expect(cards[3]!.classList.contains('option-other')).toBe(true);
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
    // The field reads its value off the commit event, so seed it and blur (mirrors Input).
    const field = container.querySelector('.option-other textarea') as HTMLTextAreaElement;
    field.value = 'roll our own';
    act(() => field.dispatchEvent(new FocusEvent('focusout', { bubbles: true })));
    expect(posted).toContainEqual({ type: 'choice.selected', blockId: 'c1', optionIds: [], other: 'roll our own' });
  });

  it('renders an Add-note affordance whose feedback posts against the choice', () => {
    render(choice('c1', ['A', 'B']), empty());
    const link = container.querySelector('.feedback-affordance .link-btn');
    expect(link?.textContent).toBe('Add note');
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
    const present: PresentApi = { post: async () => true, closed: true, currentRound: 1 };
    const block = choice('c1', ['A', 'B']);
    const interactions = empty();
    act(() =>
      root.render(
        <PresentContext.Provider value={present}>
          <KeyboardProvider blocks={[block]} interactions={interactions} closed round={1}>
            <Choice block={block} interactions={interactions} />
          </KeyboardProvider>
        </PresentContext.Provider>,
      ),
    );
    const other = container.querySelector('.option-other') as HTMLElement;
    expect(other.getAttribute('aria-disabled')).toBe('true');
    expect(other.getAttribute('tabindex')).toBe('-1');
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

  it('mounts a visual-only option visual in its detail drill-down in board mode', () => {
    render(withVisual(), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    // The disclosure exists purely because the option carries a visual.
    expect(detail).not.toBeNull();
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelector('.detail-visual .image-block img')).not.toBeNull();
    expect(detail.querySelector<HTMLImageElement>('.detail-visual img')?.alt).toBe('chart-a');
  });

  it('renders the visual alongside detail content in the same drill-down', () => {
    render(withVisual({ pros: ['fast'] }), empty());
    const detail = container.querySelector('.option-detail') as HTMLElement;
    act(() => (detail.querySelector('.cc-group-header') as HTMLElement).click());
    expect(detail.querySelector('.detail-visual .image-block')).not.toBeNull();
    expect(detail.querySelector('.detail-pros')?.textContent).toContain('fast');
  });

  it('leaves a visual-only option to the stage in focus mode — no disclosure', () => {
    renderWithStage(withVisual(), empty());
    expect(container.querySelector('.option-detail')).toBeNull();
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
