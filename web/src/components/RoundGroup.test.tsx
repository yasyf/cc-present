// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { RoundGroup } from './RoundGroup';
import { emptyState } from '../reduce';
import type { Interactions, RoundRecord, Selection } from '../events';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
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

// A one-choice closed round whose only selection is the passed one; the round stays
// collapsed, so only its header summary chips render (children never mount).
function record(choice: Selection): RoundRecord {
  return {
    number: 1,
    blocks: [{ id: 'c1', type: 'choice', multi: false, options: [{ id: 'o0', label: 'A' }] }],
    decisions: {},
    choices: { c1: choice },
    inputs: {},
    packs: {},
    feedback: {},
  };
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
});

function picksChip(): HTMLElement | undefined {
  return [...container.querySelectorAll('.round-chip')].find((c) => /pick/.test(c.textContent ?? '')) as
    | HTMLElement
    | undefined;
}

describe('RoundGroup picks chip', () => {
  it('counts a write-in-only choice, matching isDecided', () => {
    act(() => root.render(<RoundGroup record={record({ optionIds: [], other: 'roll our own' })} interactions={empty()} />));
    expect(picksChip()?.textContent).toBe('1 pick');
  });

  it('counts an authored option selection', () => {
    act(() => root.render(<RoundGroup record={record({ optionIds: ['o0'] })} interactions={empty()} />));
    expect(picksChip()?.textContent).toBe('1 pick');
  });

  it('does not count a fully cleared choice', () => {
    act(() => root.render(<RoundGroup record={record({ optionIds: [] })} interactions={empty()} />));
    expect(picksChip()).toBeUndefined();
  });
});
