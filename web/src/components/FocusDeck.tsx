import { useCallback, useEffect, useRef, useState } from 'react';
import { isDecided, submitItems } from '../decide';
import { stepTitle } from '../focus';
import type { FocusStep } from '../focus';
import { useKeyboardApi } from '../keyboard';
import { useInteractivePackTypes } from '../packs/registry';
import type { Interactions } from '../events';
import { FocusCard } from './FocusCard';
import { FocusPeek } from './FocusPeek';
import { FocusProgress } from './FocusProgress';
import { FocusNav } from './FocusNav';
import { FocusSummary } from './FocusSummary';

// A sentinel step id for the review summary — never a real block id — so the deck
// stores one anchor id and the summary survives every recompute.
const DECK_END = '__deck_end__';

const AUTO_ADVANCE_MS = 450;

export interface FocusDeckProps {
  steps: FocusStep[];
  interactions: Interactions;
  round: number;
  closed: boolean;
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

// FocusDeck owns the current step (as an anchor id, re-derived to an index on each
// recompute and clamped when the anchor vanishes). It registers a StepNav so the
// keyboard drives it, sets the cursor to the step's primary decidable (or the
// jumped-to nested one), moves DOM focus and announces on each step change, and
// auto-advances an approval 450ms after its verdict unless feedback is composing.
export function FocusDeck({ steps, interactions, round, closed }: FocusDeckProps) {
  const api = useKeyboardApi();
  const packInteractive = useInteractivePackTypes();
  const deckRef = useRef<HTMLDivElement>(null);
  const pendingCursorRef = useRef<string | null>(null);
  const prevDecidedRef = useRef<{ id: string; decided: boolean } | null>(null);
  const [currentId, setCurrentId] = useState<string>(() => steps[0]?.id ?? DECK_END);

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
  const move = useCallback((delta: 1 | -1) => go(indexRef.current + delta), [go]);
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
    for (let hop = 1; hop <= list.length; hop++) {
      const i = (from + hop) % list.length;
      if (stepUndecided(list[i]!, interactionsRef.current, packRef.current)) {
        go(i);
        return;
      }
    }
    go(list.length);
  }, [go]);

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
    if (closed || !currentStep) {
      prevDecidedRef.current = null;
      return;
    }
    const primary = currentStep.primary;
    if (!primary || primary.type !== 'approval' || currentStep.decidables.length !== 1) {
      prevDecidedRef.current = null;
      return;
    }
    const decided = isDecided(primary, interactions);
    const prev = prevDecidedRef.current;
    prevDecidedRef.current = { id: currentStep.id, decided };
    // Only an undecided→decided transition while this step is current arms the
    // timer; arriving on (or revisiting) an already-decided approval is stable.
    if (!prev || prev.id !== currentStep.id || prev.decided || !decided) return;
    const timer = setTimeout(() => {
      if (deckRef.current?.querySelector('[data-composing]')) return;
      go(indexRef.current + 1);
    }, AUTO_ADVANCE_MS);
    return () => clearTimeout(timer);
  }, [closed, currentStep, interactions, go]);

  const prevIndexRef = useRef(index);
  useEffect(() => {
    if (prevIndexRef.current === index) return;
    prevIndexRef.current = index;
    // Move DOM focus onto the freshly-mounted card (or the summary) so keyboard
    // and screen-reader users land on it; the deck never scrolls the cursor in
    // focus mode, so preventScroll keeps the viewport still.
    deckRef.current?.querySelector<HTMLElement>('.focus-card, .focus-summary')?.focus({ preventScroll: true });
    api.announce(currentStep ? `Step ${index + 1} of ${total} — ${stepTitle(currentStep)}` : 'Review');
  }, [api, index, total, currentStep]);

  const onSummary = index >= total;

  return (
    <div className="focus-deck" ref={deckRef}>
      <FocusProgress
        steps={steps}
        index={index}
        interactions={interactions}
        packInteractive={packInteractive}
        onJump={jump}
      />
      <div className="focus-stage">
        {!onSummary && index + 1 < total && <FocusPeek step={steps[index + 1]!} />}
        {onSummary ? (
          <FocusSummary steps={steps} interactions={interactions} packInteractive={packInteractive} onJump={jump} />
        ) : (
          <FocusCard key={`${round}:${currentStep!.id}`} step={currentStep!} interactions={interactions} />
        )}
      </div>
      <FocusNav index={index} total={total} onPrev={() => move(-1)} onNext={() => move(1)} />
    </div>
  );
}
