// A hand-rolled unified-diff parser: unified diff text into hunks of rows
// (old/new line numbers + kind) plus the `+++ b/path` new-file path per section.

export type DiffRowKind = 'add' | 'del' | 'context' | 'meta';

export interface DiffRow {
  kind: DiffRowKind;
  oldNo: number | null;
  newNo: number | null;
  text: string;
}

export interface DiffHunk {
  heading: string;
  path: string | null;
  rows: DiffRow[];
}

const HUNK_HEADER = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$/;
const NEW_FILE = /^\+\+\+ (?:b\/)?(.*?)(?:\t.*)?$/;

export function parseDiff(diff: string): DiffHunk[] {
  const hunks: DiffHunk[] = [];
  let current: DiffHunk | null = null;
  let currentPath: string | null = null;
  let oldNo = 0;
  let newNo = 0;
  let oldRem = 0;
  let newRem = 0;
  let complete = false;

  for (const line of diff.split('\n')) {
    const header = HUNK_HEADER.exec(line);
    if (header) {
      oldNo = Number(header[1]);
      newNo = Number(header[3]);
      oldRem = header[2] ? Number(header[2]) : 1;
      newRem = header[4] ? Number(header[4]) : 1;
      complete = oldRem <= 0 && newRem <= 0;
      current = { heading: (header[5] ?? '').trim(), path: currentPath, rows: [] };
      hunks.push(current);
      continue;
    }

    // A consumed hunk keeps only a trailing no-newline marker; anything else
    // opens the next file section.
    if (current && complete) {
      if (line[0] === '\\') {
        current.rows.push({ kind: 'meta', oldNo: null, newNo: null, text: line });
        continue;
      }
      current = null;
    }

    if (!current) {
      const file = NEW_FILE.exec(line);
      if (file) currentPath = file[1] ?? null;
      else if (line.startsWith('diff --git ')) currentPath = null;
      continue; // file headers before the next hunk
    }

    const marker = line[0];
    const text = line.slice(1);
    if (marker === '+') {
      current.rows.push({ kind: 'add', oldNo: null, newNo, text });
      newNo += 1;
      newRem -= 1;
    } else if (marker === '-') {
      current.rows.push({ kind: 'del', oldNo, newNo: null, text });
      oldNo += 1;
      oldRem -= 1;
    } else if (marker === '\\') {
      current.rows.push({ kind: 'meta', oldNo: null, newNo: null, text: line });
    } else {
      current.rows.push({ kind: 'context', oldNo, newNo, text });
      oldNo += 1;
      newNo += 1;
      oldRem -= 1;
      newRem -= 1;
    }
    if (oldRem <= 0 && newRem <= 0) complete = true;
  }

  return hunks;
}
