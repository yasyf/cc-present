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
