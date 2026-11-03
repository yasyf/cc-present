import { useEffect, useLayoutEffect, useMemo, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ToastStack } from '@cc-interact/react';
import { useEventStream } from '../stream';
import { presentKey, queryClient, usePostInteraction } from '../api';
import { emptyState } from '../reduce';
import { draftStore, useDraft } from '../drafts';
import { KeyboardProvider } from '../keyboard';
import { PresentContext } from '../present';
import { usePackToastSink } from '../packs/toasts';
import { talkEntries } from '../talk';
import type { TalkEntry } from '../talk';
import type { PresentApi } from '../present';
import type { Feedback, Interactions, PresentState } from '../events';
import { interactionErrorText } from '../interactionError';
import { BlockRenderer } from './BlockRenderer';
import { Button } from './Button';
import { ClosedBanner } from './ClosedBanner';
import { ReplyItem } from './ReplyThread';
import { useSendNote } from './useSendNote';

const STICK_TO_BOTTOM_PX = 80;

function NoteItem({ note, pending }: { note: Feedback; pending?: boolean }) {
  return (
    <div className={pending ? 'thread-item feedback-item pending' : 'thread-item feedback-item'}>
      <span className="thread-who">you</span>
      <span className="thread-text">{note.text}</span>
      {pending && <span className="thread-status">sending…</span>}
    </div>
  );
}

export interface TalkPaneProps {
  talkId: string;
  entries: TalkEntry[];
  interactions: Interactions;
  closed: boolean;
}

export function TalkPane({ talkId, entries, interactions, closed }: TalkPaneProps) {
  const { pending, sendNote } = useSendNote(talkId);
  const text = useDraft<string>(talkId, 'note') ?? '';
  const sent = (id: string) => entries.some((e) => e.kind === 'note' && e.note.id === id);
  const waiting = pending.filter((p) => !sent(p.id));

  const scrollRef = useRef<HTMLDivElement>(null);
  const stuck = useRef(true);
  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (el && stuck.current) el.scrollTop = el.scrollHeight;
  }, [entries.length, waiting.length]);

  function send() {
    const body = text.trim();
    if (body) void sendNote(body);
  }

  return (
    <>
      <div
        className="talk-scroll"
        ref={scrollRef}
        onScroll={(e) => {
          const el = e.currentTarget;
          stuck.current = el.scrollHeight - el.scrollTop - el.clientHeight < STICK_TO_BOTTOM_PX;
        }}
      >
        {entries.map((entry) => {
          if (entry.kind === 'note') return <NoteItem key={entry.note.id} note={entry.note} />;
          if (entry.kind === 'reply') return <ReplyItem key={entry.reply.id} reply={entry.reply} />;
          return <BlockRenderer key={entry.block.id} block={entry.block} interactions={interactions} />;
        })}
        {waiting.map((note) => (
          <NoteItem key={note.id} note={note} pending />
        ))}
      </div>
      {!closed && (
        <div className="talk-composer">
          <textarea
            className="field"
            value={text}
            rows={2}
            placeholder="Message the communicator…"
            onChange={(e) => {
              if (e.target.value) draftStore.set(talkId, 'note', e.target.value);
              else draftStore.clear(talkId, 'note');
            }}
            onKeyDown={(e) => {
              if (!e.repeat && !e.shiftKey && e.key === 'Enter') {
                e.preventDefault();
                send();
              }
            }}
          />
          <Button variant="primary" size="sm" onClick={send}>
            Send
          </Button>
        </div>
      )}
    </>
  );
}

export function TalkView({ subject, talkId }: { subject: string; talkId: string }) {
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
  usePackToastSink(stream.notify);

  const closed = state.interactions.closed.value;
  const currentRound = state.rounds.current;
  const api = useMemo<PresentApi>(
    () => ({
      post: (interaction) => mutation.mutateAsync(interaction).then(() => true, () => false),
      closed,
      currentRound,
    }),
    [mutation, closed, currentRound],
  );
  const entries = talkEntries(
    state.doc.blocks,
    talkId,
    state.interactions.feedback[talkId] ?? [],
    state.interactions.replies[talkId] ?? [],
  );

  useEffect(() => {
    document.body.dataset.single = '';
    return () => {
      delete document.body.dataset.single;
    };
  }, []);

  return (
    <PresentContext.Provider value={api}>
      <KeyboardProvider blocks={state.doc.blocks} interactions={state.interactions} closed={closed} round={currentRound}>
        <div className="talk">
          {closed && <ClosedBanner summary={state.interactions.closed.summary} />}
          <TalkPane talkId={talkId} entries={entries} interactions={state.interactions} closed={closed} />
          <ToastStack notifications={stream.notifications} onDismiss={stream.dismiss} />
        </div>
      </KeyboardProvider>
    </PresentContext.Provider>
  );
}
