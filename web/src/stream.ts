// The realtime stream: one EventSource per subject, patching the reduced-state
// cache through the shared reducer. The SSE plane delivers each event as a flat,
// self-describing wire frame (the payload with its `type` embedded); this module
// lifts that frame into the reducer's PresentEvent envelope. Toasts fire only for
// live agent activity: the SSE plane replays the whole log on connect, then emits
// a `caught-up` marker at the replay/live boundary. The library tracks that marker
// per connection and toasts only frames past it — until the marker lands the
// highWaterSeq fallback below treats every frame as replay history and stays silent.

import { createEventStream } from '@cc-interact/react';
import { applyEvent } from './reduce';
import { presentKey, revisionKey } from './api';
import { withToken } from './token';
import type { PresentEvent, PresentState, WireFrame } from './events';

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
  url: (subject) => withToken(`/events?session=${encodeURIComponent(subject)}`),
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
      default:
        return null;
    }
  },
  // The pre-marker fallback: a threshold above every seq means no replayed frame
  // clears it, so the whole from-zero replay (and any Last-Event-ID resume) stays
  // silent. Once the caught-up marker arrives the library ignores this and gates
  // on the exact boundary seq, so only the live tail (seq > marker) toasts.
  highWaterSeq: () => Number.POSITIVE_INFINITY,
  peerPresence: (frame) => (frame.type === 'channel.changed' ? frame.connected : null),
  onEvent: (frame, ctx) => {
    if (frame.type === 'doc.replaced') {
      ctx.queryClient.setQueryData(revisionKey(ctx.subject), frame.revision);
    }
  },
});
