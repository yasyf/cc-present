// The single-block view (contract #7): one block full-bleed at
// /p/<ref>?block=<id>, same SSE + interaction REST, no board chrome. It is what
// the iOS client loads in a WKWebView per pack block, and what a web page embeds
// in an iframe (?row= narrows a pr-groups block to one PR); a ResizeObserver
// reports the content height to the native host or the parent frame so either
// sizes to its content, and each interaction names its block to the host's
// viewed set.

import { useEffect, useMemo, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ToastStack } from '@cc-interact/react';
import { useEventStream } from '../stream';
import { presentKey, queryClient, usePostInteraction, useRoundRecords } from '../api';
import { emptyState, topLevelRound } from '../reduce';
import { flatten } from '../decide';
import { narrowToRow } from '../prgroups';
import { KeyboardProvider } from '../keyboard';
import { PresentContext } from '../present';
import { usePackToastSink } from '../packs/toasts';
import type { PresentApi } from '../present';
import type { PresentState, RoundRecord } from '../events';
import type { Block } from '../schema';
import { interactionErrorText } from '../interactionError';
import { BlockRenderer } from './BlockRenderer';
import { ClosedBanner } from './ClosedBanner';

interface NativeHandler {
  postMessage: (message: unknown) => void;
}

declare global {
  interface Window {
    webkit?: { messageHandlers?: { ccPresentHeight?: NativeHandler; ccPresentInteraction?: NativeHandler } };
  }
}

// findIn resolves an id against a block set plus every option visual: a visual is a
// real doc-unique block the iOS client addresses in this view, so the walk descends
// into choice options as well as card children.
function findIn(blocks: Block[], blockId: string): Block | undefined {
  for (const b of flatten(blocks)) {
    if (b.id === blockId) return b;
    if (b.type === 'choice') {
      for (const o of b.options) {
        if (o.visual?.id === blockId) return o.visual;
      }
    }
  }
  return undefined;
}

// A block id absent from the live doc may still belong to a closed round: the
// iOS client loads this view for a historical pack block whose id a later
// doc.replaced dropped, and its agent replies must still render somewhere. Fall
// back to the frozen blocks of closed rounds, newest first.
function findFrozen(history: RoundRecord[], blockId: string): Block | undefined {
  for (let i = history.length - 1; i >= 0; i--) {
    const frozen = findIn(history[i]!.blocks, blockId);
    if (frozen) return frozen;
  }
  return undefined;
}

export function SingleBlockView({ subject, blockId, row }: { subject: string; blockId: string; row?: number }) {
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

  const live = findIn(state.doc.blocks, blockId);
  const frozen = useRoundRecords(subject, state.rounds.history, live === undefined);
  const found = live ?? findFrozen(frozen.records ?? [], blockId);
  const block = found?.type === 'pr-groups' && row !== undefined ? narrowToRow(found, row) : found;
  const round = topLevelRound(state, blockId);
  const realClosed = state.interactions.closed.value;
  // A superseded round is read-only but not closed: roundOver disables the block,
  // while ClosedBanner and context.closed stay reserved for a closed artifact.
  const roundOver = round !== undefined && round !== state.rounds.current;
  const currentRound = state.rounds.current;

  const api = useMemo<PresentApi>(
    () => ({
      post: (interaction) => {
        if (interaction.type !== 'submit') {
          window.webkit?.messageHandlers?.ccPresentInteraction?.postMessage({ type: 'interaction', blockId: interaction.blockId });
        }
        return mutation.mutateAsync(interaction).then(() => true, () => false);
      },
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
    const native = window.webkit?.messageHandlers?.ccPresentHeight;
    const framed = window.parent !== window;
    if (!el || (!native && !framed)) return;
    const report = () => {
      const px = Math.ceil(el.getBoundingClientRect().height);
      native?.postMessage({ type: 'height', px });
      if (framed) window.parent.postMessage({ type: 'cc-present.height', blockId, row, px }, '*');
    };
    report();
    const observer = new ResizeObserver(report);
    observer.observe(el);
    return () => observer.disconnect();
  }, [blockId, row]);

  const content = block ? (
    <BlockRenderer block={block} interactions={state.interactions} />
  ) : stream.caughtUp && (frozen.records !== undefined || frozen.failed) ? (
    <div className="empty-state">
      {row === undefined ? 'No block' : `No pull request #${row} in`} <code>{blockId}</code>
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
