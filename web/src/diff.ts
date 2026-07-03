// A hand-rolled unified-diff parser. It turns unified diff text into hunks of
// rows carrying old/new line numbers and a kind, which the Diff block renders as
// a two-gutter table. It reads the hunk headers for line numbering and ignores
// the file headers (diff/index/---/+++) that precede the first hunk.

export type DiffRowKind = 'add' | 'del' | 'context' | 'meta';

export interface DiffRow {
  kind: DiffRowKind;
  oldNo: number | null;
  newNo: number | null;
  text: string;
}

export interface DiffHunk {
  heading: string;
  rows: DiffRow[];
}

const HUNK_HEADER = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$/;

export function parseDiff(diff: string): DiffHunk[] {
  const hunks: DiffHunk[] = [];
  let current: DiffHunk | null = null;
  let oldNo = 0;
  let newNo = 0;

  for (const line of diff.split('\n')) {
    const header = HUNK_HEADER.exec(line);
    if (header) {
      oldNo = Number(header[1]);
      newNo = Number(header[2]);
      current = { heading: (header[3] ?? '').trim(), rows: [] };
      hunks.push(current);
      continue;
    }
    if (!current) continue; // file headers before the first hunk

    const marker = line[0];
    const text = line.slice(1);
    if (marker === '+') {
      current.rows.push({ kind: 'add', oldNo: null, newNo, text });
      newNo += 1;
    } else if (marker === '-') {
      current.rows.push({ kind: 'del', oldNo, newNo: null, text });
      oldNo += 1;
    } else if (marker === '\\') {
      current.rows.push({ kind: 'meta', oldNo: null, newNo: null, text: line });
    } else {
      current.rows.push({ kind: 'context', oldNo, newNo, text });
      oldNo += 1;
      newNo += 1;
    }
  }

  return hunks;
}
