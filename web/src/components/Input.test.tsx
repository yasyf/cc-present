// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { Input } from './Input';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { Input as InputBlock } from '../schema';

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
const block: InputBlock = { id: 'i1', type: 'input', label: 'Name' };

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

function renderWith(post: PresentApi['post']): void {
  const present: PresentApi = { post, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[block]} interactions={empty()} closed={false} round={1}>
          <Input block={block} interactions={empty()} />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

// A commit reads the field value on blur; React's onBlur rides the bubbling
// focusout, so seed the value and dispatch that.
async function commit(value: string): Promise<void> {
  const field = container.querySelector('input') as HTMLInputElement;
  field.value = value;
  await act(async () => {
    field.dispatchEvent(new FocusEvent('focusout', { bubbles: true }));
  });
}

describe('Input commit tick', () => {
  it('keeps the success tick after the daemon accepts the value', async () => {
    renderWith(async () => true);
    await commit('Ada');
    expect(container.querySelector('.input-tick')).not.toBeNull();
  });

  it('retracts the success tick when the post is rejected', async () => {
    renderWith(async () => false);
    await commit('Ada');
    expect(container.querySelector('.input-tick')).toBeNull();
  });
});
