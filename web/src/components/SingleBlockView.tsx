// The single-block view (contract #7): one block full-bleed at
// /p/<ref>?block=<id>, same SSE + interaction REST, no board chrome. It is what
// the iOS client loads in a WKWebView per pack block; a ResizeObserver reports
// the content height to the native host so the webview sizes to its content.

import { useEffect, useMemo, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ToastStack } from '@cc-interact/react';
import { useEventStream } from '../stream';
import { presentKey, queryClient, usePostInteraction } from '../api';
import { emptyState, topLevelRound } from '../reduce';
import { flatten } from '../decide';
import { KeyboardProvider } from '../keyboard';
import { PresentContext } from '../present';
import { usePackToastSink } from '../packs/toasts';
import type { PresentApi } from '../present';
import type { PresentState } from '../events';
import type { Block } from '../schema';
import { interactionErrorText } from '../interactionError';
import { BlockRenderer } from './BlockRenderer';
import { ClosedBanner } from './ClosedBanner';

interface HeightHandler {
  postMessage: (message: unknown) => void;
}

declare global {
  interface Window {
    webkit?: { messageHandlers?: { ccPresentHeight?: HeightHandler } };
  }
}

// A block id absent from the live doc may still belong to a closed round: the
// iOS client loads this view for a historical pack block whose id a later
// doc.replaced dropped, and its agent replies must still render somewhere. Fall
// back to the frozen blocks of closed rounds, newest first.
function findBlock(state: PresentState, blockId: string): Block | undefined {
  const live = flatten(state.doc.blocks).find((b) => b.id === blockId);
  if (live) return live;
  for (let i = state.rounds.history.length - 1; i >= 0; i--) {
    const frozen = flatten(state.rounds.history[i]!.blocks).find((b) => b.id === blockId);
    if (frozen) return frozen;
  }
  return undefined;
}

export function SingleBlockView({ subject, blockId }: { subject: string; blockId: string }) {
  const stream = useEventStream();
  const { data: state } = useQuery<PresentState>({
    queryKey: presentKey(subject),
    queryFn: () => queryClient.getQueryData<PresentState>(presentKey(subject)) ?? emptyState(),
    initialData: emptyState,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const mutation = usePostInteraction(subject, (_err, interaction) =>
    stream.notify({ kind: 'error', text: interactionErrorText(interaction) }),
  );

  const block = findBlock(state, blockId);
  const round = topLevelRound(state, blockId);
  const realClosed = state.interactions.closed.value;
  // A superseded round is read-only but not closed: roundOver disables the block,
  // while ClosedBanner and context.closed stay reserved for a closed artifact.
  const roundOver = round !== undefined && round !== state.rounds.current;
  const currentRound = state.rounds.current;

  const api = useMemo<PresentApi>(
    () => ({
      post: (interaction) => mutation.mutateAsync(interaction).then(() => true, () => false),
      closed: realClosed,
      currentRound,
      roundOver,
    }),
    [mutation, realClosed, currentRound, roundOver],
  );

  usePackToastSink(stream.notify);

  const rootRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    document.body.dataset.single = '';
    return () => {
      delete document.body.dataset.single;
    };
  }, []);
  useEffect(() => {
    const el = rootRef.current;
    const handler = window.webkit?.messageHandlers?.ccPresentHeight;
    if (!el || !handler) return;
    const report = () => handler.postMessage({ type: 'height', px: Math.ceil(el.getBoundingClientRect().height) });
    report();
    const observer = new ResizeObserver(report);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const content = block ? (
    <BlockRenderer block={block} interactions={state.interactions} />
  ) : stream.caughtUp ? (
    <div className="empty-state">
      No block <code>{blockId}</code>
    </div>
  ) : (
    <div className="skeleton" aria-hidden>
      <div className="skeleton-card" />
    </div>
  );

  return (
    <PresentContext.Provider value={api}>
      <KeyboardProvider
        blocks={block ? [block] : []}
        interactions={state.interactions}
        closed={realClosed || roundOver}
        round={currentRound}
      >
        <div className="single-block" ref={rootRef}>
          {realClosed && <ClosedBanner summary={state.interactions.closed.summary} />}
          {content}
          <ToastStack notifications={stream.notifications} onDismiss={stream.dismiss} />
        </div>
      </KeyboardProvider>
    </PresentContext.Provider>
  );
}
