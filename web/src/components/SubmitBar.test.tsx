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
import { focusSteps, remaining } from '../focus';
import { revisionStore } from '../revision';
import { viewedStore } from '../viewed';
import type { Interaction, Interactions, Verdict, WireFrame } from '../events';
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
  post = async () => true,
}: {
  blocks: Block[];
  interactions: Interactions;
  showTally?: boolean;
  post?: PresentApi['post'];
}) {
  const present: PresentApi = { post, closed: false, currentRound: 1 };
  return (
    <QueryClientProvider client={new QueryClient()}>
      <PresentContext.Provider value={present}>
        <KeyboardProvider blocks={blocks} interactions={interactions} closed={false} round={1}>
          <SubmitBar
            blocks={blocks}
            progress={remaining(focusSteps(blocks, new Set()), interactions, new Set())}
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
  viewedStore.reset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  revisionStore.reset();
  viewedStore.reset();
});

function render(props: {
  blocks: Block[];
  interactions: Interactions;
  showTally?: boolean;
  post?: PresentApi['post'];
}): void {
  act(() => root.render(<Bar {...props} />));
}

const card = (id: string, children: Block[]): Block => ({ id, type: 'card', children: children as never });
const count = () => container.querySelector('.submit-count')?.textContent;
const meter = () => container.querySelector('.submit-meter') as HTMLDivElement | null;

describe('SubmitBar progress', () => {
  it('hides only the meter when the focus deck owns progress', () => {
    const blocks = [approval('a1')];
    render({ blocks, interactions: empty(), showTally: false });
    expect(meter()).toBeNull();
    expect(count()).toBe('0 of 1 answered');
    render({ blocks, interactions: empty(), showTally: true });
    expect(meter()).not.toBeNull();
  });

  it('fills the meter by answered steps', () => {
    const blocks = [approval('a1'), approval('a2'), choice('c1'), approval('a3')];
    render({
      blocks,
      interactions: withState({
        decisions: decisions({ a1: 'approved', a2: 'rejected' }),
        choices: { c1: { optionIds: ['o1'] } },
      }),
    });
    expect(count()).toBe('3 of 4 answered');
    expect(meter()!.getAttribute('aria-valuenow')).toBe('3');
    expect(meter()!.getAttribute('aria-valuemax')).toBe('4');
    expect((meter()!.firstElementChild as HTMLElement).style.inlineSize).toBe('75%');
  });

  it('counts a card as one step, answered only once every decidable in it is', () => {
    const blocks = [card('k1', [approval('k1a'), approval('k1b')]), { id: 'm1', type: 'markdown', md: 'note' } as Block];
    render({ blocks, interactions: withState({ decisions: decisions({ k1a: 'approved' }) }) });
    expect(count()).toBe('0 of 1 answered');
    render({ blocks, interactions: withState({ decisions: decisions({ k1a: 'approved', k1b: 'approved' }) }) });
    expect(count()).toBe('All answered');
  });

  it('marks completion exactly when every step is answered', () => {
    const blocks = [approval('a1'), choice('c1')];
    render({ blocks, interactions: withState({ decisions: decisions({ a1: 'approved' }) }) });
    expect(container.querySelector('.submit-btn.submit-ready')).toBeNull();
    render({
      blocks,
      interactions: withState({ decisions: decisions({ a1: 'approved' }), choices: { c1: { optionIds: ['o1'] } } }),
    });
    expect(container.querySelector('.submit-btn.submit-ready')).not.toBeNull();
    expect(count()).toBe('All answered');
  });

  it('counts what is answered, never treating a recommended option as answered', () => {
    const recommended: Choice = { id: 'c1', type: 'choice', options: [{ id: 'o1', label: 'one', recommended: true }] };
    const blocks = [approval('a1'), recommended];
    render({ blocks, interactions: empty() });
    expect(count()).toBe('0 of 2 answered');
    render({ blocks, interactions: withState({ decisions: decisions({ a1: 'approved' }) }) });
    expect(count()).toBe('1 of 2 answered');
    render({
      blocks,
      interactions: withState({ decisions: decisions({ a1: 'approved' }), choices: { c1: { optionIds: ['o1'] } } }),
    });
    expect(count()).toBe('All answered');
  });
});

describe('SubmitBar submit payload', () => {
  it('carries the blocks the human opened this round', () => {
    const posted: Interaction[] = [];
    const post = async (interaction: Interaction) => {
      posted.push(interaction);
      return true;
    };
    viewedStore.mark(['a1', 'm1']);
    render({ blocks: [approval('a1')], interactions: withState({ decisions: decisions({ a1: 'approved' }) }), post });
    act(() => (container.querySelector('.submit-btn') as HTMLButtonElement).click());
    expect(posted).toEqual([{ type: 'submit', revision: 0, viewed: ['a1', 'm1'] }]);
  });

  it('reads Submitting… while the post awaits its echo', () => {
    render({ blocks: [approval('a1')], interactions: withState({ decisions: decisions({ a1: 'approved' }) }) });
    const btn = () => container.querySelector('.submit-btn') as HTMLButtonElement;
    act(() => btn().click());
    expect(btn().textContent).toBe('Submitting…');
    expect(btn().disabled).toBe(true);
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
    expect(container.querySelector('.submit-warn')?.textContent).toBe('1 approval unanswered');
    act(() => (container.querySelector('.submit-actions .btn-ghost') as HTMLButtonElement).click());
    expect(btn().textContent).toBe('Submit');
    expect(container.querySelector('.submit-warn')).toBeNull();
  });

  it('arms on a board whose only unanswered item is a choice', () => {
    const posted: Interaction[] = [];
    const post = async (interaction: Interaction) => {
      posted.push(interaction);
      return true;
    };
    render({ blocks: [choice('c1')], interactions: empty(), post });
    const btn = () => container.querySelector('.submit-btn') as HTMLButtonElement;
    act(() => btn().click());
    expect(btn().textContent).toBe('Submit anyway?');
    expect(container.querySelector('.submit-warn')?.textContent).toBe('1 choice unanswered');
    expect(posted).toEqual([]);
  });

  it('names every unanswered kind in the warning', () => {
    const blocks = [approval('a1'), choice('c1'), approval('a2'), approval('a3')];
    render({ blocks, interactions: withState({ decisions: decisions({ a3: 'approved' }) }) });
    act(() => (container.querySelector('.submit-btn') as HTMLButtonElement).click());
    expect(container.querySelector('.submit-warn')?.textContent).toBe('2 approvals, 1 choice unanswered');
  });

  it('jumps to the first unanswered item from Go to first and disarms', () => {
    const blocks = [approval('a1'), choice('c1'), approval('a2')];
    render({ blocks, interactions: withState({ decisions: decisions({ a1: 'approved' }) }) });
    const btn = () => container.querySelector('.submit-btn') as HTMLButtonElement;
    act(() => btn().click());
    const ghosts = [...container.querySelectorAll('.submit-actions .btn-ghost')] as HTMLButtonElement[];
    expect(ghosts.map((b) => b.textContent)).toEqual(['Cancel', 'Go to first']);
    act(() => ghosts[1]!.click());
    expect(container.querySelector('.sr-only[role="status"]')?.textContent).toBe('Item 2 of 3');
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
