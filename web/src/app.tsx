// The single view. There is no router: the subject is read from /p/<ref>, the
// reduced-state cache is seeded empty and filled by the SSE replay, and every
// block renders off that one cache. Interactions flow through usePostInteraction
// and the FLIP hook animates top-level reorders on block.upserted.

import { useMemo, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AppShell, NotificationsBar, useFlip } from '@cc-interact/react';
import { EventStreamProvider, useEventStream } from './stream';
import { SubjectProvider, presentKey, queryClient, usePostInteraction } from './api';
import { emptyState } from './reduce';
import { PresentContext } from './present';
import type { PresentApi } from './present';
import type { PresentState } from './events';
import { BlockRenderer } from './components/BlockRenderer';
import { DocHeader } from './components/DocHeader';
import { SubmitBar } from './components/SubmitBar';
import { ClosedBanner } from './components/ClosedBanner';

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
  return (
    <SubjectProvider value={{ subject, scope: undefined }}>
      <EventStreamProvider subject={subject}>
        <PresentView subject={subject} />
      </EventStreamProvider>
    </SubjectProvider>
  );
}

function PresentView({ subject }: { subject: string }) {
  const stream = useEventStream();
  const { data: state } = useQuery<PresentState>({
    queryKey: presentKey(subject),
    // No get-document endpoint exists; the cache is built by the SSE replay. A
    // refetch just returns the current reduced state so it never wipes it.
    queryFn: () => queryClient.getQueryData<PresentState>(presentKey(subject)) ?? emptyState(),
    initialData: emptyState,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const mutation = usePostInteraction(subject);
  const closed = state.interactions.closed.value;
  const api = useMemo<PresentApi>(
    () => ({ post: (interaction) => mutation.mutate(interaction), closed }),
    [mutation, closed],
  );

  const listRef = useRef<HTMLDivElement>(null);
  useFlip(listRef);

  return (
    <PresentContext.Provider value={api}>
      <AppShell
        header={
          <DocHeader doc={state.doc} connected={stream.connected} peerPresent={stream.peerPresent} />
        }
        notifications={
          <NotificationsBar
            connected={stream.connected}
            notifications={stream.notifications}
            onDismiss={stream.dismiss}
          />
        }
        main={
          <>
            {closed && <ClosedBanner summary={state.interactions.closed.summary} />}
            <div className="blocks" ref={listRef}>
              {state.doc.blocks.map((block) => (
                <div className="block-row" key={block.id} data-flip-key={block.id}>
                  <BlockRenderer block={block} interactions={state.interactions} />
                </div>
              ))}
            </div>
            <SubmitBar doc={state.doc} interactions={state.interactions} subject={subject} />
          </>
        }
      />
    </PresentContext.Provider>
  );
}
