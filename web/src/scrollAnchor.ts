const USER_SCROLL = ['wheel', 'touchmove', 'keydown', 'pointerdown'] as const;

/**
 * Scrolls `target` to the top and re-scrolls on each `container` resize, until
 * the first user scroll input or `ms` elapses.
 */
export function anchorScroll(target: HTMLElement, container: HTMLElement, ms: number): void {
  const observer = new ResizeObserver(() => target.scrollIntoView({ block: 'start' }));
  const stop = () => {
    observer.disconnect();
    clearTimeout(timer);
    for (const type of USER_SCROLL) window.removeEventListener(type, stop);
  };
  const timer = setTimeout(stop, ms);
  for (const type of USER_SCROLL) window.addEventListener(type, stop, { passive: true });
  target.scrollIntoView({ block: 'start' });
  observer.observe(container);
}
