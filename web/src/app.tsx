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
import { SubjectProvider, loadSnapshot, presentKey, queryClient, snapshotKey, usePostInteraction } from './api';
import { emptyState } from './reduce';
import { boardPhase } from './lifecycle';
import { focusSteps, remaining } from './focus';
import type { FocusStep } from './focus';
import { loadView, resolveMode, saveView } from './viewmode';
import type { ViewMode } from './viewmode';
import { KeyboardProvider, useKeyboardApi } from './keyboard';
import { ExpandAllProvider, useExpandAll } from './expand';
import { useInteractivePackTypes } from './packs/registry';
import { PresentContext } from './present';
import type { PresentApi } from './present';
import type { PresentState } from './events';
import { ActiveBlockProvider, useActiveBlock, useJump } from './activeBlock';
import { useMediaQuery } from './useMediaQuery';
import { threadFeed } from './threadFeed';
import { useThreadOpener } from './threadOpen';
import { draftStore } from './drafts';
import { revisionStore } from './revision';
import { viewedStore } from './viewed';
import type { ThreadEntry, ThreadProjection } from './threadFeed';
import { interactionErrorText } from './interactionError';
import { BoardBlocks } from './components/BoardBlocks';
import { DocHeader } from './components/DocHeader';
import type { DocHeaderProps } from './components/DocHeader';
import { SubmitBar } from './components/SubmitBar';
import { RefPopover } from './components/RefPopover';
import { FocusDeck } from './components/FocusDeck';
import { WaitingPanel } from './components/WaitingPanel';
import { BoardSkeleton } from './components/BoardSkeleton';
import { ClosedBanner } from './components/ClosedBanner';
import { ConnectError } from './components/ConnectError';
import { SingleBlockView } from './components/SingleBlockView';
import { SidebarPanel } from './components/SidebarPanel';
import { CommentsSheet } from './components/CommentsSheet';
import { MarginRail } from './components/MarginRail';
import { useRailOpen } from './railOpen';
import { anchorScroll } from './scrollAnchor';
import { ThreadHostContext } from './components/threadHost';
import { FocusLeadContext } from './components/focusStep';
import type { FocusLeadValue } from './components/focusStep';

// A dead EventSource never retries and never flips connected/caughtUp, so a
// stuck connect gets a visible error after this long.
const CONNECT_TIMEOUT_MS = 8000;
const ANCHOR_MS = 3000;

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
  draftStore.scope(subject);
  const params = new URLSearchParams(window.location.search);
  const blockId = params.get('block');
  const row = params.has('row') ? Number(params.get('row')) : undefined;
  return (
    <SubjectProvider value={{ subject, scope: undefined }}>
      <SnapshotGate subject={subject}>
        <EventStreamProvider subject={subject}>
          <ExpandAllProvider>
            {blockId ? (
              <SingleBlockView subject={subject} blockId={blockId} row={row} />
            ) : (
              <PresentView subject={subject} />
            )}
            <RefPopover />
          </ExpandAllProvider>
        </EventStreamProvider>
      </SnapshotGate>
    </SubjectProvider>
  );
}

function SnapshotGate({ subject, children }: { subject: string; children: ReactNode }) {
  const snapshot = useQuery({
    queryKey: snapshotKey(subject),
    queryFn: () => loadSnapshot(subject),
    staleTime: Infinity,
    gcTime: Infinity,
  });
  if (snapshot.isError) return <ConnectError />;
  if (snapshot.isPending) return <BoardSkeleton />;
  return children;
}

