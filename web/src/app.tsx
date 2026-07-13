// The single view. There is no router: the subject is read from /p/<ref>, the
// reduced-state cache is seeded empty and filled by the SSE replay, and every
// block renders off that one cache. Interactions flow through usePostInteraction
// and the FLIP hook animates top-level reorders on block.upserted.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AppShell, ToastStack, useFlip } from '@cc-interact/react';
import { EventStreamProvider, useEventStream } from './stream';
import { setPackToastSink } from './packs/toasts';
import { SubjectProvider, presentKey, queryClient, usePostInteraction } from './api';
import { emptyState } from './reduce';
import { boardPhase } from './lifecycle';
import { focusSteps } from './focus';
import { loadView, resolveMode, saveView } from './viewmode';
import type { ViewMode } from './viewmode';
import { KeyboardProvider } from './keyboard';
import { ExpandAllProvider, useExpandAll } from './expand';
import { useInteractivePackTypes } from './packs/registry';
import { PresentContext } from './present';
import type { PresentApi } from './present';
import type { PresentState } from './events';
import { interactionErrorText } from './interactionError';
import { BoardBlocks } from './components/BoardBlocks';
import { RoundGroup } from './components/RoundGroup';
import { DocHeader } from './components/DocHeader';
import { SubmitBar } from './components/SubmitBar';
import { FocusDeck } from './components/FocusDeck';
import { WaitingPanel } from './components/WaitingPanel';
import { BoardSkeleton } from './components/BoardSkeleton';
import { ClosedBanner } from './components/ClosedBanner';
import { SingleBlockView } from './components/SingleBlockView';

function subjectFromPath(): string | null {
  const match = /^\/p\/(.+)$/.exec(window.location.pathname);
  return match && match[1] ? decodeURIComponent(match[1]) : null;
}

export function App() {
  const subject = subjectFromPath();
  if (!subject) {
    return (
      <div className="empty-state">
        Open a presentation link: <code>/p/&lt;ref&gt;</code>
      </div>
    );
  }
  const blockId = new URLSearchParams(window.location.search).get('block');
  return (
    <SubjectProvider value={{ subject, scope: undefined }}>
      <EventStreamProvider subject={subject}>
        <ExpandAllProvider>
          {blockId ? (
            <SingleBlockView subject={subject} blockId={blockId} />
          ) : (
            <PresentView subject={subject} />
          )}
        </ExpandAllProvider>
      </EventStreamProvider>
    </SubjectProvider>
  );
}

function PresentView({ subject }: { subject: string }) {
  const stream = useEventStream();
  useEffect(() => setPackToastSink(stream.notify), [stream.notify]);
  const { data: state } = useQuery<PresentState>({
    queryKey: presentKey(subject),
    // No get-document endpoint exists; the cache is built by the SSE replay. A
    // refetch just returns the current reduced state so it never wipes it.
    queryFn: () => queryClient.getQueryData<PresentState>(presentKey(subject)) ?? emptyState(),
    initialData: emptyState,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const mutation = usePostInteraction(subject, (_err, interaction) =>
    stream.notify({ kind: 'error', text: interactionErrorText(interaction) }),
  );

  const phase = boardPhase(state);
  const closed = state.interactions.closed.value;
  const currentRound = state.rounds.current;
  const hasHistory = state.rounds.history.length > 0;
  const liveBlocks = phase.kind === 'waiting' ? [] : phase.blocks;
  // Content the cache already holds must survive a stream-provider remount (HMR,
  // StrictMode, subject remount) that resets caughtUp: the skeleton shows only
  // for a genuinely empty board that has not caught up yet.
  const hasContent = state.doc.blocks.length > 0 || hasHistory || closed;

  const api = useMemo<PresentApi>(
    () => ({
      post: (interaction) => mutation.mutateAsync(interaction).then(() => true, () => false),
      closed,
      currentRound,
    }),
    [mutation, closed, currentRound],
  );

  const packInteractive = useInteractivePackTypes();
  const steps = useMemo(() => focusSteps(liveBlocks, packInteractive), [liveBlocks, packInteractive]);
  const [override, setOverride] = useState<ViewMode | null>(() => loadView(subject));
  const mode = resolveMode(state.doc.presentation, override, steps);
  const setView = useCallback(
    (m: ViewMode) => {
      setOverride(m);
      saveView(subject, m);
    },
    [subject],
  );
  const toggleView = useCallback(() => setView(mode === 'focus' ? 'board' : 'focus'), [mode, setView]);
  const expandAll = useExpandAll();

  const listRef = useRef<HTMLDivElement>(null);
  useFlip(listRef);

  const board = (
    <>
      {closed && <ClosedBanner summary={state.interactions.closed.summary} />}
      {mode === 'focus' ? (
        <>
          {hasHistory && (
            <div className="focus-history">
              {state.rounds.history.map((record) => (
                <RoundGroup key={`round-${record.number}`} record={record} interactions={state.interactions} />
              ))}
            </div>
          )}
          {phase.kind === 'waiting' ? (
            <WaitingPanel round={currentRound} lastRound={phase.lastRound} />
          ) : (
            <FocusDeck
              key={currentRound}
              steps={steps}
              interactions={state.interactions}
              round={currentRound}
              closed={closed}
            />
          )}
        </>
      ) : (
        <>
          <div className="spine">
            <div className="blocks" ref={listRef}>
              {state.rounds.history.map((record) => (
                <div
                  className="round-group spine-node"
                  key={`round-${record.number}`}
                  data-flip-key={`round-${record.number}`}
                  data-node={record.number}
                >
                  <RoundGroup record={record} interactions={state.interactions} />
                </div>
              ))}
              {liveBlocks.length > 0 && (hasHistory || state.rounds.currentTitle) && (
                <div className="round-current-header spine-node spine-current" data-node={currentRound}>
                  Round {currentRound}
                  {state.rounds.currentTitle && ` · ${state.rounds.currentTitle}`}
                </div>
              )}
              <BoardBlocks blocks={liveBlocks} interactions={state.interactions} packInteractive={packInteractive} />
            </div>
          </div>
          {phase.kind === 'waiting' && <WaitingPanel round={currentRound} lastRound={phase.lastRound} />}
        </>
      )}
      {phase.kind === 'live' && (
        <SubmitBar
          blocks={liveBlocks}
          doc={state.doc}
          interactions={state.interactions}
          subject={subject}
          hasHistory={hasHistory}
        />
      )}
    </>
  );

  return (
    <PresentContext.Provider value={api}>
      <KeyboardProvider
        blocks={liveBlocks}
        interactions={state.interactions}
        closed={closed}
        round={currentRound}
        onViewToggle={toggleView}
        onExpandAll={expandAll.toggle}
      >
        <AppShell
          header={
            <DocHeader
              doc={state.doc}
              connected={stream.connected}
              peerPresent={stream.peerPresent}
              mode={mode}
              onSetView={setView}
            />
          }
          main={stream.caughtUp || hasContent ? board : <BoardSkeleton />}
        />
        <ToastStack notifications={stream.notifications} onDismiss={stream.dismiss} />
      </KeyboardProvider>
    </PresentContext.Provider>
  );
}
