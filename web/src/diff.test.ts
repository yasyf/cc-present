import { describe, expect, it } from 'vitest';
import { parseDiff } from './diff';

const SAMPLE = `diff --git a/greet.py b/greet.py
index 111..222 100644
--- a/greet.py
+++ b/greet.py
@@ -1,4 +1,4 @@ def greet(name):
 def greet(name):
-    print("hi")
+    print("hello")
+    print("!")
 return None`;

describe('parseDiff', () => {
  it('parses one hunk with its heading', () => {
    const hunks = parseDiff(SAMPLE);
    expect(hunks).toHaveLength(1);
    expect(hunks[0]?.heading).toBe('def greet(name):');
  });

  it('classifies rows and numbers both gutters', () => {
    const rows = parseDiff(SAMPLE)[0]?.rows ?? [];
    expect(rows.map((r) => r.kind)).toEqual(['context', 'del', 'add', 'add', 'context']);

    // context: def greet(name):
    expect(rows[0]).toMatchObject({ kind: 'context', oldNo: 1, newNo: 1 });
    // del: print("hi") — old side only
    expect(rows[1]).toMatchObject({ kind: 'del', oldNo: 2, newNo: null, text: '    print("hi")' });
    // add: print("hello") — new side only
    expect(rows[2]).toMatchObject({ kind: 'add', oldNo: null, newNo: 2, text: '    print("hello")' });
    // add: print("!")
    expect(rows[3]).toMatchObject({ kind: 'add', oldNo: null, newNo: 3 });
    // context after the changes: old advanced past one deletion, new past two additions
    expect(rows[4]).toMatchObject({ kind: 'context', oldNo: 3, newNo: 4, text: 'return None' });
  });

  it('ignores file headers before the first hunk', () => {
    const hunks = parseDiff(SAMPLE);
    const texts = hunks.flatMap((h) => h.rows.map((r) => r.text));
    expect(texts.some((t) => t.includes('diff --git'))).toBe(false);
    expect(texts.some((t) => t.includes('index 111'))).toBe(false);
  });

  it('captures a no-newline marker as a meta row', () => {
    const rows = parseDiff(`@@ -1 +1 @@\n-a\n+b\n\\ No newline at end of file`)[0]?.rows ?? [];
    expect(rows.at(-1)).toMatchObject({ kind: 'meta', oldNo: null, newNo: null });
  });
});
