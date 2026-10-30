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

function render(interactions: Interactions, overrides: Partial<PresentApi> = {}, shown: PRGroupsBlock = block): unknown[] {
  const posted: unknown[] = [];
  const present: PresentApi = {
    post: async (ev) => {
      posted.push(ev);
      return true;
    },
    closed: false,
    currentRound: 1,
    ...overrides,
  };
  act(() =>
    root.render(
      <QueryClientProvider client={new QueryClient()}>
        <SubjectProvider value={{ subject: 'board--abcd', scope: undefined }}>
          <PresentContext.Provider value={present}>
            <KeyboardProvider blocks={[shown]} interactions={interactions} closed={false} round={1}>
              <PRGroups block={shown} interactions={interactions} />
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
  it('names the stack the row tap also decides', () => {
    const stacked = { ...row(8, 'abc1234'), stack: [{ number: 6, head: '1112223aaa' }, { number: 7, head: '4445556' }] };
    render(withVerdicts({}), {}, { ...block, groups: [{ ...block.groups[0]!, prs: [stacked] }] });
    expect(rowEl(8).querySelector('.prg-stack')?.textContent).toBe('One tap also decides #6 at 1112223, #7 at 4445556');
  });

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

  it('moves a row answered at its head out of the tap list into the collapsed answered list', () => {
    render(withVerdicts({ pg: { '8': { verdict: 'approved', head: 'abc1234', at: '2026-10-26T20:03:00Z' } } }));
    const answered = container.querySelector<HTMLDetailsElement>('details.prg-answered')!;
    expect(answered.open).toBe(false);
    expect(answered.querySelector('summary')!.textContent).toBe('1 approved, landing');
    expect(answered.contains(rowEl(8))).toBe(true);
    expect(button(rowEl(8), 'Approve')).toBeUndefined();
    expect(container.querySelectorAll('.prg-rows .prg-row')).toHaveLength(1);
    expect(container.querySelector('.prg-group-count')!.textContent).toBe('1 waiting on you');
  });

  it('hides a group with nothing waiting and shows a carried tap with its reason', () => {
    render(
      withVerdicts({
        pg: {
          '8': { verdict: 'approved', head: 'abc1234', at: '2026-10-26T20:03:00Z' },
          '9': { verdict: 'approved', head: 'def5678', at: '2026-10-26T20:03:00Z', carriedFrom: '0001111', carryNote: 'patch-identical replay' },
        },
      }),
    );
    expect(container.querySelector('.prg-group')).toBeNull();
    expect(container.querySelector('.prg-verdicts')).toBeNull();
    expect(rowEl(9).querySelector('.prg-given')!.textContent).toMatch(/^Your tap at 0001111 on .+ carries to def5678: patch-identical replay$/);
  });

  it('applies a key verdict to the row holding focus', () => {
    const posted = render(withVerdicts({}));
    act(() => button(rowEl(9), 'Approve').dispatchEvent(new FocusEvent('focusin', { bubbles: true })));
    act(() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true })));
    expect(posted).toEqual([{ type: 'triage.decided', blockId: 'pg', verdicts: { '9': { verdict: 'approved', head: 'def5678' } } }]);
  });

  it('locks every verdict once its round is over', () => {
    render(withVerdicts({}), { roundOver: true });
    expect(button(rowEl(8), 'Approve').disabled).toBe(true);
    expect(button(rowEl(9), 'Hold').disabled).toBe(true);
    expect(container.textContent).not.toContain('Approve all in this group');
  });

  it('expands a row in place to its full text', () => {
    render(withVerdicts({}));
    expect(rowEl(9).textContent).not.toContain('Full text for 9.');
    act(() => rowEl(9).querySelector<HTMLButtonElement>('.prg-row-head')!.click());
    expect(rowEl(9).textContent).toContain('Full text for 9.');
    expect(rowEl(9).querySelector('.pr-block')).not.toBeNull();
  });
});
