// The focus/board view resolution and per-subject persistence. resolveMode is the
// precedence the client renders by (viewmode.test.ts); loadView/saveView persist a
// viewer's explicit toggle in localStorage so it survives a reload.

import type { FocusStep } from './focus';

export type ViewMode = 'focus' | 'board';

// resolveMode: an explicit viewer override wins, then the doc's per-push hint,
// then the derived default — focus when any step decides, board otherwise.
export function resolveMode(
  presentation: ViewMode | undefined,
  override: ViewMode | null,
  steps: FocusStep[],
): ViewMode {
  return override ?? presentation ?? (steps.some((s) => s.kind === 'decision') ? 'focus' : 'board');
}

export function viewKey(ref: string): string {
  return `cc-present:view:${ref}`;
}

// loadView reads a persisted override, treating any non-mode value as absent.
export function loadView(ref: string): ViewMode | null {
  const v = localStorage.getItem(viewKey(ref));
  return v === 'focus' || v === 'board' ? v : null;
}

export function saveView(ref: string, mode: ViewMode): void {
  localStorage.setItem(viewKey(ref), mode);
}
