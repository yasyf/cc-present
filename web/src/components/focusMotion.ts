import type { Variants } from 'motion/react';

// The exit direction the deck threads through AnimatePresence.custom: a verdict
// (swipe / a·r / button) flies the card fully off-screen, plain navigation slides
// it a short way. dir is the sign of travel — right (+1) for an approval or a
// forward move, left (-1) for a rejection or a step back.
export interface ExitCustom {
  dir: 1 | -1;
  kind: 'verdict' | 'nav';
}

// cardVariants animate the deck's AnimatePresence child. A card enters from the
// peek's resting transform (so the next card appears to promote to the front) and
// settles with a spring; it exits in the threaded direction — a full fly-off on a
// verdict, a short slide on navigation. rotate/x on a swipeable card come from its
// own drag motion values, so the variants never set them on enter/center.
export const cardVariants: Variants = {
  enter: { opacity: 0, y: 10, scale: 0.96 },
  center: { opacity: 1, y: 0, scale: 1, transition: { type: 'spring', stiffness: 360, damping: 30 } },
  exit: (c: ExitCustom) =>
    c.kind === 'verdict'
      ? { x: c.dir * 1.2 * window.innerWidth, opacity: 0, transition: { duration: 0.28, ease: 'easeOut' } }
      : { x: c.dir * 44, opacity: 0, transition: { duration: 0.2, ease: 'easeOut' } },
};
