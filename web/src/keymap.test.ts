import { describe, expect, it } from 'vitest';
import { KEYMAP, interpretKey } from './keymap';
import type { KbdAction, KeyDescriptor } from './keymap';

const key = (over: Partial<KeyDescriptor>): KeyDescriptor => ({
  key: 'a',
  meta: false,
  ctrl: false,
  alt: false,
  shift: false,
  repeat: false,
  ...over,
});

interface Case {
  name: string;
  key: KeyDescriptor;
  typing: boolean;
  closed: boolean;
  expected: KbdAction | null;
}

const cases: Case[] = [
  { name: 'j moves forward', key: key({ key: 'j' }), typing: false, closed: false, expected: { kind: 'move', delta: 1 } },
  { name: 'ArrowDown moves forward', key: key({ key: 'ArrowDown' }), typing: false, closed: false, expected: { kind: 'move', delta: 1 } },
  { name: 'k moves back', key: key({ key: 'k' }), typing: false, closed: false, expected: { kind: 'move', delta: -1 } },
  { name: 'ArrowUp moves back', key: key({ key: 'ArrowUp' }), typing: false, closed: false, expected: { kind: 'move', delta: -1 } },
  { name: 'n jumps to next undecided', key: key({ key: 'n' }), typing: false, closed: false, expected: { kind: 'next-undecided' } },
  { name: 'a approves', key: key({ key: 'a' }), typing: false, closed: false, expected: { kind: 'verdict', verdict: 'approved' } },
  { name: 'r rejects', key: key({ key: 'r' }), typing: false, closed: false, expected: { kind: 'verdict', verdict: 'rejected' } },
  { name: 'c clears', key: key({ key: 'c' }), typing: false, closed: false, expected: { kind: 'clear' } },
  { name: 'f engages', key: key({ key: 'f' }), typing: false, closed: false, expected: { kind: 'engage' } },
  { name: '3 toggles option 3', key: key({ key: '3' }), typing: false, closed: false, expected: { kind: 'choose', option: 3 } },
  { name: '0 is unbound', key: key({ key: '0' }), typing: false, closed: false, expected: null },
  { name: 'meta+Enter submits', key: key({ key: 'Enter', meta: true }), typing: false, closed: false, expected: { kind: 'submit' } },
  { name: 'ctrl+Enter submits', key: key({ key: 'Enter', ctrl: true }), typing: false, closed: false, expected: { kind: 'submit' } },
  { name: 'held mod+Enter (repeat) is dropped', key: key({ key: 'Enter', meta: true, repeat: true }), typing: false, closed: false, expected: null },
  { name: 'mod+Enter submits while typing', key: key({ key: 'Enter', ctrl: true }), typing: true, closed: false, expected: { kind: 'submit' } },
  { name: 'held mod+Enter while typing is dropped', key: key({ key: 'Enter', meta: true, repeat: true }), typing: true, closed: false, expected: null },
  { name: 'v toggles the view', key: key({ key: 'v' }), typing: false, closed: false, expected: { kind: 'view-toggle' } },
  { name: 'held v (repeat) is dropped', key: key({ key: 'v', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held v (repeat) is dropped on a closed board', key: key({ key: 'v', repeat: true }), typing: false, closed: true, expected: null },
  { name: 'held a (repeat) is dropped', key: key({ key: 'a', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held r (repeat) is dropped', key: key({ key: 'r', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held c (repeat) is dropped', key: key({ key: 'c', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held f (repeat) is dropped', key: key({ key: 'f', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held 3 (repeat) is dropped', key: key({ key: '3', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held ? (repeat) is dropped', key: key({ key: '?', shift: true, repeat: true }), typing: false, closed: false, expected: null },
  { name: 'held j (repeat) still moves', key: key({ key: 'j', repeat: true }), typing: false, closed: false, expected: { kind: 'move', delta: 1 } },
  { name: 'held n (repeat) still jumps to next undecided', key: key({ key: 'n', repeat: true }), typing: false, closed: false, expected: { kind: 'next-undecided' } },
  { name: 'held Escape (repeat) still escapes', key: key({ key: 'Escape', repeat: true }), typing: false, closed: false, expected: { kind: 'escape' } },
  { name: 'v survives a closed board', key: key({ key: 'v' }), typing: false, closed: true, expected: { kind: 'view-toggle' } },
  { name: 'typing swallows v', key: key({ key: 'v' }), typing: true, closed: false, expected: null },
  { name: 'meta+v is unbound', key: key({ key: 'v', meta: true }), typing: false, closed: false, expected: null },
  { name: 'e expands all', key: key({ key: 'e' }), typing: false, closed: false, expected: { kind: 'expand-all' } },
  { name: 'held e (repeat) is dropped', key: key({ key: 'e', repeat: true }), typing: false, closed: false, expected: null },
  { name: 'e survives a closed board', key: key({ key: 'e' }), typing: false, closed: true, expected: { kind: 'expand-all' } },
  { name: 'typing swallows e', key: key({ key: 'e' }), typing: true, closed: false, expected: null },
  { name: 'meta+e is unbound', key: key({ key: 'e', meta: true }), typing: false, closed: false, expected: null },
  { name: '? toggles help', key: key({ key: '?', shift: true }), typing: false, closed: false, expected: { kind: 'help-toggle' } },
  { name: 'Escape escapes', key: key({ key: 'Escape' }), typing: false, closed: false, expected: { kind: 'escape' } },
  { name: 'typing swallows a letter', key: key({ key: 'a' }), typing: true, closed: false, expected: null },
  { name: 'typing swallows ? as a literal', key: key({ key: '?', shift: true }), typing: true, closed: false, expected: null },
  { name: 'Escape survives typing to blur the field', key: key({ key: 'Escape' }), typing: true, closed: false, expected: { kind: 'escape' } },
  { name: 'closed swallows navigation', key: key({ key: 'j' }), typing: false, closed: true, expected: null },
  { name: 'closed swallows a verdict', key: key({ key: 'a' }), typing: false, closed: true, expected: null },
  { name: '? survives a closed board', key: key({ key: '?', shift: true }), typing: false, closed: true, expected: { kind: 'help-toggle' } },
  { name: 'Escape survives a closed board', key: key({ key: 'Escape' }), typing: false, closed: true, expected: { kind: 'escape' } },
  { name: 'alt+j is unbound', key: key({ key: 'j', alt: true }), typing: false, closed: false, expected: null },
  { name: 'meta+j is unbound', key: key({ key: 'j', meta: true }), typing: false, closed: false, expected: null },
];

describe('interpretKey', () => {
  for (const c of cases) {
    it(c.name, () => {
      expect(interpretKey(c.key, c.typing, c.closed)).toEqual(c.expected);
    });
  }
});

// The interpretKey submit assertions above describe the pure grammar, which the
// help overlay renders from KEYMAP. The chord also submits from a text field, but
// an open feedback composer intercepts mod+Enter (preventDefault before this ever
// runs) to send the note — the row must document that so the two never drift.
describe('KEYMAP', () => {
  it('documents that mod+Enter sends the note while writing feedback', () => {
    const row = KEYMAP.find((r) => r.keys.includes('⏎'));
    expect(row?.action).toMatch(/feedback/i);
  });

  it('documents the view toggle so the help overlay renders it', () => {
    const row = KEYMAP.find((r) => r.keys.includes('v'));
    expect(row?.action).toMatch(/view/i);
  });

  it('documents the expand-all toggle so the help overlay renders it', () => {
    const row = KEYMAP.find((r) => r.keys.includes('e'));
    expect(row?.action).toMatch(/expand/i);
  });
});
