import { forwardRef, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { CollapsedGroup } from '@cc-interact/react';
import { m, useDragControls, useIsPresent, useMotionValue, useReducedMotion, useTransform } from 'motion/react';
import type { MotionValue, PanInfo } from 'motion/react';
import type { FocusStep } from '../focus';
import { stepHeadline, swipeVerdict } from '../focus';
import type { Block, OptionVisual } from '../schema';
import type { Interaction, Interactions } from '../events';
import { usePresent } from '../present';
import { revisionStore, useRevisingBanner, useUnseenChange } from '../revision';
import { useExpandAll } from '../expand';
import { renderMarkdown } from '../markdown';
import { BlockBody, BlockRenderer } from './BlockRenderer';
import { Clamped } from './Clamped';
import { FocusStageContext, FocusStepContext } from './focusStep';
import type { FocusStageValue } from './focusStep';
import { cardVariants } from './focusMotion';

// Heavy lead-in blocks collapse to a one-line titled disclosure; markdown clamps.
// term and filetree join them; chart and record stay expanded (bounded-height).
const HEAVY_CONTEXT = new Set(['code', 'diff', 'table', 'image', 'diagram', 'term', 'filetree']);

// contextTitle labels a demoted context or option-visual disclosure, else a type name.
// Mirrors iOS optionVisualTitle/focusContextTitle.
export function contextTitle(block: Block): string {
  switch (block.type) {
    case 'code':
      return block.title ?? block.lang;
    case 'diff':
      return block.title ?? 'Diff';
    case 'diagram':
      return block.title ?? 'Diagram';
    case 'image':
      return block.caption ?? block.alt;
    case 'table':
      return 'Table';
    case 'chart':
      return block.title ?? 'Chart';
    case 'term':
      return block.title ?? 'Terminal';
    case 'filetree':
      return block.title ?? 'Files';
    case 'record':
      return block.title ?? 'Record';
    default:
      return block.type;
  }
}

function FocusContextBlock({ block, interactions }: { block: Block; interactions: Interactions }) {
  const { epoch, expanded } = useExpandAll();
  if (block.type === 'markdown') {
    return <Clamped html={renderMarkdown(block.md)} lines={6} className="prose markdown-block focus-context-md" />;
  }
  if (HEAVY_CONTEXT.has(block.type)) {
    return (
      <CollapsedGroup key={epoch} defaultExpanded={expanded} header={contextTitle(block)}>
        <BlockRenderer block={block} interactions={interactions} />
      </CollapsedGroup>
    );
  }
  return <BlockRenderer block={block} interactions={interactions} />;
}

// useCompact tracks the --bp-compact (440px) breakpoint, mirroring useTheme's
// media-query subscription, so the stage can collapse on a phone.
function useCompact(): boolean {
  const [compact, setCompact] = useState(() => window.matchMedia('(max-width: 440px)').matches);
  useEffect(() => {
    const mql = window.matchMedia('(max-width: 440px)');
    const onChange = (e: MediaQueryListEvent) => setCompact(e.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);
  return compact;
}

// StageMedia is the option-visual stage. It always mounts the .focus-media slot
// (empty until an option publishes a visual); at --bp-compact a present visual
// collapses to a titled <details> so it never crowds the options, expanding inline.
function StageMedia({ visual, interactions }: { visual: OptionVisual | null; interactions: Interactions }) {
  const compact = useCompact();
  if (!visual) return <div className="focus-media" />;
  if (compact) {
    return (
      <details className="focus-media focus-media-disclosure">
        <summary className="focus-media-summary">{contextTitle(visual)}</summary>
        <div className="focus-media-body">
          <BlockBody block={visual} interactions={interactions} />
        </div>
      </details>
    );
  }
  return (
    <div className="focus-media">
      <BlockBody block={visual} interactions={interactions} />
    </div>
  );
}

// RevisionCallout fills the reserved .focus-revision slot: a warn banner while the
// step is being revised (passive after the 120s decay), else its changed-since-seen
// note. Controls stay live throughout — this is warn-only.
function RevisionCallout({ stepId }: { stepId: string }) {
  const banner = useRevisingBanner(stepId);
  const change = useUnseenChange(stepId);
  if (banner) {
    if (banner.passive) {
      return <p className="focus-revision-line passive">Claude may still be revising this step</p>;
    }
    return (
      <p className="focus-revision-line revising">
        Claude is rewriting this step{banner.note ? ` — ${banner.note}` : ''}
      </p>
    );
  }
  if (change) {
    const lead = change.kind === 'added' ? 'Claude added this step' : 'Updated after your earlier pick';
    return (
      <p className="focus-revision-line callout">
        {lead}
        {change.note ? ` — ${change.note}` : ''}
      </p>
    );
  }
  return null;
}

// A drag starts only off a non-interactive part of the card: elements that are
// focusable ([tabindex] other than the card's own -1), links/buttons (native or
// ARIA), form fields, or opt-out targets never begin a swipe — pack components
// render arbitrary such markup inside the card body.
export const NO_DRAG =
  'button, a, input, textarea, select, [contenteditable], [data-no-drag], [tabindex]:not([tabindex="-1"]), [role="button"], [role="link"]';

// swipeCommit is the drag-end decision path: a swipe past threshold posts the same
// decision.created event the verdict buttons and a/r keys post, then reports
// whether it committed; a short drag returns false and snaps back. Tests call it
// directly with synthetic PanInfo since jsdom cannot drive a real gesture.
export function swipeCommit(
  offsetX: number,
  velocityX: number,
  blockId: string,
  post: (interaction: Interaction) => unknown,
): boolean {
  const verdict = swipeVerdict(offsetX, velocityX);
  if (!verdict) return false;
  post({ type: 'decision.created', blockId, verdict });
  return true;
}

// resolveDragEnd is the released-gesture decision, lifted out of the component so
// tests can drive it with synthetic PanInfo. Under reduced motion it snaps the
// tilt back — dragSnapToOrigin's inertia is suppressed there — then posts a
// verdict only when the round is open, the card is still present (a keyboard nav
// or churn can retire it mid-drag), and the primary the gesture began against
// still leads the step (a same-id upsert can swap it).
export function resolveDragEnd(
  info: PanInfo,
  ctx: { closed: boolean; present: boolean; reduced: boolean; primaryId: string | undefined; dragPrimary: string | undefined },
  x: Pick<MotionValue<number>, 'set'>,
  post: (interaction: Interaction) => unknown,
): boolean {
  if (ctx.reduced) x.set(0);
  if (ctx.closed || !ctx.present || ctx.primaryId === undefined || ctx.primaryId !== ctx.dragPrimary) return false;
  return swipeCommit(info.offset.x, info.velocity.x, ctx.primaryId, post);
}

// FocusCard renders the live step body: its lead-in context then the focal block,
// each through BlockRenderer so the decidables register with the keyboard. It is
// the AnimatePresence child — a motion element that enters, exits with the deck's
// directional fly-off, and marks itself data-exiting while animating out so the
// deck and its tests target the live card. It forwards the ref AnimatePresence's
// popLayout mode injects so the outgoing card can be measured and lifted out of
// flow. A lone approval also becomes a swipe-to-decide surface: horizontal drag
// (started only off interactive children) tilts the card and reveals
// APPROVE/REJECT affordances, and a committed swipe drives the shared decide path.
// It never wraps a block in .block-row/data-flip-key — FLIP is board-only.
export const FocusCard = forwardRef<HTMLDivElement, { step: FocusStep; interactions: Interactions }>(function FocusCard(
  { step, interactions },
  ref,
) {
  const { post, closed } = usePresent();
  const closedRef = useRef(closed);
  closedRef.current = closed;
  const present = useIsPresent();
  const presentRef = useRef(present);
  presentRef.current = present;
  const reduced = useReducedMotion();
  const controls = useDragControls();
  const x = useMotionValue(0);
  const rotate = useTransform(x, [-280, 280], [-8, 8]);
  const approveOpacity = useTransform(x, [30, 130], [0, 1]);
  const rejectOpacity = useTransform(x, [-130, -30], [1, 0]);

  // The active option's visual, published by the step's Choice; rendered in the
  // stage above the controls. Resets with the step (this card remounts per step).
  const [activeVisual, setActiveVisual] = useState<OptionVisual | null>(null);
  const stageValue = useMemo<FocusStageValue>(() => ({ setVisual: setActiveVisual }), []);

  const primaryId = step.primary?.id;
  const swipeable = step.swipeable && primaryId !== undefined && !closed;
  // The primary a live drag began against; a same-id card upsert can swap the
  // step's primary (c1/a1 → c1/a2) under a held gesture without remounting.
  const dragPrimaryRef = useRef<string | undefined>(undefined);

  // Snap the tilt upright the instant the primary swaps under a held drag, so the
  // card never lingers rotated over the block it no longer represents.
  useEffect(() => {
    x.set(0);
  }, [primaryId, x]);

  // Mark the step seen on departure — this keyed card unmounts on a step change — so
  // its badge and callout clear once viewed but persist while the human is here.
  useEffect(() => () => revisionStore.markSeen(step.id), [step.id]);

  const onPointerDown = useCallback(
    (e: ReactPointerEvent<HTMLDivElement>) => {
      if ((e.target as HTMLElement).closest(NO_DRAG)) return;
      dragPrimaryRef.current = primaryId;
      controls.start(e);
    },
    [controls, primaryId],
  );

  const onDragEnd = useCallback(
    (_: unknown, info: PanInfo) => {
      // Read closed/present/primary at release, not at render: the round can
      // close and a keyboard nav, churn, or same-id upsert can retire or reshape
      // this card mid-drag, and framer fires onDragEnd after the re-render.
      resolveDragEnd(
        info,
        { closed: closedRef.current, present: presentRef.current, reduced: reduced === true, primaryId, dragPrimary: dragPrimaryRef.current },
        x,
        post,
      );
    },
    [primaryId, post, x, reduced],
  );

  const dragProps = swipeable
    ? {
        drag: 'x' as const,
        dragListener: false,
        dragControls: controls,
        dragSnapToOrigin: !reduced,
        dragMomentum: false,
        onPointerDown,
        onDragEnd,
        style: { x, rotate },
      }
    : {};

  const headline = stepHeadline(step);
  const headingId = `focus-q-${step.id}`;
  const card = step.block.type === 'card' ? step.block : null;
  const eyebrow = card && !headline.fromCard ? card.title : undefined;
  const status = card?.status;
  const chips = card?.chips;
  const hasMeta = eyebrow !== undefined || status !== undefined || (chips?.length ?? 0) > 0;

  return (
    <FocusStepContext.Provider value={{ headlineId: headline.suppressId }}>
      <FocusStageContext.Provider value={stageValue}>
      <m.div
        ref={ref}
        className="focus-card"
        tabIndex={-1}
        data-exiting={!present || undefined}
        aria-labelledby={headline.text ? headingId : undefined}
        variants={cardVariants}
        initial="enter"
        animate="center"
        exit="exit"
        {...dragProps}
      >
        {swipeable && (
          <>
            <m.div className="swipe-label approve" aria-hidden style={{ opacity: approveOpacity }}>
              Approve
            </m.div>
            <m.div className="swipe-label reject" aria-hidden style={{ opacity: rejectOpacity }}>
              Reject
            </m.div>
          </>
        )}
        {hasMeta && (
          <div className="focus-meta">
            {eyebrow !== undefined && <span className="focus-meta-eyebrow">{eyebrow}</span>}
            {status && <span className={`status status-${status}`}>{status}</span>}
            {chips && chips.length > 0 && (
              <span className="chips">
                {chips.map((chip, i) => (
                  <span key={i} className={`chip chip-${chip.tone ?? 'default'}`}>
                    {chip.label}
                  </span>
                ))}
              </span>
            )}
          </div>
        )}
        <div className="focus-revision">
          <RevisionCallout stepId={step.id} />
        </div>
        {headline.text && (
          <h2 id={headingId} className="focus-question">
            {headline.text}
          </h2>
        )}
        <div className="focus-card-body">
          {step.context.length > 0 && (
            <div className="focus-context">
              {step.context.map((block) => (
                <FocusContextBlock key={block.id} block={block} interactions={interactions} />
              ))}
            </div>
          )}
          <StageMedia visual={activeVisual} interactions={interactions} />
          <BlockRenderer key={step.block.id} block={step.block} interactions={interactions} />
        </div>
      </m.div>
      </FocusStageContext.Provider>
    </FocusStepContext.Provider>
  );
});
