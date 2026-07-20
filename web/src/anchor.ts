const letters = 'abcdefghjkmnpqrstvwxyz';
const alphabet = `0123456789${letters}`;
const refPattern = /^(?:(\d+)(?:-(\d+))?#)?([a-hjkmnp-tv-z][0-9a-hjkmnp-tv-z]{3})$/;
const encoder = new TextEncoder();

export interface AnchorRef {
  line: number;
  end: number;
  hash: string;
}

export interface AnchorResolution {
  start: number;
  end: number;
  moved: boolean;
  from: number;
}

function isAnchorSpace(code: number): boolean {
  return (
    (code >= 0x0009 && code <= 0x000d) ||
    code === 0x0020 ||
    code === 0x0085 ||
    code === 0x00a0 ||
    code === 0x1680 ||
    (code >= 0x2000 && code <= 0x200a) ||
    code === 0x2028 ||
    code === 0x2029 ||
    code === 0x202f ||
    code === 0x205f ||
    code === 0x3000
  );
}

function trimAnchorSpace(value: string): string {
  let start = 0;
  let end = value.length;
  while (start < end && isAnchorSpace(value.charCodeAt(start))) {
    start++;
  }
  while (end > start && isAnchorSpace(value.charCodeAt(end - 1))) {
    end--;
  }
  return value.slice(start, end);
}

export function anchorOf(line: string): string {
  let h = 2166136261;
  for (const byte of encoder.encode(trimAnchorSpace(line))) {
    h = Math.imul(h ^ byte, 16777619) >>> 0;
  }

  const value = h % 720896;
  return (
    letters.charAt(value >>> 15) +
    alphabet.charAt((value >>> 10) & 31) +
    alphabet.charAt((value >>> 5) & 31) +
    alphabet.charAt(value & 31)
  );
}

export function parseAnchor(ref: string): AnchorRef {
  const match = refPattern.exec(ref);
  if (match === null || match[0] !== ref) {
    throw new Error(`parse anchor ${JSON.stringify(ref)}: invalid anchor reference`);
  }

  const hash = match[3]!;
  if (match[1] === undefined) {
    return { line: 0, end: 0, hash };
  }

  const line = Number(match[1]);
  if (!Number.isSafeInteger(line)) {
    throw new Error(`parse anchor ${JSON.stringify(ref)} line: invalid integer`);
  }
  if (line === 0) {
    throw new Error(`parse anchor ${JSON.stringify(ref)} line must be positive: invalid anchor reference`);
  }
  if (match[2] === undefined) {
    return { line, end: line, hash };
  }

  const end = Number(match[2]);
  if (!Number.isSafeInteger(end)) {
    throw new Error(`parse anchor ${JSON.stringify(ref)} range end: invalid integer`);
  }
  if (end < line) {
    throw new Error(`parse anchor ${JSON.stringify(ref)} range is reversed: invalid anchor reference`);
  }
  return { line, end, hash };
}

export function formatAnchor(line: number, hash: string): string {
  return `${line}#${hash}`;
}

export function formatRangeAnchor(start: number, end: number, hash: string): string {
  return `${start}-${end}#${hash}`;
}

export function resolveAnchor(ref: AnchorRef, lines: string[]): AnchorResolution {
  if (ref.line > 0 && ref.line <= lines.length && anchorOf(lines[ref.line - 1]!) === ref.hash) {
    return resolved(ref, ref.line, lines.length);
  }

  const candidates: number[] = [];
  for (let i = 0; i < lines.length; i++) {
    if (anchorOf(lines[i]!) === ref.hash) {
      candidates.push(i + 1);
    }
  }
  if (candidates.length === 0) {
    throw new Error(`anchor ${ref.hash} not found: content changed`);
  }
  if (ref.line === 0) {
    if (candidates.length > 1) {
      throw new Error(`anchor ${ref.hash} is ambiguous; candidates [${candidates.join(' ')}]: multiple matching lines`);
    }
    return resolved(ref, candidates[0]!, lines.length);
  }

  let nearest = candidates[0]!;
  let nearestDistance = distance(nearest, ref.line);
  for (const candidate of candidates.slice(1)) {
    const candidateDistance = distance(candidate, ref.line);
    if (candidateDistance < nearestDistance) {
      nearest = candidate;
      nearestDistance = candidateDistance;
    }
  }
  return resolved(ref, nearest, lines.length);
}

function resolved(ref: AnchorRef, start: number, lineCount: number): AnchorResolution {
  let end = start;
  if (ref.line > 0) {
    end = Math.min(ref.end, lineCount) + start - ref.line;
    end = Math.max(end, start);
    end = Math.min(end, lineCount);
  }
  const moved = ref.line > 0 && start !== ref.line;
  return { start, end, moved, from: moved ? ref.line : 0 };
}

function distance(a: number, b: number): number {
  return Math.abs(a - b);
}
