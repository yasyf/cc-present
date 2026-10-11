// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { Input } from './Input';
import { draftStore } from '../drafts';
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
afterEach(() => draftStore.reset());

function renderWith(post: PresentApi['post'], interactions: Interactions = empty()): void {
  const present: PresentApi = { post, closed: false, currentRound: 1 };
  act(() =>
    root.render(
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
          <Input block={block} interactions={interactions} />
        </KeyboardProvider>
      </PresentContext.Provider>,
    ),
  );
}

const field = (): HTMLInputElement => container.querySelector('input') as HTMLInputElement;

function type(value: string): void {
  const el = field();
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

async function blur(): Promise<void> {
  await act(async () => {
    field().dispatchEvent(new FocusEvent('focusout', { bubbles: true }));
  });
}

async function commit(value: string): Promise<void> {
  type(value);
  await blur();
}

function pendingPost(): { post: PresentApi['post']; settle: (ok: boolean) => Promise<void> } {
  let resolve!: (ok: boolean) => void;
  return {
    post: () =>
      new Promise((r) => {
        resolve = r;
      }),
    settle: (ok) => act(async () => resolve(ok)),
  };
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

describe('Input failed save', () => {
  it('keeps the typed text and un-ticks when a rejected post rolls the value back', async () => {
    const { post, settle } = pendingPost();
    const answered: Interactions = { ...empty(), inputs: { i1: { text: 'Ada', round: 1 } } };

    renderWith(post);
    await commit('Ada');
    renderWith(post, answered);
    expect(container.querySelector('.input-tick')).not.toBeNull();

    renderWith(post);
    expect(field().value).toBe('Ada');
    await settle(false);

    expect(field().value).toBe('Ada');
    expect(container.querySelector('.input-tick')).toBeNull();
  });

  it('keeps newer typing when the pending save is rejected', async () => {
    const { post, settle } = pendingPost();
    renderWith(post);
    await commit('Ada');
    type('Ada Lovelace');
    await settle(false);

    expect(field().value).toBe('Ada Lovelace');
    expect(draftStore.get('i1', 'input')).toBe('Ada Lovelace');
  });

  it('restores the sent text when the field was typed back to it before the save failed', async () => {
    const { post, settle } = pendingPost();
    const answered: Interactions = { ...empty(), inputs: { i1: { text: 'Ada', round: 1 } } };
    renderWith(post);
    await commit('Ada');
    renderWith(post, answered);
    type('Adam');
    type('Ada');
    expect(draftStore.get('i1', 'input')).toBeUndefined();

    renderWith(post);
    await settle(false);

    expect(field().value).toBe('Ada');
  });

  it('clears the draft once a later save lands', async () => {
    let accept = false;
    renderWith(async () => accept);
    await commit('Ada');
    expect(draftStore.get('i1', 'input')).toBe('Ada');

    accept = true;
    await commit('Ada');
    expect(draftStore.get('i1', 'input')).toBeUndefined();
  });

  it('drops the draft when the field is put back to the committed value', async () => {
    renderWith(async () => false);
    await commit('Ada');
    await commit('');
    expect(draftStore.get('i1', 'input')).toBeUndefined();
  });
});

describe('Input unsent text', () => {
  it('holds every keystroke in the draft store, so a remount before any blur shows it', () => {
    renderWith(async () => true);
    type('half a na');
    expect(draftStore.get('i1', 'input')).toBe('half a na');

    act(() => root.unmount());
    root = createRoot(container);
    renderWith(async () => true);

    expect(field().value).toBe('half a na');
  });

  it('keeps newer typing as the draft when the earlier save lands', async () => {
    const { post, settle } = pendingPost();
    renderWith(post);
    await commit('Ada');
    type('Ada Lovelace');
    await settle(true);

    expect(draftStore.get('i1', 'input')).toBe('Ada Lovelace');
    expect(field().value).toBe('Ada Lovelace');
  });

  it('clears the draft as soon as the field reads the committed value again', () => {
    renderWith(async () => true, { ...empty(), inputs: { i1: { text: 'Ada', round: 1 } } });
    type('Adam');
    expect(draftStore.get('i1', 'input')).toBe('Adam');
    type('Ada');
    expect(draftStore.get('i1', 'input')).toBeUndefined();
    expect(field().value).toBe('Ada');
  });
});
