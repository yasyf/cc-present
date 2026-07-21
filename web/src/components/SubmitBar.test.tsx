// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { SubmitBar } from './SubmitBar';
import { emptyState } from '../reduce';
import { revisionStore } from '../revision';
import type { Interactions, Verdict, WireFrame } from '../events';
import type { Approval, Block, Choice, Doc } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
Element.prototype.scrollIntoView = () => {};

const approval = (id: string): Approval => ({ id, type: 'approval' });
const choice = (id: string): Choice => ({ id, type: 'choice', options: [{ id: 'o1', label: 'one' }] });
const empty = (): Interactions => emptyState().interactions;
const withState = (over: Partial<Interactions>): Interactions => ({ ...empty(), ...over });
const decisions = (over: Record<string, Verdict>): Interactions['decisions'] =>
  Object.fromEntries(Object.entries(over).map(([id, verdict]) => [id, { verdict }]));

const doc = (blocks: Block[]): Doc => ({ version: 1, title: '', submit: { label: 'Submit' }, blocks });

function Bar({
  blocks,
  interactions,
  showTally = true,
}: {
  blocks: Block[];
  interactions: Interactions;
  showTally?: boolean;
}) {
  const present: PresentApi = { post: async () => true, closed: false, currentRound: 1 };
  return (
    <QueryClientProvider client={new QueryClient()}>
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={blocks} interactions={interactions} closed={false} round={1}>
          <SubmitBar
            blocks={blocks}
            showTally={showTally}
            doc={doc(blocks)}
            interactions={interactions}
            subject="s"
            hasHistory={false}
          />
        </KeyboardProvider>
      </PresentContext.Provider>
    </QueryClientProvider>
  );
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  revisionStore.reset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  revisionStore.reset();
});

function render(props: { blocks: Block[]; interactions: Interactions; showTally?: boolean }): void {
  act(() => root.render(<Bar {...props} />));
}

const segClasses = (): string[][] =>
  [...container.querySelectorAll('.tally-seg')].map((s) => [...s.classList]);

describe('SubmitBar tally segments', () => {
  it('hides only the per-item rail when the focus deck owns progress', () => {
    const blocks = [approval('a1')];
    render({ blocks, interactions: empty(), showTally: false });
    expect(container.querySelector('.tally-strip')).toBeNull();
    expect(container.querySelector('.submit-count')?.textContent).toBe('0/1 decided');
    render({ blocks, interactions: empty(), showTally: true });
    expect(container.querySelector('.tally-strip')).not.toBeNull();
  });

  it('inks one segment per decidable by its state', () => {
    const blocks = [approval('a1'), approval('a2'), choice('c1'), approval('a3')];
    render({
      blocks,
      interactions: withState({
        decisions: decisions({ a1: 'approved', a2: 'rejected' }),
        choices: { c1: { optionIds: ['o1'] } },
      }),
    });
    const classes = segClasses();
    expect(classes.length).toBe(4);
    expect(classes[0]).toContain('tally-approved');
    expect(classes[1]).toContain('tally-rejected');
    expect(classes[2]).toContain('tally-decided');
    expect(classes[3]).toContain('tally-undecided');
    expect(container.querySelector('.tally-strip.tally-complete')).toBeNull();
  });

  it('keeps each segment a jump button with its positional aria-label', () => {
    const blocks = [approval('a1'), approval('a2')];
    render({ blocks, interactions: empty() });
    const segs = container.querySelectorAll('.tally-seg');
    expect(segs[0]!.getAttribute('aria-label')).toBe('Item 1 of 2, undecided — jump');
    expect(segs[1]!.getAttribute('aria-label')).toBe('Item 2 of 2, undecided — jump');
  });

  it('adds the completion class exactly when every item is decided', () => {
    const blocks = [approval('a1'), choice('c1')];
    render({ blocks, interactions: withState({ decisions: decisions({ a1: 'approved' }) }) });
    expect(container.querySelector('.tally-strip.tally-complete')).toBeNull();
    expect(container.querySelector('.submit-btn.submit-ready')).toBeNull();
    render({
      blocks,
      interactions: withState({ decisions: decisions({ a1: 'approved' }), choices: { c1: { optionIds: ['o1'] } } }),
    });
    expect(container.querySelector('.tally-strip.tally-complete')).not.toBeNull();
    expect(container.querySelector('.submit-btn.submit-ready')).not.toBeNull();
  });
});

describe('SubmitBar armed confirm', () => {
  it('arms a confirm on an undecided approval and cancels it cleanly', () => {
    const blocks = [approval('a1')];
    render({ blocks, interactions: empty() });
    const btn = () => container.querySelector('.submit-btn') as HTMLButtonElement;
    expect(btn().textContent).toBe('Submit');
    act(() => btn().click());
    expect(btn().textContent).toBe('Submit anyway?');
    expect(container.querySelector('.submit-warn')?.textContent).toBe('1 approval still undecided');
    act(() => (container.querySelector('.submit-actions .btn-ghost') as HTMLButtonElement).click());
    expect(btn().textContent).toBe('Submit');
    expect(container.querySelector('.submit-warn')).toBeNull();
  });

  it('warns that the agent is still revising while keeping submit enabled', () => {
    revisionStore.ingest({ type: 'revising.changed', blockIds: ['a1'] } as WireFrame, undefined, {
      blockIds: ['a1'],
    });
    render({ blocks: [approval('a1')], interactions: empty() });
    expect(container.querySelector('.submit-revising')?.textContent).toBe('Claude is still revising 1 step');
    expect((container.querySelector('.submit-btn') as HTMLButtonElement).disabled).toBe(false);
  });

  it('clears the arm on Escape while focus sits on the body', () => {
    const blocks = [approval('a1')];
    render({ blocks, interactions: empty() });
    const btn = () => container.querySelector('.submit-btn') as HTMLButtonElement;
    act(() => btn().click());
    expect(btn().textContent).toBe('Submit anyway?');
    act(() => {
      document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    });
    expect(btn().textContent).toBe('Submit');
    expect(container.querySelector('.submit-warn')).toBeNull();
  });
});
