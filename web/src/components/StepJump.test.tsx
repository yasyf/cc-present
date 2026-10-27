// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import { SubjectProvider, queryClient } from '../api';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { ActiveBlockProvider, useActiveBlock } from '../activeBlock';
import { focusSteps, remaining } from '../focus';
import { emptyState } from '../reduce';
import type { Interactions, RoundEntry, RoundRecord } from '../events';
import type { Block } from '../schema';
import { StepJump, progressLabel } from './StepJump';

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
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.setAttribute('open', '');
};
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  if (!this.open) return;
  this.removeAttribute('open');
  this.dispatchEvent(new Event('close'));
};

const empty = (): Interactions => emptyState().interactions;

const round = (number: number, blocks: Block[]): RoundRecord => ({
  number,
  blocks,
  decisions: {},
  choices: {},
  inputs: {},
  packs: {},
  feedback: {},
  annotations: {},
  triage: {},
  viewed: {},
});

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  Element.prototype.scrollIntoView = vi.fn();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(() => {
  act(() => root.unmount());
  container.remove();
  queryClient.clear();
  vi.unstubAllGlobals();
});

function RevealButton({ blockId }: { blockId: string }) {
  const { revealBlock } = useActiveBlock();
  return <button type="button" data-testid="reveal" onClick={() => revealBlock(blockId)} />;
}

function render(blocks: Block[], history: RoundEntry[]): void {
  const steps = focusSteps(blocks, new Set());
  const present: PresentApi = { post: async () => true, closed: false, currentRound: history.length + 1 };
  act(() =>
    root.render(
      <QueryClientProvider client={queryClient}>
        <SubjectProvider value={{ subject: 'board', scope: undefined }}>
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={blocks} interactions={empty()} closed={false} round={history.length + 1}>
              <ActiveBlockProvider>
                <StepJump
                  steps={steps}
                  progress={remaining(steps, empty(), new Set())}
                  interactions={empty()}
                  packInteractive={new Set()}
                  history={history}
                />
                <RevealButton blockId="old" />
              </ActiveBlockProvider>
            </KeyboardProvider>
          </PresentContext.Provider>
        </SubjectProvider>
      </QueryClientProvider>,
    ),
  );
}

const click = (selector: string) => act(() => (container.querySelector(selector) as HTMLButtonElement).click());
const drawer = () => container.querySelector('.history-drawer') as HTMLDialogElement | null;

describe('progressLabel', () => {
  const cases: { name: string; left: number; total: number; steps: number; expected: string }[] = [
    { name: 'counts what is left', left: 3, total: 8, steps: 10, expected: '3 of 8 left' },
    { name: 'settles once everything is answered', left: 0, total: 8, steps: 10, expected: 'All answered' },
    { name: 'falls back to steps on a board with nothing to decide', left: 0, total: 0, steps: 2, expected: 'Steps' },
    { name: 'names the history when no step is live', left: 0, total: 0, steps: 0, expected: 'Earlier rounds' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(progressLabel({ left: c.left, total: c.total }, c.steps)).toBe(c.expected);
    });
  }
});

describe('StepJump history drawer', () => {
  const live: Block[] = [{ id: 'a1', type: 'approval', prompt: 'Ship it' }];
  const history = [round(1, [{ id: 'old', type: 'approval', prompt: 'Earlier ask' }])];

  it('opens earlier rounds from the step menu, never above the deck', () => {
    render(live, history);
    expect(container.querySelector('.step-jump')?.textContent).toBe('1 of 1 left');
    expect(container.querySelector('.cc-group')).toBeNull();
    click('.step-jump');
    expect(container.querySelector('.step-jump-history')?.textContent).toBe('Earlier rounds (1)');
    click('.step-jump-history');
    expect(container.querySelector('.step-jump-dialog')!.hasAttribute('open')).toBe(false);
    expect(drawer()!.hasAttribute('open')).toBe(true);
    expect(drawer()!.querySelector('.round-title')?.textContent).toContain('Round 1');
  });

  it('leaves the entry out when there is no history', () => {
    render(live, []);
    click('.step-jump');
    expect(container.querySelector('.step-jump-history')).toBeNull();
    expect(drawer()).toBeNull();
  });

  it('opens the drawer straight from the header when no step is live', () => {
    render([], history);
    expect(container.querySelector('.step-jump')?.textContent).toBe('Earlier rounds');
    click('.step-jump');
    expect(drawer()!.hasAttribute('open')).toBe(true);
  });

  it('fetches a stubbed round once, when the drawer opens', async () => {
    const fetch = vi.fn(async (_url: string) => new Response(JSON.stringify({ history })));
    vi.stubGlobal('fetch', fetch);
    render([], [{ number: 1, outline: [{ id: 'old', type: 'approval', label: 'Earlier ask' }], feedback: {} }]);
    expect(fetch).not.toHaveBeenCalled();
    click('.step-jump');
    await vi.waitFor(() => expect(drawer()!.querySelector('.round-title')?.textContent).toContain('Round 1'));
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0]![0]).toContain('/api/rounds?session=board');
  });

  it('opens the drawer on the revealed round when a thread jumps to an old block', () => {
    render(live, history);
    click('[data-testid="reveal"]');
    expect(drawer()!.hasAttribute('open')).toBe(true);
    const revealed = drawer()!.querySelector('.round-block[data-block-id="old"]')!;
    expect(revealed.scrollIntoView).toHaveBeenCalledTimes(1);
  });
});
