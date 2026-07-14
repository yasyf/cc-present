// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { Choice } from './Choice';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { Choice as ChoiceBlock } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
Element.prototype.scrollIntoView = () => {};
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
    const tags = [...container.querySelectorAll('.option-index')];
    expect(tags.map((t) => t.textContent)).toEqual(['1', '2', '3']);
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
