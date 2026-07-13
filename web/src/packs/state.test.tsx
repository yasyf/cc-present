// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Component, act } from 'react';
import type { ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import {
  PackBlockScopeContext,
  packStateListenerScopesForTest,
  resetPackStateForTest,
  usePackState,
} from './state';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
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

function Block({ id, type = 'pack.demo', children }: { id: string; type?: string; children: ReactNode }) {
  return <PackBlockScopeContext.Provider value={{ id, type }}>{children}</PackBlockScopeContext.Provider>;
}

class Catch extends Component<{ onError: (e: Error) => void; children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError(): { failed: boolean } {
    return { failed: true };
  }
  componentDidCatch(error: Error): void {
    this.props.onError(error);
  }
  render(): ReactNode {
    return this.state.failed ? <span>caught</span> : this.props.children;
  }
}

let renders = 0;
function Counter({ stateKey }: { stateKey: string }) {
  renders++;
  const [n, setN] = usePackState(stateKey, { count: 0 });
  return (
    <button type="button" onClick={() => setN({ count: n.count + 1 })}>
      count={n.count}
    </button>
  );
}

describe('usePackState', () => {
  it('seeds initial lazily and updates without a snapshot-instability loop', () => {
    renders = 0;
    render(
      <Block id="b1">
        <Counter stateKey="k" />
      </Block>,
    );
    expect(container.textContent).toContain('count=0');
    const settled = renders;
    act(() => container.querySelector('button')?.click());
    expect(container.textContent).toContain('count=1');
    // A per-render initial object would loop into a getSnapshot error; a bounded
    // render count on a settled tree proves the seed is stable.
    expect(renders).toBeLessThan(settled + 4);
  });

  it('isolates state per key within a block', () => {
    function TwoKeys() {
      const [a, setA] = usePackState('a', 'A0');
      const [b] = usePackState('b', 'B0');
      return (
        <button type="button" onClick={() => setA('A1')}>
          a={a} b={b}
        </button>
      );
    }
    render(
      <Block id="b1">
        <TwoKeys />
      </Block>,
    );
    expect(container.textContent).toContain('a=A0 b=B0');
    act(() => container.querySelector('button')?.click());
    expect(container.textContent).toContain('a=A1 b=B0');
  });

  it('isolates state per block id under the same key', () => {
    function Named({ label }: { label: string }) {
      const [v, setV] = usePackState('shared', label);
      return (
        <button type="button" onClick={() => setV(`${label}!`)}>
          {v}
        </button>
      );
    }
    render(
      <>
        <Block id="b1">
          <Named label="one" />
        </Block>
        <Block id="b2">
          <Named label="two" />
        </Block>
      </>,
    );
    const buttons = () => container.querySelectorAll('button');
    expect(buttons()[0]?.textContent).toBe('one');
    expect(buttons()[1]?.textContent).toBe('two');
    act(() => (buttons()[0] as HTMLButtonElement).click());
    expect(buttons()[0]?.textContent).toBe('one!');
    expect(buttons()[1]?.textContent).toBe('two');
  });

  it('isolates state per block type under the same id', () => {
    function Named({ label }: { label: string }) {
      const [v, setV] = usePackState('shared', label);
      return (
        <button type="button" onClick={() => setV(`${label}!`)}>
          {v}
        </button>
      );
    }
    render(
      <>
        <Block id="b1" type="pack.alpha">
          <Named label="alpha" />
        </Block>
        <Block id="b1" type="pack.beta">
          <Named label="beta" />
        </Block>
      </>,
    );
    const buttons = () => container.querySelectorAll('button');
    expect(buttons()[0]?.textContent).toBe('alpha');
    expect(buttons()[1]?.textContent).toBe('beta');
    act(() => (buttons()[0] as HTMLButtonElement).click());
    expect(buttons()[0]?.textContent).toBe('alpha!');
    expect(buttons()[1]?.textContent).toBe('beta');
  });

  it('throws when called outside a pack block', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    let caught: Error | null = null;
    function Bare() {
      usePackState('k', 0);
      return <span>ok</span>;
    }
    render(
      <Catch onError={(e) => (caught = e)}>
        <Bare />
      </Catch>,
    );
    expect(container.textContent).toContain('caught');
    expect(caught).not.toBeNull();
    expect((caught as unknown as Error).message).toContain('inside a pack block');
    spy.mockRestore();
  });

  it('survives an unmount and remount for the same block and key', () => {
    function Persist() {
      const [v, setV] = usePackState('p', 0);
      return (
        <button type="button" onClick={() => setV(v + 1)}>
          v={v}
        </button>
      );
    }
    render(
      <Block id="b1">
        <Persist />
      </Block>,
    );
    act(() => container.querySelector('button')?.click());
    act(() => container.querySelector('button')?.click());
    expect(container.textContent).toContain('v=2');

    act(() => root.unmount());
    root = createRoot(container);
    render(
      <Block id="b1">
        <Persist />
      </Block>,
    );
    expect(container.textContent).toContain('v=2');
  });

  it('prunes the listeners map on final unmount while the draft survives', () => {
    function Draft({ next }: { next: string }) {
      const [v, setV] = usePackState('d', 'seed');
      return (
        <button type="button" onClick={() => setV(next)}>
          {v}
        </button>
      );
    }
    render(
      <Block id="b1">
        <Draft next="edited" />
      </Block>,
    );
    act(() => container.querySelector('button')?.click());
    expect(container.textContent).toContain('edited');
    expect(packStateListenerScopesForTest()).toBeGreaterThan(0);

    act(() => root.unmount());
    expect(packStateListenerScopesForTest()).toBe(0);

    // The stored draft outlives the pruned listener, and a fresh subscribe still emits.
    root = createRoot(container);
    render(
      <Block id="b1">
        <Draft next="again" />
      </Block>,
    );
    expect(container.textContent).toContain('edited');
    act(() => container.querySelector('button')?.click());
    expect(container.textContent).toContain('again');
  });
});
