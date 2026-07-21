import { useCallback, useEffect, useRef, useState } from 'react';
import type { SyntheticEvent } from 'react';
import { AnimatePresence, LazyMotion, MotionConfig, domMax } from 'motion/react';
import { submitItems } from '../decide';
import { autoAdvances, stepTitle } from '../focus';
import type { FocusStep } from '../focus';
import { useKeyboardApi } from '../keyboard';
import { revisionStore } from '../revision';
import { useInteractivePackTypes } from '../packs/registry';
import type { Block } from '../schema';
import type { Interactions } from '../events';
import { FocusCard } from './FocusCard';
import { FocusPeek } from './FocusPeek';
import { FocusProgress } from './FocusProgress';
import { WizardFooter } from './WizardFooter';
import { FocusSummary } from './FocusSummary';
import type { ExitCustom } from './focusMotion';

// A sentinel step id for the review summary — never a real block id — so the deck
// stores one anchor id and the summary survives every recompute.
const DECK_END = '__deck_end__';

const AUTO_ADVANCE_MS = 450;

export interface FocusDeckProps {
  steps: FocusStep[];
  interactions: Interactions;
  round: number;
  closed: boolean;
  // Reports whether the deck rests on the Review summary, so the app mounts the
  // SubmitBar there and nowhere else in focus mode.
  onEndChange?: (atEnd: boolean) => void;
}

function stepUndecided(step: FocusStep, interactions: Interactions, packInteractive: ReadonlySet<string>): boolean {
  return submitItems([step.block], interactions, packInteractive).some((item) => !item.decided);
}

// deckIndex resolves the stored anchor to a rendered index: the summary sentinel
// (or an empty deck) maps past the last step; a vanished anchor clamps to the
// nearest surviving step, never the summary — that is reached only by navigating
// there or an empty deck.
function deckIndex(steps: FocusStep[], currentId: string, lastIndex: number): number {
  if (currentId === DECK_END || steps.length === 0) return steps.length;
  const i = steps.findIndex((s) => s.id === currentId);
  return i >= 0 ? i : Math.min(lastIndex, steps.length - 1);
}

// decisionSignature is the selection-only (re-)arm fingerprint: an approval's
// verdict or a single-select choice's optionIds + write-in; null when undecided.
function decisionSignature(primary: Block, interactions: Interactions): string | null {
  if (primary.type === 'approval') {
    const verdict = interactions.decisions[primary.id]?.verdict;
    return verdict === 'approved' || verdict === 'rejected' ? verdict : null;
  }
  const selection = interactions.choices[primary.id];
  if (!selection || (selection.optionIds.length === 0 && selection.other === undefined)) return null;
  return `${selection.optionIds.join(',')} ${selection.other ?? ''}`;
}

