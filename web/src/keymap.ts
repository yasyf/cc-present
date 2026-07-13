// The keyboard grammar as a pure function: a KeyDescriptor (a plain snapshot of a
// keydown, DOM-free) plus whether the target is a text field and whether the
// board is closed, folded into a KbdAction the provider dispatches. KEYMAP is the
// same grammar in prose, driving the help overlay so the two never drift.

export interface KeyDescriptor {
  key: string;
  meta: boolean;
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
  repeat: boolean;
}

export type KbdAction =
  | { kind: 'move'; delta: 1 | -1 }
  | { kind: 'next-undecided' }
  | { kind: 'verdict'; verdict: 'approved' | 'rejected' }
  | { kind: 'clear' }
  | { kind: 'choose'; option: number }
  | { kind: 'engage' }
  | { kind: 'submit' }
  | { kind: 'view-toggle' }
  | { kind: 'expand-all' }
  | { kind: 'help-toggle' }
  | { kind: 'escape' };

export interface KeymapRow {
  keys: string[];
  context: string;
  action: string;
}

export const KEYMAP: KeymapRow[] = [
  { keys: ['j', '↓', 'k', '↑'], context: 'Browsing', action: 'Move to the next / previous item' },
  { keys: ['n'], context: 'Browsing', action: 'Jump to the next undecided item' },
  { keys: ['a', 'r'], context: 'On an approval', action: 'Approve / reject (press again to clear)' },
  { keys: ['c'], context: 'On an approval', action: 'Clear the verdict' },
  { keys: ['1', '…', '9'], context: 'On a choice', action: 'Toggle option 1–9' },
  { keys: ['f'], context: 'On an approval or field', action: 'Add feedback / focus the field' },
  { keys: ['⌘/Ctrl', '⏎'], context: 'Anywhere', action: 'Submit the round (confirm when items are undecided); while writing feedback, sends the note instead' },
  { keys: ['v'], context: 'Anywhere', action: 'Toggle focus / board view' },
  { keys: ['e'], context: 'Anywhere', action: 'Expand / collapse all clamped content' },
  { keys: ['?'], context: 'Anywhere', action: 'Toggle this help' },
  { keys: ['Esc'], context: 'Anywhere', action: 'Close help, else leave the field' },
];

// TOGGLE_SUPPRESSED lists the actions a re-press oscillates; a held key must not
// auto-repeat them. Movement, escape, and submit stay repeatable and stay out.
const TOGGLE_SUPPRESSED = new Set<KbdAction['kind']>([
  'verdict',
  'clear',
  'choose',
  'engage',
  'view-toggle',
  'expand-all',
  'help-toggle',
]);

// resolveKey is the raw grammar (auto-repeat ignored): Esc survives a text field,
// `?`/`v`/`e` survive a closed board, and a field swallows the rest.
function resolveKey(d: KeyDescriptor, typing: boolean, closed: boolean): KbdAction | null {
  if (d.key === 'Escape') return { kind: 'escape' };
  if (typing) return null;

  if (d.key === '?') return { kind: 'help-toggle' };
  if (d.key === 'v') return { kind: 'view-toggle' };
  if (d.key === 'e') return { kind: 'expand-all' };
  if (closed) return null;

  switch (d.key) {
    case 'j':
    case 'ArrowDown':
      return { kind: 'move', delta: 1 };
    case 'k':
    case 'ArrowUp':
      return { kind: 'move', delta: -1 };
    case 'n':
      return { kind: 'next-undecided' };
    case 'a':
      return { kind: 'verdict', verdict: 'approved' };
    case 'r':
      return { kind: 'verdict', verdict: 'rejected' };
    case 'c':
      return { kind: 'clear' };
    case 'f':
      return { kind: 'engage' };
  }
  if (/^[1-9]$/.test(d.key)) return { kind: 'choose', option: Number(d.key) };
  return null;
}

// interpretKey resolves one keydown: mod+Enter submits (its repeat dropped so a
// held chord can't blow through an armed confirm), a modifier otherwise bails, and
// one guard drops the repeat of every toggle-semantics action so a held key can't
// oscillate a decision or overlay; movement and Esc stay repeatable.
export function interpretKey(d: KeyDescriptor, typing: boolean, closed: boolean): KbdAction | null {
  if ((d.meta || d.ctrl) && d.key === 'Enter') {
    return d.repeat ? null : { kind: 'submit' };
  }
  if (d.meta || d.ctrl || d.alt) return null;

  const action = resolveKey(d, typing, closed);
  return action && d.repeat && TOGGLE_SUPPRESSED.has(action.kind) ? null : action;
}
