// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { StatusPill } from './Card';
import { PresentContext } from '../present';
import type { CardStatus } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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

function render(status: CardStatus, closed: boolean): HTMLElement {
  act(() =>
    root.render(
      <PresentContext.Provider value={{ post: async () => true, closed, currentRound: 1 }}>
        <StatusPill status={status} />
      </PresentContext.Provider>,
    ),
  );
  return container.querySelector('.status')!;
}

describe('StatusPill', () => {
  it('shows an open card as open while the board is live', () => {
    const pill = render('open', false);
    expect(pill.textContent).toBe('open');
    expect(pill.className).toBe('status status-open');
  });

  it('shows an open card as closed once the board is closed', () => {
    const pill = render('open', true);
    expect(pill.textContent).toBe('closed');
    expect(pill.className).toBe('status status-closed');
  });

  it('keeps a settled status on a closed board', () => {
    expect(render('resolved', true).textContent).toBe('resolved');
  });
});