function PresentView({ subject }: { subject: string }) {
  const stream = useEventStream();
  usePackToastSink(stream.notify);
  const { data: state } = useQuery<PresentState>({
    queryKey: presentKey(subject),
    // SnapshotGate seeds the cache from /api/state and the stream patches it. A
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
  // Content the cache already held at mount must survive a stream-provider remount
  // (HMR, StrictMode, subject remount) that resets caughtUp. Content that arrives
  // during this connection's replay waits for caughtUp, so the replay's earlier
  // documents never paint.
  const [heldAtMount] = useState(() => state.doc.blocks.length > 0 || hasHistory || closed);
  const boardReady = stream.caughtUp || heldAtMount;

  const [connectTimedOut, setConnectTimedOut] = useState(false);
  useEffect(() => {
    if (boardReady || stream.connected) return;
    const timer = setTimeout(() => setConnectTimedOut(true), CONNECT_TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [boardReady, stream.connected]);

  useEffect(() => viewedStore.scope(subject, currentRound), [subject, currentRound]);

  const api = useMemo<PresentApi>(
    () => ({
      post: (interaction) => {
        if (interaction.type !== 'submit') viewedStore.mark([interaction.blockId]);
        return mutation.mutateAsync(interaction).then(() => true, () => false);
      },
      closed,
      currentRound,
    }),
    [mutation, closed, currentRound],
  );

  const packInteractive = useInteractivePackTypes();
  const steps = useMemo(() => focusSteps(liveBlocks, packInteractive), [liveBlocks, packInteractive]);
  const progress = remaining(steps, state.interactions, packInteractive);
  const unanswered = phase.kind === 'live' && !closed && progress.left > 0;
  useEffect(() => {
    document.title = `${unanswered ? '● ' : ''}${state.doc.title || 'cc-present'}`;
  }, [unanswered, state.doc.title]);
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

  const [deckAt, setDeckAt] = useState<{ round: number; id: string } | null>(null);
  const deckId = deckAt?.round === currentRound ? deckAt.id : undefined;
  const onDeckChange = useCallback((id: string) => setDeckAt({ round: currentRound, id }), [currentRound]);
  const shownDeckRef = useRef(deckId);
  useEffect(() => {
    const shown = shownDeckRef.current;
    shownDeckRef.current = deckId;
    if (shown !== undefined && shown !== deckId) revealDeck();
  }, [deckId]);
  const deckIndex = deckId === undefined ? 0 : steps.findIndex((s) => s.id === deckId);
  const lead = useMemo<FocusLeadValue | null>(() => {
    const first = steps[0];
    const { intro, stats } = state.doc;
    return first && (intro || stats?.length) ? { stepId: first.id, intro, stats } : null;
  }, [steps, state.doc]);

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
      <ViewEntry mode={mode} steps={steps} deckId={deckId} />
      {closed && <ClosedBanner summary={state.interactions.closed.summary} />}
      {mode === 'focus' ? (
        <>
          {phase.kind === 'waiting' ? (
            <WaitingPanel round={currentRound} lastRound={phase.lastRound} />
          ) : (
            <FocusLeadContext.Provider value={lead}>
              <FocusDeck
                key={currentRound}
                steps={steps}
                interactions={state.interactions}
                round={currentRound}
                closed={closed}
                onEndChange={setFocusAtEnd}
                initialId={deckId}
                onCurrentChange={onDeckChange}
              />
            </FocusLeadContext.Provider>
          )}
        </>
      ) : (
        <>
          <div className="spine">
            <div className="blocks" ref={listRef}>
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
          progress={progress}
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
              boardReady={boardReady}
              connectTimedOut={connectTimedOut}
              header={{
                doc: state.doc,
                round: currentRound,
                connected: stream.connected,
                peerPresent: stream.peerPresent,
                closed,
                mode,
                onSetView: setView,
                step:
                  steps.length > 0 || hasHistory
                    ? {
                        steps,
                        current: mode === 'focus' ? deckIndex : undefined,
                        progress,
                        interactions: state.interactions,
                        packInteractive,
                        history: state.rounds.history,
                      }
                    : undefined,
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

function revealDeck() {
  const deck = document.querySelector('.focus-deck');
  if (!deck) return;
  const headerBottom = document.querySelector('.doc-header')!.getBoundingClientRect().bottom;
  if (deck.getBoundingClientRect().top < headerBottom) deck.scrollIntoView({ block: 'start' });
}

function ViewEntry({ mode, steps, deckId }: { mode: ViewMode; steps: FocusStep[]; deckId: string | undefined }) {
  const api = useKeyboardApi();
  const enteredRef = useRef(mode);
  useEffect(() => {
    if (enteredRef.current === mode) return;
    enteredRef.current = mode;
    if (mode === 'focus') {
      revealDeck();
      document
        .querySelector<HTMLElement>('.focus-card:not([data-exiting]), .focus-summary:not([data-exiting])')
        ?.focus({ preventScroll: true });
      return;
    }
    const step = steps.find((s) => s.id === deckId);
    api.setCursor(step?.primary?.id ?? null);
    if (!step) return;
    const row = document.querySelector<HTMLElement>(`.block-row[data-block-id="${CSS.escape(step.id)}"]`);
    if (!row) return;
    const stop = anchorScroll(row, row.closest<HTMLElement>('.board-stage')!, ANCHOR_MS);
    row.focus({ preventScroll: true });
    return stop;
  }, [api, mode, steps, deckId]);
  return null;
}

function totalComments(projection: ThreadProjection): number {
  const count = (e: ThreadEntry) => e.feedback.length + e.replies.length;
  return (projection.pinned ? count(projection.pinned) : 0) + projection.feed.reduce((n, e) => n + count(e), 0);
}

// PresentShell renders inside the keyboard/active-block providers so it can read
// the pinned block and the rail breakpoint: at desktop the margin rail floats over
// the board as a fixed overlay (collapsed to a strip, expanded by hover / focus /
// pin via useRailOpen); below 1100px it collapses to the comments sheet, opened from
// the DocHeader trigger. body[data-single] never reaches here.
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
  const { setPinnedOpen } = active;
  const jump = useJump();
  useThreadOpener((entry) => {
    jump(entry);
    setPinnedOpen(true);
  });
  const isDesktop = useMediaQuery('(min-width: 1100px)');
  const projection = threadFeed(state, active.activeId);
  const rail = useRailOpen({
    pinnedOpen: active.pinnedOpen,
    composing: active.composing,
    onDismiss: () => setPinnedOpen(false),
  });

  const threadShowing = isDesktop ? rail.open : active.panelOpen;
  const pinnedId = projection.pinned?.blockId;
  const pinnedReplies = projection.pinned?.replies.length;
  useEffect(() => {
    if (threadShowing && pinnedId !== undefined) revisionStore.markRepliesRead(pinnedId);
  }, [threadShowing, pinnedId, pinnedReplies]);

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
      {isDesktop && (
        <MarginRail
          open={rail.open}
          projection={projection}
          activeId={projection.pinned?.blockId ?? null}
          total={totalComments(projection)}
          onToggle={() => setPinnedOpen(!active.pinnedOpen)}
          railRef={rail.ref}
        >
          {panel(false)}
        </MarginRail>
      )}
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
          boardReady && (
            <DocHeader
              {...header}
              commentCount={!isDesktop ? totalComments(projection) : undefined}
              onOpenComments={!isDesktop ? active.openPanel : undefined}
            />
          )
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
