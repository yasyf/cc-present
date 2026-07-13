import { forwardRef, useCallback, useEffect, useRef } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { m, useDragControls, useIsPresent, useMotionValue, useReducedMotion, useTransform } from 'motion/react';
import type { MotionValue, PanInfo } from 'motion/react';
import type { FocusStep } from '../focus';
import { swipeVerdict } from '../focus';
import type { Interaction, Interactions } from '../events';
import { usePresent } from '../present';
import { BlockRenderer } from './BlockRenderer';
import { cardVariants } from './focusMotion';

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

  return (
    <m.div
      ref={ref}
      className="focus-card"
      tabIndex={-1}
      data-exiting={!present || undefined}
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
      {step.tier && <div className="focus-tier">{step.tier}</div>}
      <div className="focus-card-body">
        {step.context.map((block) => (
          <BlockRenderer key={block.id} block={block} interactions={interactions} />
        ))}
        <BlockRenderer key={step.block.id} block={step.block} interactions={interactions} />
      </div>
    </m.div>
  );
});
