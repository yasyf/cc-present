// The realtime stream: one EventSource per subject, patching the reduced-state
// cache through the shared reducer. The SSE plane delivers each event as a flat,
// self-describing wire frame (the payload with its `type` embedded); this module
// lifts that frame into the reducer's PresentEvent envelope. Toasts fire only for
// live agent activity: the SSE plane replays the whole log on connect, then emits
// a `caught-up` marker at the replay/live boundary. The library tracks that marker
// per connection and toasts only frames past it — until the marker lands the
// highWaterSeq fallback below treats every frame as replay history and stays silent.

import { createEventStream } from '@cc-interact/react';
import type { StreamToast } from '@cc-interact/react';
import { applyEvent } from './reduce';
import { presentKey, revisionKey } from './api';
import { markdownText } from './markdown';
import { getInteractivePackTypes } from './packs/registry';
import { revisionStore } from './revision';
import { blockLabel, threadFeed } from './threadFeed';
import { hasThreadOpener, openThread } from './threadOpen';
import { withToken } from './token';
import type { PresentEvent, PresentState, WireFrame } from './events';

// TOAST_TEXT is the toast copy per wire type; null for frames that never toast
// (human echoes, channel presence). `satisfies` keeps it exhaustive. A reply
// shows this copy only when no thread owns its block.
export const TOAST_TEXT = {
  'doc.replaced': 'The board was redrafted',
  'block.upserted': 'A block was updated',
  'block.removed': 'A block was removed',
  'reply.created': 'The agent replied',
  'round.started': 'A new round started',
  // Per-step revision markers replace toast noise; the announcement never toasts.
  'revising.changed': null,
  'present.closed': 'The session closed',
  'decision.created': null,
  'choice.selected': null,
  'feedback.created': null,
  'annotation.created': null,
  'annotation.removed': null,
  'triage.decided': null,
  'input.submitted': null,
  'pack.interaction': null,
  submit: null,
  'channel.changed': null,
} satisfies Record<WireFrame['type'], string | null>;

// inFocusMode is true while the focus deck is mounted — its `.focus-deck` root
// exists only in focus mode's live phase; board mode has no such node.
export function inFocusMode(): boolean {
  return typeof document !== 'undefined' && document.querySelector('.focus-deck') !== null;
}

// toastFor is the per-frame toast, or null, from the states before and after the
// frame reduced. Focus mode drops the generic block.upserted toast — the per-step
// revision callout replaces it; board keeps it.
export function toastFor(frame: WireFrame, before: PresentState, after: PresentState): StreamToast | null {
  const text = TOAST_TEXT[frame.type];
  if (text === null) return null;
  if (frame.type === 'block.upserted') return inFocusMode() ? null : named(text, blockLabel(after.doc.blocks, frame.block.id));
  if (frame.type === 'block.removed') return named(text, blockLabel(before.doc.blocks, frame.id));
  if (frame.type !== 'reply.created') return { kind: 'info', text };
  const entry = threadFeed(after, frame.blockId, getInteractivePackTypes()).pinned;
  if (!entry) return { kind: 'info', text };
  const note = entry.feedback.at(-1)?.text;
  return {
    kind: 'info',
    title: entry.label,
    durationMs: 12_000,
    text: `${note ? `You: ${note}\n` : ''}Agent: ${markdownText(frame.md)}`,
    onClick: hasThreadOpener() ? () => openThread(entry) : undefined,
  };
}

function named(text: string, title: string | null): StreamToast {
  return title ? { kind: 'info', title, text } : { kind: 'info', text };
}

// frameToEvent lifts a flat self-describing wire frame into the reducer's
// PresentEvent envelope. applyEvent reads only `type` and `payload`, and the flat
// frame carries the payload fields at top level, so the frame is its own payload;
// origin and seq are not on the wire and the reduction never reads them.
function frameToEvent(frame: WireFrame): PresentEvent {
  return { type: frame.type, payload: frame } as unknown as PresentEvent;
}

export const { EventStreamProvider, useEventStream } = createEventStream<WireFrame, PresentState>({
  queryKey: (subject) => presentKey(subject),
  // Mirror the library's default `/events?session=<subject>`, then carry the page
  // token so an off-loopback EventSource authenticates; loopback stays byte-identical.
  url: (subject) => {
    // Called just before each (re)connection, where the library resets its
    // caught-up seq: close the revision store's live gate for the coming replay.
    revisionStore.beginConnection();
    return withToken(`/events?session=${encodeURIComponent(subject)}`);
  },
  reduce: (cache, frame) => applyEvent(cache, frameToEvent(frame)),
  toast: (frame, ctx) =>
    toastFor(frame, ctx.cache!, ctx.queryClient.getQueryData<PresentState>(presentKey(ctx.subject))!),
  // The pre-marker fallback: a threshold above every seq means no replayed frame
  // clears it, so the whole from-zero replay (and any Last-Event-ID resume) stays
  // silent. Once the caught-up marker arrives the library ignores this and gates
  // on the exact boundary seq, so only the live tail (seq > marker) toasts.
  highWaterSeq: () => Number.POSITIVE_INFINITY,
  peerPresence: (frame) => (frame.type === 'channel.changed' ? frame.connected : null),
  // Opens the revision store's live gate at the replay/live boundary, mirroring the
  // toast gate: only frames past this record changed badges.
  onCaughtUp: () => revisionStore.markLive(),
  onEvent: (frame, ctx) => {
    if (frame.type === 'doc.replaced') {
      ctx.queryClient.setQueryData(revisionKey(ctx.subject), frame.revision);
    }
    // ctx.cache is the pre-reduce state (note lift); the reduced revising set the
    // mirror syncs to is read back post-reduce.
    const post = ctx.queryClient.getQueryData<PresentState>(presentKey(ctx.subject));
    revisionStore.ingest(frame, ctx.cache, post?.revising ?? { blockIds: [] });
  },
});
