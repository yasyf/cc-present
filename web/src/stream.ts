// The realtime stream: one EventSource per subject, patching the reduced-state
// cache through the shared reducer. The library defaults the URL to
// /events?session=<subject>, which the same daemon serves, so no override is
// needed. Toasts fire only for live agent activity: the replay arrives as a
// burst on connect, and a macrotask after the first frame snapshots the
// high-water seq as the replay/live boundary so replayed frames stay silent.

import { createEventStream } from '@cc-interact/react';
import { applyEvent } from './reduce';
import { presentKey, revisionKey } from './api';
import type { PresentEvent, PresentState } from './events';

let boundary = Number.MAX_SAFE_INTEGER;
let maxSeq = 0;
let scheduled = false;

export const { EventStreamProvider, useEventStream } = createEventStream<PresentEvent, PresentState>({
  queryKey: (subject) => presentKey(subject),
  reduce: (cache, ev) => applyEvent(cache, ev),
  toast: (ev) => {
    if (ev.origin !== 'agent') return null;
    switch (ev.type) {
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
  highWaterSeq: () => boundary,
  peerPresence: (ev) => (ev.type === 'channel.changed' ? ev.payload.connected : null),
  onEvent: (ev, ctx) => {
    maxSeq = Math.max(maxSeq, ev.seq);
    if (!scheduled) {
      scheduled = true;
      setTimeout(() => {
        boundary = maxSeq;
      }, 0);
    }
    if (ev.type === 'doc.replaced') {
      ctx.queryClient.setQueryData(revisionKey(ctx.subject), ev.payload.revision);
    }
  },
});
