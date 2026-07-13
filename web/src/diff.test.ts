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
  it('parses one hunk with its heading and new-file path', () => {
    const hunks = parseDiff(SAMPLE);
    expect(hunks).toHaveLength(1);
    expect(hunks[0]?.heading).toBe('def greet(name):');
    expect(hunks[0]?.path).toBe('greet.py');
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

describe('parseDiff file-path capture', () => {
  it('captures each file path across a multi-file diff and keeps headers out of rows', () => {
    const multi = `diff --git a/a.ts b/a.ts
index 1..2 100644
--- a/a.ts
+++ b/a.ts
@@ -1,2 +1,2 @@
-const a = 1;
+const a = 2;
 export default a;
diff --git a/b.py b/b.py
index 3..4 100644
--- a/b.py
+++ b/b.py
@@ -1 +1 @@
-x = 1
+x = 2`;
    const hunks = parseDiff(multi);
    expect(hunks).toHaveLength(2);
    expect(hunks[0]?.path).toBe('a.ts');
    expect(hunks[1]?.path).toBe('b.py');
    // the second file's headers must not leak into the first hunk's rows
    const first = hunks[0]?.rows ?? [];
    expect(first).toHaveLength(3);
    expect(first.some((r) => r.text.includes('diff --git'))).toBe(false);
    expect(first.some((r) => r.text.includes('b/b.py'))).toBe(false);
  });

  it('infers a deleted file from its old path when the new path is /dev/null', () => {
    const deleted = `diff --git a/gone.go b/gone.go
deleted file mode 100644
index 5..0
--- a/gone.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package main
-// bye`;
    const hunks = parseDiff(deleted);
    expect(hunks).toHaveLength(1);
    expect(hunks[0]?.path).toBe('gone.go');
    expect(hunks[0]?.rows.map((r) => r.kind)).toEqual(['del', 'del']);
  });

  it('strips surrounding quotes and the a// b/ prefix from a quoted path', () => {
    const quoted = `diff --git "a/caf\\303\\251.py" "b/caf\\303\\251.py"
index 9..a 100644
--- "a/caf\\303\\251.py"
+++ "b/caf\\303\\251.py"
@@ -1 +1 @@
-x = 1
+x = 2`;
    // git quotes and octal-escapes non-ASCII bytes; the parser drops only the
    // surrounding quotes and prefix, leaving the extension inferable.
    expect(parseDiff(quoted)[0]?.path).toBe('caf\\303\\251.py');
  });

  it('captures the rename target as the new-file path', () => {
    const renamed = `diff --git a/old.ts b/new.ts
similarity index 90%
rename from old.ts
rename to new.ts
index 7..8 100644
--- a/old.ts
+++ b/new.ts
@@ -1 +1 @@
-const x = 1;
+const x = 2;`;
    const hunks = parseDiff(renamed);
    expect(hunks[0]?.path).toBe('new.ts');
  });

  it('handles a tab-suffixed +++ header', () => {
    const tabbed = `--- a/x.rs\t2024-01-01\n+++ b/x.rs\t2024-01-02\n@@ -1 +1 @@\n-a\n+b`;
    expect(parseDiff(tabbed)[0]?.path).toBe('x.rs');
  });
});
