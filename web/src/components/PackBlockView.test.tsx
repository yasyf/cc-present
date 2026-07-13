// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import type { ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { PresentContext } from '../present';
import type { PresentApi } from '../present';
import { KeyboardProvider } from '../keyboard';
import { PackBlockView } from './PackBlockView';
import { markPacksLoaded, registerPack, resetPacksForTest } from '../packs/registry';
import type { PackComponent, PackComponentProps } from '../packs/registry';
import { resetPackStateForTest, usePackState } from '../packs/state';
import type { PackBlock } from '../schema';
import type { Interactions } from '../events';
import type { PackInfo } from '../packs/manifest';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function emptyInteractions(): Interactions {
  return {
    decisions: {},
    choices: {},
    inputs: {},
    packs: {},
    feedback: {},
    replies: {},
    submitted: { value: false, revision: 0 },
    closed: { value: false },
  };
}

const packBlock = (id: string, type: string, extra: Record<string, unknown> = {}): PackBlock =>
  ({ id, type, ...extra }) as PackBlock;

function api(over: Partial<PresentApi> = {}): PresentApi {
  return { post: async () => true, closed: false, currentRound: 1, ...over };
}

function def(name: string, blocks: { type: string; interactive: boolean }[]): PackInfo {
  return {
    name,
    version: '0',
    description: '',
    bundle: `/packs/${name}/dist/pack.js`,
    blocks: blocks.map((b) => ({ ...b, schema: {} })),
  };
}

function Board({ blocks, interactions, present }: { blocks: PackBlock[]; interactions: Interactions; present: PresentApi }) {
  return (
    <PresentContext.Provider value={present}>
      <KeyboardProvider blocks={blocks} interactions={interactions} closed={present.closed} round={present.currentRound}>
        {blocks.map((b) => (
          <PackBlockView key={b.id} block={b} interactions={interactions} />
        ))}
      </KeyboardProvider>
    </PresentContext.Provider>
  );
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  resetPacksForTest();
  resetPackStateForTest();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function render(ui: ReactNode): void {
  act(() => {
    root.render(ui);
  });
}

describe('PackBlockView placeholder', () => {
  it('renders a labeled placeholder for an unknown pack', () => {
    markPacksLoaded();
    render(<Board blocks={[packBlock('r1', 'ex.rating')]} interactions={emptyInteractions()} present={api()} />);
    expect(container.textContent).toContain('ex.rating');
    expect(container.textContent).toContain('r1');
    expect(container.textContent).toContain('unknown pack');
  });
});

describe('PackBlockView component render', () => {
  const Rating: PackComponent = ({ value, submit, disabled }: PackComponentProps) => (
    <button type="button" disabled={disabled} onClick={() => submit({ value: 5 })}>
      rating={String((value as { value?: number } | undefined)?.value ?? 'none')}
    </button>
  );

  it('renders the pack component with its value and wires submit to a pack.interaction', () => {
    registerPack(def('ex', [{ type: 'ex.rating', interactive: true }]), { rating: Rating });
    markPacksLoaded();
    const post = vi.fn(async () => true);
    const interactions = { ...emptyInteractions(), packs: { r1: { payload: { value: 3 } } } };
    render(<Board blocks={[packBlock('r1', 'ex.rating')]} interactions={interactions} present={api({ post })} />);

    expect(container.textContent).toContain('rating=3');
    const button = container.querySelector('button');
    expect(button?.disabled).toBe(false);
    act(() => button?.click());
    expect(post).toHaveBeenCalledWith({ type: 'pack.interaction', blockId: 'r1', payload: { value: 5 } });
  });

  it('disables the component when the board is closed', () => {
    registerPack(def('ex', [{ type: 'ex.rating', interactive: true }]), { rating: Rating });
    markPacksLoaded();
    render(<Board blocks={[packBlock('r1', 'ex.rating')]} interactions={emptyInteractions()} present={api({ closed: true })} />);
    expect(container.querySelector('button')?.disabled).toBe(true);
  });

  it('disables the component for a non-interactive pack type', () => {
    registerPack(def('ex', [{ type: 'ex.rating', interactive: false }]), { rating: Rating });
    markPacksLoaded();
    render(<Board blocks={[packBlock('r1', 'ex.rating')]} interactions={emptyInteractions()} present={api()} />);
    expect(container.querySelector('button')?.disabled).toBe(true);
  });
});

describe('PackBlockView v2 host surface', () => {
  const Ctx: PackComponent = ({ context }: PackComponentProps) => (
    <span>
      round={context.round} closed={String(context.closed)} over={String(context.roundOver)}
    </span>
  );

  it('passes the decomposed lifecycle context to the pack component', () => {
    registerPack(def('ex', [{ type: 'ex.ctx', interactive: true }]), { ctx: Ctx });
    markPacksLoaded();
    render(
      <Board
        blocks={[packBlock('c1', 'ex.ctx')]}
        interactions={emptyInteractions()}
        present={api({ closed: true, currentRound: 4 })}
      />,
    );
    expect(container.textContent).toContain('round=4');
    expect(container.textContent).toContain('closed=true');
    expect(container.textContent).toContain('over=false');
  });

  it('scopes usePackState to the block so a pack keeps its own draft', () => {
    const Draft: PackComponent = () => {
      const [n, setN] = usePackState('n', 0);
      return (
        <button type="button" onClick={() => setN(n + 1)}>
          draft={n}
        </button>
      );
    };
    registerPack(def('ex', [{ type: 'ex.draft', interactive: true }]), { draft: Draft });
    markPacksLoaded();
    render(<Board blocks={[packBlock('d1', 'ex.draft')]} interactions={emptyInteractions()} present={api()} />);
    expect(container.textContent).toContain('draft=0');
    act(() => container.querySelector('button')?.click());
    expect(container.textContent).toContain('draft=1');
  });
});

describe('PackBlockView error boundary', () => {
  const OkComponent: PackComponent = () => <span>ok-content</span>;
  const Conditional: PackComponent = ({ block }: PackComponentProps) => {
    if ((block as { bad?: boolean }).bad) throw new Error('bad pack data');
    return <span>recovered</span>;
  };

  let errorSpy: ReturnType<typeof vi.spyOn>;
  beforeEach(() => {
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    registerPack(
      def('ex', [
        { type: 'ex.cond', interactive: false },
        { type: 'ex.ok', interactive: false },
      ]),
      { cond: Conditional, ok: OkComponent },
    );
    markPacksLoaded();
  });
  afterEach(() => {
    errorSpy.mockRestore();
  });

  it('contains a crashing component and leaves the sibling rendered', () => {
    render(
      <Board
        blocks={[packBlock('c1', 'ex.cond', { bad: true }), packBlock('o1', 'ex.ok')]}
        interactions={emptyInteractions()}
        present={api()}
      />,
    );
    expect(container.textContent).toContain('crashed while rendering');
    expect(container.textContent).toContain('ok-content');
  });

  it('resets and retries when the block object changes (agent redraft)', () => {
    const badBlock = packBlock('c1', 'ex.cond', { bad: true });
    render(<Board blocks={[badBlock]} interactions={emptyInteractions()} present={api()} />);
    expect(container.textContent).toContain('crashed while rendering');

    const fixedBlock = packBlock('c1', 'ex.cond');
    render(<Board blocks={[fixedBlock]} interactions={emptyInteractions()} present={api()} />);
    expect(container.textContent).toContain('recovered');
    expect(container.textContent).not.toContain('crashed while rendering');
  });
});