// FocusDeck owns the current step (an anchor id, re-derived to an index each
// recompute and clamped when the anchor vanishes). It registers a StepNav for the
// keyboard, sets the cursor to the step's primary decidable (or the jumped-to
// nested one), moves DOM focus and announces on each step change, and auto-advances
// a lone approval or single-select choice 450ms after its decision.
export function FocusDeck({ steps, interactions, round, closed, onEndChange }: FocusDeckProps) {
  const api = useKeyboardApi();
  const packInteractive = useInteractivePackTypes();
  const deckRef = useRef<HTMLDivElement>(null);
  const pendingCursorRef = useRef<string | null>(null);
  const prevDecidedRef = useRef<{ id: string; sig: string | null } | null>(null);
  const timerRef = useRef<{ id: ReturnType<typeof setTimeout>; stepId: string } | null>(null);
  const [currentId, setCurrentId] = useState<string>(() => steps[0]?.id ?? DECK_END);
  // The direction AnimatePresence flies the outgoing card: a verdict sends it fully
  // off toward its sign, plain navigation slides it toward the move.
  const [exitCustom, setExitCustom] = useState<ExitCustom>({ dir: 1, kind: 'nav' });
  // True while the 450ms auto-advance is armed — drives the Next control's cue.
  const [advancing, setAdvancing] = useState(false);

  const total = steps.length;
  const lastIndexRef = useRef(0);
  const index = deckIndex(steps, currentId, lastIndexRef.current);
  const currentStep = index < total ? steps[index]! : undefined;
  const wantId = currentStep ? currentStep.id : DECK_END;

  const stepsRef = useRef(steps);
  const indexRef = useRef(index);
  const interactionsRef = useRef(interactions);
  const packRef = useRef(packInteractive);
  stepsRef.current = steps;
  indexRef.current = index;
  interactionsRef.current = interactions;
  packRef.current = packInteractive;

  const go = useCallback((i: number) => {
    const list = stepsRef.current;
    const clamped = Math.max(0, Math.min(i, list.length));
    setCurrentId(clamped >= list.length ? DECK_END : list[clamped]!.id);
  }, []);
  const move = useCallback(
    (delta: 1 | -1) => {
      setExitCustom({ dir: delta, kind: 'nav' });
      go(indexRef.current + delta);
    },
    [go],
  );
  const jump = useCallback(
    (id: string) => {
      const list = stepsRef.current;
      const i = list.findIndex((s) => s.id === id || s.decidables.includes(id));
      if (i < 0) return;
      // A jump to a nested decidable lands the cursor on that id; a jump to the
      // anchor (or a non-decidable target) falls back to the step's primary.
      const target = list[i]!.decidables.includes(id) ? id : list[i]!.primary?.id ?? null;
      if (i === indexRef.current) {
        // Same-step jump: go bails on an unchanged anchor, so the cursor effect
        // never fires — set the cursor here instead.
        api.setCursor(target);
        return;
      }
      setExitCustom({ dir: i > indexRef.current ? 1 : -1, kind: 'nav' });
      pendingCursorRef.current = target;
      go(i);
    },
    [api, go],
  );
  const next = useCallback(() => {
    const list = stepsRef.current;
    // Wrap modularly across the deck (mirroring nextUndecided) so an undecided
    // step behind the cursor is reached; land on the summary only when nothing is.
    const from = indexRef.current;
    const revising = revisionStore.revisingSet();
    const land = (i: number) => {
      setExitCustom({ dir: i > from ? 1 : -1, kind: 'nav' });
      go(i);
    };
    // First pass prefers non-revising undecided steps (momentum); a revising step
    // stays reachable on the second pass — never locked out.
    for (let hop = 1; hop <= list.length; hop++) {
      const i = (from + hop) % list.length;
      const s = list[i]!;
      if (stepUndecided(s, interactionsRef.current, packRef.current) && !revising.has(s.id)) {
        land(i);
        return;
      }
    }
    for (let hop = 1; hop <= list.length; hop++) {
      const i = (from + hop) % list.length;
      if (stepUndecided(list[i]!, interactionsRef.current, packRef.current)) {
        land(i);
        return;
      }
    }
    setExitCustom({ dir: 1, kind: 'nav' });
    go(list.length);
  }, [go]);

  // Stray input retracts an armed advance; a pick (.verdict/.option — its re-decide
  // re-arms below) or anything in a live composer ([data-composing]) is exempt.
  const cancelAdvance = useCallback((e: SyntheticEvent) => {
    if (e.target instanceof Element && e.target.closest('.verdict, .option, [data-composing]')) return;
    if (timerRef.current) {
      clearTimeout(timerRef.current.id);
      timerRef.current = null;
    }
    setAdvancing(false);
  }, []);

  // Keep the stored anchor and the last-known position in sync with the rendered
  // index so a subsequent recompute clamps to the right neighbourhood.
  useEffect(() => {
    if (wantId !== currentId) setCurrentId(wantId);
    lastIndexRef.current = index;
  }, [wantId, currentId, index]);

  useEffect(() => {
    api.registerStepNav({ move, next, jump });
    return () => api.registerStepNav(null);
  }, [api, move, next, jump]);

  const primaryId = currentStep?.primary?.id ?? null;
  useEffect(() => {
    const pending = pendingCursorRef.current;
    pendingCursorRef.current = null;
    api.setCursor(pending ?? primaryId);
  }, [api, primaryId]);

  useEffect(() => {
    const advancer = !closed && currentStep && autoAdvances(currentStep) ? currentStep.primary : undefined;
    const stepId = currentStep?.id;
    const sig = advancer ? decisionSignature(advancer, interactions) : null;
    const decided = sig !== null;
    const prev = prevDecidedRef.current;
    prevDecidedRef.current = advancer ? { id: stepId!, sig } : null;
    // The signature changed on this same step — a first pick or a re-pick — so
    // (re-)arm below; a redundant echo keeps the signature and stays false.
    const revised = prev !== null && prev.id === stepId && prev.sig !== sig;

    // An armed timer survives the echo re-render; cancel on a step change, a cleared
    // decision, or a revised signature (the re-pick re-arms below), else keep it.
    const armed = timerRef.current;
    if (armed && (armed.stepId !== stepId || !decided || revised)) {
      clearTimeout(armed.id);
      timerRef.current = null;
      setAdvancing(false);
    }
    // Arm 450ms after the signature changes on this lone current decidable — never
    // on arrival, revisit, or echo. The fly-off sign is read when the timer fires.
    if (advancer && decided && revised && !timerRef.current) {
      const advancerId = advancer.id;
      const isApproval = advancer.type === 'approval';
      setAdvancing(true);
      api.announce('Advancing to the next step');
      timerRef.current = {
        stepId: stepId!,
        id: setTimeout(() => {
          timerRef.current = null;
          setAdvancing(false);
          // A composer holds the advance whether it lives inline in the card or out
          // in the margin rail / comments sheet (rail mode routes `f` there).
          if (
            deckRef.current?.querySelector('.focus-card:not([data-exiting]) [data-composing]') ||
            document.querySelector('.margin-rail [data-composing], dialog [data-composing]')
          )
            return;
          const dir: 1 | -1 =
            isApproval && interactionsRef.current.decisions[advancerId]?.verdict === 'rejected' ? -1 : 1;
          setExitCustom({ dir, kind: 'verdict' });
          go(indexRef.current + 1);
        }, AUTO_ADVANCE_MS),
      };
    }
  }, [api, closed, currentStep, interactions, go]);

  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current.id);
    },
    [],
  );

  const prevIndexRef = useRef(index);
  useEffect(() => {
    if (prevIndexRef.current === index) return;
    prevIndexRef.current = index;
    // Move DOM focus onto the freshly-mounted card (or the summary) so keyboard
    // and screen-reader users land on it; the deck never scrolls the cursor in
    // focus mode, so preventScroll keeps the viewport still.
    deckRef.current
      ?.querySelector<HTMLElement>('.focus-card:not([data-exiting]), .focus-summary:not([data-exiting])')
      ?.focus({ preventScroll: true });
    api.announce(currentStep ? `Step ${index + 1} of ${total} — ${stepTitle(currentStep)}` : 'Review');
  }, [api, index, total, currentStep]);

  const onSummary = index >= total;
  useEffect(() => {
    onEndChange?.(onSummary);
  }, [onSummary, onEndChange]);

  // A clamp (the current anchor vanished from live churn) is not a verdict
  // advance: fly the outgoing card out on a plain nav slide, never the stale
  // verdict fly-off exitCustom still holds from an earlier decision. This is
  // derived for THIS render — the exit is captured now, before the sync effect
  // updates currentId — and framer locks an already-airborne exit's variant at
  // its start, so the override only reaches the card leaving on this commit.
  const clamped = currentStep !== undefined && currentStep.id !== currentId;
  const exitCustomForPresence: ExitCustom =
    clamped && exitCustom.kind === 'verdict' ? { dir: 1, kind: 'nav' } : exitCustom;

  return (
    <LazyMotion features={domMax} strict>
      <MotionConfig reducedMotion="user">
        <div
          className="focus-deck"
          ref={deckRef}
          onKeyDownCapture={cancelAdvance}
          onPointerDownCapture={cancelAdvance}
        >
          <FocusProgress steps={steps} index={index} />
          <div className="focus-stage">
            {!onSummary && index + 1 < total && <FocusPeek step={steps[index + 1]!} />}
            <AnimatePresence mode="popLayout" custom={exitCustomForPresence} initial={false}>
              {onSummary ? (
                <FocusSummary
                  key={`${round}:${DECK_END}`}
                  steps={steps}
                  interactions={interactions}
                  packInteractive={packInteractive}
                  onJump={jump}
                />
              ) : (
                <FocusCard key={`${round}:${currentStep!.id}`} step={currentStep!} interactions={interactions} />
              )}
            </AnimatePresence>
          </div>
          <WizardFooter
            index={index}
            total={total}
            advancing={advancing}
            onPrev={() => move(-1)}
            onNext={() => move(1)}
            steps={steps}
            interactions={interactions}
            packInteractive={packInteractive}
            onJump={jump}
          />
        </div>
      </MotionConfig>
    </LazyMotion>
  );
}
