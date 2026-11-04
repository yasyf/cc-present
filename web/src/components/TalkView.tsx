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
import { ClosedBanner } from './ClosedBanner';
import { renderMarkdown } from '../markdown';
import { useSendNote } from './useSendNote';

const STICK_TO_BOTTOM_PX = 80;

const COMPOSER_MAX_PX = 160;

function NoteItem({ note, pending }: { note: Feedback; pending?: boolean }) {
  return (
    <div className={pending ? 'talk-you pending' : 'talk-you'}>
      <div className="talk-bubble">{note.text}</div>
      {pending && <div className="talk-status">Sending</div>}
    </div>
  );
}

type AgentEntry = Exclude<TalkEntry, { kind: 'note' }>;
type Turn = { kind: 'note'; note: Feedback } | { kind: 'agent'; key: string; entries: AgentEntry[] };

function turnsOf(entries: TalkEntry[]): Turn[] {
  const turns: Turn[] = [];
  for (const entry of entries) {
    const last = turns.at(-1);
    if (entry.kind === 'note') turns.push({ kind: 'note', note: entry.note });
    else if (last?.kind === 'agent') last.entries.push(entry);
    else turns.push({ kind: 'agent', key: entry.kind === 'reply' ? entry.reply.id : entry.block.id, entries: [entry] });
  }
  return turns;
}

function AgentTurn({ entries, interactions }: { entries: AgentEntry[]; interactions: Interactions }) {
  return (
    <div className="talk-agent">
      <div className="talk-from">Communicator</div>
      {entries.map((entry) =>
        entry.kind === 'reply' ? (
          <div key={entry.reply.id} className="prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(entry.reply.md) }} />
        ) : (
          <BlockRenderer key={entry.block.id} block={entry.block} interactions={interactions} />
        ),
      )}
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
  const boxRef = useRef<HTMLTextAreaElement>(null);
  const stuck = useRef(true);
  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (el && stuck.current) el.scrollTop = el.scrollHeight;
  }, [entries.length, waiting.length]);

  useLayoutEffect(() => {
    const el = boxRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, COMPOSER_MAX_PX)}px`;
  }, [text]);

  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      if (e.source === window.parent && e.data?.type === 'cc-present.draft') {
        draftStore.set(talkId, 'note', String(e.data.text));
      }
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [talkId]);

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
        {entries.length === 0 && waiting.length === 0 && (
          <div className="talk-empty">
            <div className="talk-empty-title">Ask the communicator</div>
            <div>It answers here, and can reply with tables you filter and open.</div>
          </div>
        )}
        {turnsOf(entries).map((turn) =>
          turn.kind === 'note' ? (
            <NoteItem key={turn.note.id} note={turn.note} />
          ) : (
            <AgentTurn key={turn.key} entries={turn.entries} interactions={interactions} />
          ),
        )}
        {waiting.map((note) => (
          <NoteItem key={note.id} note={note} pending />
        ))}
        {!closed && waiting.length === 0 && entries.at(-1)?.kind === 'note' && (
          <div className="talk-awaiting">
            <span className="talk-dots" aria-hidden="true">
              <i />
              <i />
              <i />
            </span>
            Waiting for the communicator
          </div>
        )}
      </div>
      {!closed && (
        <div className="talk-composer">
          <div className="talk-box">
            <textarea
              ref={boxRef}
              value={text}
              rows={1}
              aria-label="Message the communicator"
              placeholder="Message the communicator"
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
            <button type="button" className="talk-send" aria-label="Send" disabled={!text.trim()} onClick={send}>
              <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M8 13V3M3.5 7.5 8 3l4.5 4.5" />
              </svg>
            </button>
          </div>
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
  const mutation = usePostInteraction(subject, (err, interaction) =>
    stream.notify({ kind: 'error', text: interactionErrorText(interaction, err) }),
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
