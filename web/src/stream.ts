// The realtime stream: one EventSource per subject, patching the reduced-state
// cache through the shared reducer. The SSE plane delivers each event as a flat,
// self-describing wire frame (the payload with its `type` embedded); this module
// lifts that frame into the reducer's PresentEvent envelope. Toasts fire only for
// live agent activity: the replay arrives as a burst on connect, and a macrotask
// after the first frame marks the replay/live boundary so replayed frames stay
// silent.

import { createEventStream } from '@cc-interact/react';
import { applyEvent } from './reduce';
import { presentKey, revisionKey } from './api';
import type { PresentEvent, PresentState, WireFrame } from './events';

// The replay gate. The SSE plane replays the whole log from cursor 0 on every
// connect, so those frames patch state silently; a macrotask after the first
// frame flips replayDone, and every later (live) frame toasts. The library gates
// on the frame's lastEventId against highWaterSeq, so a threshold above every seq
// suppresses during replay and below every seq allows afterward — the seq itself
// is the SSE event id and never rides in the payload to threshold on directly.
let replayDone = false;
let scheduled = false;

// frameToEvent lifts a flat self-describing wire frame into the reducer's
// PresentEvent envelope. applyEvent reads only `type` and `payload`, and the flat
// frame carries the payload fields at top level, so the frame is its own payload;
// origin and seq are not on the wire and the reduction never reads them.
function frameToEvent(frame: WireFrame): PresentEvent {
  return { type: frame.type, payload: frame } as unknown as PresentEvent;
}

export const { EventStreamProvider, useEventStream } = createEventStream<WireFrame, PresentState>({
  queryKey: (subject) => presentKey(subject),
  reduce: (cache, frame) => applyEvent(cache, frameToEvent(frame)),
  toast: (frame) => {
    // Every toasted type is agent-authored; the browser's own human echoes and
    // the system lifecycle frames fall through to null. Origin is not on the
    // wire, so the type discriminant alone gates the toast.
    switch (frame.type) {
      case 'block.upserted':
        return { kind: 'info', text: 'A block was updated' };
      case 'reply.created':
        return { kind: 'info', text: 'The agent replied to your feedback' };
      case 'present.closed':
        return { kind: 'warn', text: 'The agent closed this presentation' };
      default:
        return null;
    }
  },
  highWaterSeq: () => (replayDone ? -1 : Number.MAX_SAFE_INTEGER),
  peerPresence: (frame) => (frame.type === 'channel.changed' ? frame.connected : null),
  onEvent: (frame, ctx) => {
    if (!scheduled) {
      scheduled = true;
      setTimeout(() => {
        replayDone = true;
      }, 0);
    }
    if (frame.type === 'doc.replaced') {
      ctx.queryClient.setQueryData(revisionKey(ctx.subject), frame.revision);
    }
  },
});
