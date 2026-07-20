// Shared ANSI escape handling; imported by TermView (strip all) and highlight.ts (keep SGR).

// OSC (ESC] ... BEL|ST, keeps OSC-8 link text), CSI (ESC[ ... final), or any other ESC.
const ANY_ESCAPE = new RegExp(
  [
    '\\u001B\\][^\\u0007\\u001B]*(?:\\u0007|\\u001B\\\\)',
    '\\u001B\\[[0-?]*[ -/]*[@-~]',
    '\\u001B[ -/]*[0-~]',
  ].join('|'),
  'g',
);

// Every escape but an SGR run (a CSI ending in `m`); the lookahead spares the SGR's ESC[.
const NON_SGR_ESCAPE = new RegExp(
  [
    '\\u001B\\][^\\u0007\\u001B]*(?:\\u0007|\\u001B\\\\)',
    '\\u001B\\[[0-?]*[ -/]*[@-ln-~]',
    '\\u001B(?!\\[)[ -/]*[0-~]',
  ].join('|'),
  'g',
);

// stripAnsi drops every ANSI escape, leaving the plain text a terminal would render.
export function stripAnsi(text: string): string {
  return text.replace(ANY_ESCAPE, '');
}

// sanitizeForSgr keeps SGR runs, drops cursor/erase/OSC shiki misreads, normalizes ESC[m.
export function sanitizeForSgr(text: string): string {
  return text.replace(/\x1b\[m/g, '\x1b[0m').replace(NON_SGR_ESCAPE, '');
}
