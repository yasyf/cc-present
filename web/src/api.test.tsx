// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import type { UseMutationResult } from '@tanstack/react-query';
import { presentKey, queryClient, usePostInteraction } from './api';
import { reduce } from './reduce';
import type { Interaction, PresentState } from './events';
import type { Doc } from './schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const SUBJECT = 's';
const doc: Doc = { version: 1, title: '', blocks: [{ id: 'a1', type: 'approval' }] };
const approve: Interaction = { type: 'decision.created', blockId: 'a1', verdict: 'approved' };

const cached = (): PresentState => queryClient.getQueryData<PresentState>(presentKey(SUBJECT))!;

let container: HTMLDivElement;
let root: Root;
let mutation: UseMutationResult<unknown, Error, Interaction>;

function Poster({ onError }: { onError: (err: Error, interaction: Interaction) => void }) {
  mutation = usePostInteraction(SUBJECT, onError);
  return null;
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  queryClient.setQueryData(
    presentKey(SUBJECT),
    reduce([{ origin: 'agent', type: 'doc.replaced', seq: 1, payload: { schemaVersion: 1, type: 'doc.replaced', doc, revision: 1 } }]),
  );
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  queryClient.clear();
  vi.unstubAllGlobals();
});

describe('usePostInteraction', () => {
  it('shows an answer optimistically and rolls it back when the POST fails', async () => {
    let fail!: (err: Error) => void;
    const fetchStub = vi.fn(
      () =>
        new Promise<Response>((_resolve, reject) => {
          fail = reject;
        }),
    );
    vi.stubGlobal('fetch', fetchStub);
    const onError = vi.fn();
    act(() =>
      root.render(
        <QueryClientProvider client={queryClient}>
          <Poster onError={onError} />
        </QueryClientProvider>,
      ),
    );

    let posted!: Promise<boolean>;
    await act(async () => {
      posted = mutation.mutateAsync(approve).then(
        () => true,
        () => false,
      );
      await vi.waitFor(() => expect(fetchStub).toHaveBeenCalled());
    });
    expect(cached().interactions.decisions.a1).toEqual({ verdict: 'approved' });

    await act(async () => {
      fail(new Error('offline'));
      await posted;
    });

    expect(await posted).toBe(false);
    expect(cached().interactions.decisions.a1).toBeUndefined();
    expect(onError).toHaveBeenCalledWith(expect.any(Error), approve);
  });
});
