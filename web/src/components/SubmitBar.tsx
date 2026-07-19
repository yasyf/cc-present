import { useEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { Block, Doc } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { revisionKey } from '../api';
import { undecidedKey } from '../submit';
import { submitItems } from '../decide';
import type { SubmitItem } from '../decide';
import { useKeyboardApi } from '../keyboard';
import { useRevisionSummary } from '../revision';
import { useInteractivePackTypes } from '../packs/registry';

export interface SubmitBarProps {
  // The current round's live blocks; the decided/total tally spans only these.
  blocks: Block[];
  // The tally strip is the board's per-item rail; the focus deck's dot rail owns per-step progress there.
  showTally: boolean;
  doc: Doc;
  interactions: Interactions;
  subject: string;
  hasHistory: boolean;
}

export function SubmitBar({ blocks, showTally, doc, interactions, subject, hasHistory }: SubmitBarProps) {
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
  const total = items.length;
  const decided = items.filter((i) => i.decided).length;
  const complete = total > 0 && decided === total;
  // A tally segment's ink: an approval carries its verdict color, any other
  // decided item the pencil, and an undecided one the hollow-hold state.
  const segState = (item: SubmitItem): string => {
    if (!item.decided) return 'undecided';
    if (item.kind === 'approval') {
      return interactions.decisions[item.id]?.verdict === 'rejected' ? 'rejected' : 'approved';
    }
    return 'decided';
  };
  const undecidedApprovalIds = items.filter((i) => i.kind === 'approval' && !i.decided).map((i) => i.id);
  const undecidedApprovals = undecidedApprovalIds.length;
  const armedKey = undecidedKey(undecidedApprovalIds);

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
  // approvals (not just their count) and the round, so a same-round block swap
  // that preserves the count still derives it false — no stale "Submit anyway?".
  const confirming = armed !== null && armed.ids === armedKey && armed.round === currentRound;

  const hidden = total === 0 && !doc.submit;

  const submit = () => {
    // Invocation-time guard: registration is effect-driven, so a chord can land
    // in the commit-to-cleanup window after the bar hides — the stale handle
    // must no-op.
    if (hidden || inFlight) return;
    if (undecidedApprovals > 0 && !confirming) {
      setArmed({ ids: armedKey, round: currentRound });
      return;
    }
    setInFlight(true);
    void post({ type: 'submit', revision }).then((ok) => {
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
            className={`tally-strip${complete ? ' tally-complete' : ''}`}
            role="group"
            aria-label="review progress"
          >
            {items.map((item, i) => (
              <button
                key={item.id}
                type="button"
                style={{ '--i': i } as CSSProperties}
                className={`tally-seg tally-${segState(item)}`}
                aria-label={`Item ${i + 1} of ${total}, ${item.decided ? 'decided' : 'undecided'} — jump`}
                onClick={() => kbd.jumpTo(item.id)}
              />
            ))}
          </div>
        )}
        <button
          type="button"
          className={`submit-count${complete ? ' submit-done' : ''}`}
          onClick={() => kbd.jumpNextUndecided()}
        >
          {decided}/{total} decided
        </button>
        {confirming && (
          <span className="submit-warn" role="status">
            {undecidedApprovals} {undecidedApprovals === 1 ? 'approval' : 'approvals'} still undecided
          </span>
        )}
        {revisingLine && (
          <span className="submit-revising" role="status">
            {revisingLine}
          </span>
        )}
        {doc.submit?.note && <span className="submit-note">{doc.submit.note}</span>}
      </div>
      <div className="submit-actions">
        {confirming && (
          <button type="button" className="link-btn" onClick={() => setArmed(null)}>
            Cancel
          </button>
        )}
        <button
          type="button"
          className={`primary submit-btn${confirming ? ' confirm' : ''}${complete && !confirming ? ' submit-ready' : ''}`}
          disabled={inFlight}
          onClick={submit}
        >
          {confirming ? 'Submit anyway?' : label}
        </button>
      </div>
    </div>
  );
}
