import { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { Block, Doc } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { revisionKey } from '../api';
import { unansweredSummary, undecidedKey } from '../submit';
import { submitItems } from '../decide';
import type { Remaining } from '../focus';
import { useKeyboardApi } from '../keyboard';
import { useRevisionSummary } from '../revision';
import { viewedStore } from '../viewed';
import { useInteractivePackTypes } from '../packs/registry';
import { Button } from './Button';

export interface SubmitBarProps {
  // The current round's live blocks; the armed confirm names what is undecided among these.
  blocks: Block[];
  progress: Remaining;
  // The meter is the board's progress rail; the focus deck's dot rail owns per-step progress there.
  showTally: boolean;
  doc: Doc;
  interactions: Interactions;
  subject: string;
  hasHistory: boolean;
}

export function SubmitBar({ blocks, progress, showTally, doc, interactions, subject, hasHistory }: SubmitBarProps) {
  const { post, currentRound } = usePresent();
  const kbd = useKeyboardApi();
  const packInteractive = useInteractivePackTypes();
  const { data: revision } = useQuery<number>({
    queryKey: revisionKey(subject),
    queryFn: () => 0,
    initialData: 0,
    staleTime: Infinity,
  });
  const [armed, setArmed] = useState<{ ids: string; round: number } | null>(null);
  // The submit is no longer optimistic, so the bar stays mounted until the echo
  // advances the round; inFlight bridges that window so a second click can't
  // post twice. A failed post re-enables for retry; a successful one unmounts
  // the bar with the echo.
  const [inFlight, setInFlight] = useState(false);

  const items = submitItems(blocks, interactions, packInteractive);
  const { left, total } = progress;
  const complete = total > 0 && left === 0;
  const unanswered = items.filter((i) => !i.decided);
  const armedKey = undecidedKey(unanswered.map((i) => i.id));

  // The agent's declared revising set surfaces here as a warning: submit stays
  // enabled (human sovereignty), but the reviewer is told work is still in flight.
  const revising = useRevisionSummary();
  const revisingLine =
    revising.revisingCount > 0
      ? `Claude is still revising ${revising.revisingCount} ${revising.revisingCount === 1 ? 'step' : 'steps'}`
      : revising.drafting
        ? 'Claude is still drafting a step'
        : null;

  const label = doc.submit?.label ?? 'Submit';
  // The confirm is derived, never synced: it keys on the exact set of undecided
  // items and the round, so a same-count block swap still derives it false.
  const confirming = armed !== null && armed.ids === armedKey && armed.round === currentRound;

  const hidden = total === 0 && !doc.submit;

  const submit = () => {
    // Invocation-time guard: registration is effect-driven, so a chord can land
    // in the commit-to-cleanup window after the bar hides — the stale handle
    // must no-op.
    if (hidden || inFlight) return;
    if (unanswered.length > 0 && !confirming) {
      setArmed({ ids: armedKey, round: currentRound });
      return;
    }
    setInFlight(true);
    void post({ type: 'submit', revision, viewed: viewedStore.viewed() }).then((ok) => {
      if (!ok) setInFlight(false);
    });
    setArmed(null);
  };

  // The keyboard layer's mod+Enter drives the exact same confirm-aware submit as
  // the button; a latest-value ref keeps the registered handle current without
  // re-registering each render. Registration is gated on visibility so the chord
  // is a no-op exactly when the bar renders nothing.
  const submitRef = useRef(submit);
  submitRef.current = submit;
  // The bar's own onKeyDown fires only with focus inside it, so a body-level Esc
  // routes through the keyboard layer; the handle reports whether it cancelled.
  const cancelConfirmRef = useRef<() => boolean>(() => false);
  cancelConfirmRef.current = () => {
    if (!confirming) return false;
    setArmed(null);
    return true;
  };
  useEffect(() => {
    if (hidden) return;
    kbd.registerSubmit(() => submitRef.current());
    kbd.registerEscape(() => cancelConfirmRef.current());
    return () => {
      kbd.registerSubmit(null);
      kbd.registerEscape(null);
    };
  }, [kbd, hidden]);

  if (hidden) return null;

  return (
    <div
      className="submit-bar"
      onKeyDown={(e) => {
        if (e.key === 'Escape') setArmed(null);
      }}
    >
      <div className="submit-status">
        {hasHistory && <span className="submit-round">Round {currentRound}</span>}
        {total > 0 && showTally && (
          <div
            className="submit-meter"
            role="progressbar"
            aria-label="review progress"
            aria-valuemin={0}
            aria-valuemax={total}
            aria-valuenow={total - left}
          >
            <span className="submit-meter-fill" style={{ inlineSize: `${((total - left) / total) * 100}%` }} />
          </div>
        )}
        {total > 0 && (
          <button
            type="button"
            className={`submit-count${complete ? ' submit-done' : ''}`}
            onClick={() => kbd.jumpNextUndecided()}
          >
            {complete ? 'All answered' : `${total - left} of ${total} answered`}
          </button>
        )}
        {confirming && (
          <span className="submit-warn" role="status">
            {unansweredSummary(items)}
          </span>
        )}
        {revisingLine && (
          <span className="submit-revising" role="status">
            {revisingLine}
          </span>
        )}
      </div>
      {doc.submit?.note && <span className="submit-note">{doc.submit.note}</span>}
      <div className="submit-actions">
        {confirming && (
          <>
            <Button variant="ghost" size="sm" onClick={() => setArmed(null)}>
              Cancel
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setArmed(null);
                kbd.jumpTo(unanswered[0]!.id);
              }}
            >
              Go to first
            </Button>
          </>
        )}
        <Button
          variant="primary"
          size="lg"
          className={`submit-btn${confirming ? ' confirm' : ''}${complete && !confirming ? ' submit-ready' : ''}`}
          disabled={inFlight}
          onClick={submit}
        >
          {inFlight ? 'Submitting…' : confirming ? 'Submit anyway?' : label}
        </Button>
      </div>
    </div>
  );
}
