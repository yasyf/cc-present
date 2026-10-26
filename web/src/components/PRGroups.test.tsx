// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SubjectProvider } from '../api';
import { draftStore } from '../drafts';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { PRGroups as PRGroupsBlock, PRRow } from '../schema';
import { PRGroups } from './PRGroups';

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
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

const row = (number: number, head: string): PRRow => ({
  number,
  head,
  title: `Title ${number}`,
  change: `Users see change ${number}.`,
  plan: 'Matches the plan.',
  risk: 'Low.',
  review: 'Approved; CI green.',
  blocker: 'Nothing; lands on your approval.',
  md: `Full text for ${number}.`,
});

const block: PRGroupsBlock = {
  id: 'pg',
  type: 'pr-groups',
  repo: 'a/b',
  groups: [{ id: 'g1', title: 'Opening chats', summary: 'Every chat opens.', prs: [row(8, 'abc1234'), row(9, 'def5678')] }],
};

const withVerdicts = (triage: Interactions['triage']): Interactions => ({ ...emptyState().interactions, triage });

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
  draftStore.reset();
});

function render(interactions: Interactions): unknown[] {
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
      <QueryClientProvider client={new QueryClient()}>
        <SubjectProvider value={{ subject: 'board--abcd', scope: undefined }}>
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={[block]} interactions={interactions} closed={false} round={1}>
              <PRGroups block={block} interactions={interactions} />
            </KeyboardProvider>
          </PresentContext.Provider>
        </SubjectProvider>
      </QueryClientProvider>,
    ),
  );
  return posted;
}

function rowEl(n: number): HTMLElement {
  return container.querySelector(`[data-row="${n}"]`)!;
}

function button(scope: HTMLElement, text: string): HTMLButtonElement {
  return [...scope.querySelectorAll('button')].find((b) => b.textContent?.includes(text))!;
}

describe('PRGroups', () => {
  it('shows the group summary and all five facts on each row', () => {
    render(withVerdicts({}));
    expect(container.textContent).toContain('Every chat opens.');
    for (const label of ['What changes', 'Against the plan', 'Risk', 'Review and CI', 'Blocks landing']) {
      expect(rowEl(8).textContent).toContain(label);
    }
    expect(rowEl(8).textContent).toContain('Users see change 8.');
  });

  it('posts an approval pinned to the row head', () => {
    const posted = render(withVerdicts({}));
    act(() => button(rowEl(8), 'Approve').click());
    expect(posted).toEqual([{ type: 'triage.decided', blockId: 'pg', verdicts: { '8': { verdict: 'approved', head: 'abc1234' } } }]);
  });

  it('fans the group approval out to every row', () => {
    const posted = render(withVerdicts({}));
    act(() => button(container, 'Approve all in this group').click());
    expect(posted).toEqual([
      {
        type: 'triage.decided',
        blockId: 'pg',
        verdicts: { '8': { verdict: 'approved', head: 'abc1234' }, '9': { verdict: 'approved', head: 'def5678' } },
      },
    ]);
  });

  it('keeps a verdict from an older head visible and asks for a new tap', () => {
    render(withVerdicts({ pg: { '8': { verdict: 'approved', head: '0001111', at: '2026-10-26T20:03:00Z' } } }));
    expect(rowEl(8).className).toContain('prg-stale');
    expect(rowEl(8).querySelector('.prg-stale')!.textContent).toMatch(/^Approved at 0001111 on .+; new head abc1234 needs your tap\.$/);
    expect(button(rowEl(8), 'Approve').getAttribute('aria-checked')).toBe('false');
  });

  it('expands a row in place to its full text', () => {
    render(withVerdicts({}));
    expect(rowEl(9).textContent).not.toContain('Full text for 9.');
    act(() => rowEl(9).querySelector<HTMLButtonElement>('.prg-row-head')!.click());
    expect(rowEl(9).textContent).toContain('Full text for 9.');
    expect(rowEl(9).querySelector('.pr-block')).not.toBeNull();
  });
});
