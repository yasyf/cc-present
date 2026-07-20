import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { anchorOf, formatAnchor, formatRangeAnchor, parseAnchor, resolveAnchor } from './anchor';

const corpusPath = fileURLToPath(new URL('../../internal/anchor/testdata/anchors.json', import.meta.url));

interface HashCase {
  line: string;
  hash: string;
}

type ParseCase =
  | { ref: string; line: number; end: number; hash: string }
  | { ref: string; error: true };

type ResolveCase =
  | { ref: string; lines: string[]; start: number; end: number; moved: boolean; from?: number }
  | { ref: string; lines: string[]; error: string };

interface AnchorCorpus {
  hash: HashCase[];
  parse: ParseCase[];
  resolve: ResolveCase[];
}

const corpus = JSON.parse(readFileSync(corpusPath, 'utf8')) as AnchorCorpus;

describe('anchor conformance corpus', () => {
  describe('hash', () => {
    for (const [index, test] of corpus.hash.entries()) {
      it(`case ${index}: ${JSON.stringify(test.line)}`, () => {
        expect(anchorOf(test.line)).toBe(test.hash);
      });
    }
  });

  describe('parse', () => {
    for (const test of corpus.parse) {
      it(test.ref, () => {
        if ('error' in test) {
          expect(() => parseAnchor(test.ref)).toThrow(Error);
          return;
        }
        expect(parseAnchor(test.ref)).toEqual({ line: test.line, end: test.end, hash: test.hash });
      });
    }
  });

  describe('resolve', () => {
    for (const [index, test] of corpus.resolve.entries()) {
      it(`case ${index}: ${test.ref}`, () => {
        const ref = parseAnchor(test.ref);
        if ('error' in test) {
          expect(() => resolveAnchor(ref, test.lines)).toThrow(test.error);
          return;
        }
        expect(resolveAnchor(ref, test.lines)).toEqual({
          start: test.start,
          end: test.end,
          moved: test.moved,
          from: test.from ?? 0,
        });
      });
    }
  });
});

describe('anchor formatting', () => {
  it('formats single-line and ranged anchors', () => {
    expect(formatAnchor(12, 'xrkm')).toBe('12#xrkm');
    expect(formatRangeAnchor(2, 4, 'tj58')).toBe('2-4#tj58');
  });
});

it('does not trim U+FEFF before hashing', () => {
  expect(anchorOf('\ufeffreturn nil')).not.toBe(anchorOf('return nil'));
});
