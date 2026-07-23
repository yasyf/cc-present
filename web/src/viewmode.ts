// The focus/board view resolution and per-subject persistence. resolveMode is the
// precedence the client renders by (viewmode.test.ts); loadView/saveView persist a
// viewer's explicit toggle in localStorage so it survives a reload.

import type { FocusStep } from './focus';
import { loadView, saveView } from './preferences';
import type { ViewMode } from './preferences';

export type { ViewMode } from './preferences';

// resolveMode: an explicit viewer override wins, then the doc's per-push hint,
// then the derived default — focus when any step decides, board otherwise.
export function resolveMode(
  presentation: ViewMode | undefined,
  override: ViewMode | null,
  steps: FocusStep[],
): ViewMode {
  return override ?? presentation ?? (steps.some((s) => s.kind === 'decision') ? 'focus' : 'board');
}

export { loadView, saveView };
