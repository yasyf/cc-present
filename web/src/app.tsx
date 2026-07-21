// The single view. There is no router: the subject is read from /p/<ref>, the
// reduced-state cache is seeded empty and filled by the SSE replay, and every
// block renders off that one cache. Interactions flow through usePostInteraction
// and the FLIP hook animates top-level reorders on block.upserted.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ComponentProps, ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AppShell, ToastStack, useFlip } from '@cc-interact/react';
import { EventStreamProvider, useEventStream } from './stream';
import { usePackToastSink } from './packs/toasts';
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
import { ActiveBlockProvider, useActiveBlock } from './activeBlock';
import { useMediaQuery } from './useMediaQuery';
import { threadFeed } from './threadFeed';
import type { ThreadEntry, ThreadProjection } from './threadFeed';
import { interactionErrorText } from './interactionError';
import { BoardBlocks } from './components/BoardBlocks';
import { RoundGroup } from './components/RoundGroup';
import { DocHeader } from './components/DocHeader';
import type { DocHeaderProps } from './components/DocHeader';
import { SubmitBar } from './components/SubmitBar';
import { FocusDeck } from './components/FocusDeck';
import { WaitingPanel } from './components/WaitingPanel';
import { BoardSkeleton } from './components/BoardSkeleton';
import { ClosedBanner } from './components/ClosedBanner';
import { ConnectError } from './components/ConnectError';
import { SingleBlockView } from './components/SingleBlockView';
import { SidebarPanel } from './components/SidebarPanel';
import { CommentsSheet } from './components/CommentsSheet';
import { ThreadHostContext } from './components/threadHost';

// A dead EventSource never retries and never flips connected/caughtUp, so a
// stuck connect gets a visible error after this long.
const CONNECT_TIMEOUT_MS = 8000;

function subjectFromPath(): string | null {
  const match = /^\/p\/(.+)$/.exec(window.location.pathname);
  return match && match[1] ? decodeURIComponent(match[1]) : null;
}

export function App() {
  const subject = subjectFromPath();
  if (!subject) {
    return (
      <div className="empty-state">
        <div className="empty-card crop-frame">
          <p className="empty-lead">No board here yet — start one from your agent session:</p>
          <code className="empty-snippet">cc-present start</code>
        </div>
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
  usePackToastSink(stream.notify);
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

  const [connectTimedOut, setConnectTimedOut] = useState(false);
  useEffect(() => {
    if (stream.caughtUp || hasContent) return;
    const timer = setTimeout(() => setConnectTimedOut(true), CONNECT_TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [stream.caughtUp, hasContent]);

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

  // In focus mode the SubmitBar mounts only at the Review summary the deck reports;
  // reset outside focus-live so a new round never flashes it mid-deck.
  const [focusAtEnd, setFocusAtEnd] = useState(false);
  useEffect(() => {
    if (mode !== 'focus' || phase.kind !== 'live') setFocusAtEnd(false);
  }, [mode, phase.kind]);

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
              onEndChange={setFocusAtEnd}
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
      {phase.kind === 'live' && (mode !== 'focus' || focusAtEnd) && (
        <SubmitBar
          blocks={liveBlocks}
          doc={state.doc}
          interactions={state.interactions}
          subject={subject}
          hasHistory={hasHistory}
          showTally={mode !== 'focus'}
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
        <ActiveBlockProvider>
          <ThreadHostContext.Provider value="rail">
            <PresentShell
              state={state}
              board={board}
              boardReady={stream.caughtUp || hasContent}
              connectTimedOut={connectTimedOut}
              header={{
                doc: state.doc,
                round: currentRound,
                connected: stream.connected,
                peerPresent: stream.peerPresent,
                mode,
                onSetView: setView,
              }}
              notifications={stream.notifications}
              onDismiss={stream.dismiss}
            />
          </ThreadHostContext.Provider>
        </ActiveBlockProvider>
      </KeyboardProvider>
    </PresentContext.Provider>
  );
}

function totalComments(projection: ThreadProjection): number {
  const count = (e: ThreadEntry) => e.feedback.length + e.replies.length;
  return (projection.pinned ? count(projection.pinned) : 0) + projection.feed.reduce((n, e) => n + count(e), 0);
}

// PresentShell renders inside the keyboard/active-block providers so it can read
// the pinned block and the rail breakpoint: at desktop the margin rail sits in a
// two-column grid beside the stage; below 1100px it collapses to the comments
// sheet, opened from the DocHeader trigger. body[data-single] never reaches here.
function PresentShell({
  state,
  board,
  boardReady,
  connectTimedOut,
  header,
  notifications,
  onDismiss,
}: {
  state: PresentState;
  board: ReactNode;
  boardReady: boolean;
  connectTimedOut: boolean;
  header: Omit<DocHeaderProps, 'commentCount' | 'onOpenComments'>;
  notifications: ComponentProps<typeof ToastStack>['notifications'];
  onDismiss: ComponentProps<typeof ToastStack>['onDismiss'];
}) {
  const active = useActiveBlock();
  const isDesktop = useMediaQuery('(min-width: 1100px)');
  const projection = threadFeed(state, active.activeId);

  const panel = (openComposerOnMount: boolean) => (
    <SidebarPanel
      projection={projection}
      composeEpoch={active.composeEpoch}
      openComposerOnMount={openComposerOnMount}
      onJumped={active.closePanel}
    />
  );

  const main = boardReady ? (
    <div className="board">
      <div className="board-stage">{board}</div>
      {isDesktop && <aside className="margin-rail">{panel(false)}</aside>}
    </div>
  ) : connectTimedOut ? (
    <ConnectError />
  ) : (
    <BoardSkeleton />
  );

  return (
    <>
      <AppShell
        header={
          <DocHeader
            {...header}
            commentCount={!isDesktop ? totalComments(projection) : undefined}
            onOpenComments={!isDesktop ? active.openPanel : undefined}
          />
        }
        main={main}
      />
      {!isDesktop && (
        <CommentsSheet open={active.panelOpen} onClose={active.closePanel}>
          {panel(active.panelCompose)}
        </CommentsSheet>
      )}
      <ToastStack notifications={notifications} onDismiss={onDismiss} />
    </>
  );
}
