@testable import CcPresentKit
import Testing

@Suite("DiffParser")
struct DiffParserTests {
    struct Case {
        let name: String
        let input: String
        let want: [DiffHunk]
    }

    static let cases: [Case] = [
        Case(
            name: "normal hunk with context, delete, and add",
            input: "@@ -1,3 +1,3 @@ heading\n ctx\n-old\n+new\n ctx2",
            want: [
                DiffHunk(heading: "heading", rows: [
                    DiffRow(kind: .context, oldNo: 1, newNo: 1, text: "ctx"),
                    DiffRow(kind: .del, oldNo: 2, newNo: nil, text: "old"),
                    DiffRow(kind: .add, oldNo: nil, newNo: 2, text: "new"),
                    DiffRow(kind: .context, oldNo: 3, newNo: 3, text: "ctx2"),
                ]),
            ]
        ),
        Case(
            name: "multiple hunks each restart numbering from their header",
            input: "@@ -1,1 +1,1 @@\n-a\n+b\n@@ -10,2 +10,2 @@ second\n c\n+d",
            want: [
                DiffHunk(heading: "", rows: [
                    DiffRow(kind: .del, oldNo: 1, newNo: nil, text: "a"),
                    DiffRow(kind: .add, oldNo: nil, newNo: 1, text: "b"),
                ]),
                DiffHunk(heading: "second", rows: [
                    DiffRow(kind: .context, oldNo: 10, newNo: 10, text: "c"),
                    DiffRow(kind: .add, oldNo: nil, newNo: 11, text: "d"),
                ]),
            ]
        ),
        Case(
            name: "add-only new file, with ---/+++ file headers dropped before the hunk",
            input: "--- /dev/null\n+++ b/new.txt\n@@ -0,0 +1,2 @@\n+line1\n+line2",
            want: [
                DiffHunk(heading: "", rows: [
                    DiffRow(kind: .add, oldNo: nil, newNo: 1, text: "line1"),
                    DiffRow(kind: .add, oldNo: nil, newNo: 2, text: "line2"),
                ]),
            ]
        ),
        Case(
            name: "remove-only advances only the old-side counter",
            input: "@@ -1,2 +0,0 @@\n-gone1\n-gone2",
            want: [
                DiffHunk(heading: "", rows: [
                    DiffRow(kind: .del, oldNo: 1, newNo: nil, text: "gone1"),
                    DiffRow(kind: .del, oldNo: 2, newNo: nil, text: "gone2"),
                ]),
            ]
        ),
        Case(
            name: "no-newline meta marker keeps the whole line as text and moves no counter",
            input: "@@ -1,1 +1,1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file",
            want: [
                DiffHunk(heading: "", rows: [
                    DiffRow(kind: .del, oldNo: 1, newNo: nil, text: "old"),
                    DiffRow(kind: .meta, oldNo: nil, newNo: nil, text: "\\ No newline at end of file"),
                    DiffRow(kind: .add, oldNo: nil, newNo: 1, text: "new"),
                    DiffRow(kind: .meta, oldNo: nil, newNo: nil, text: "\\ No newline at end of file"),
                ]),
            ]
        ),
        Case(
            name: "header without line counts and a heading that gets trimmed",
            input: "@@ -5 +7 @@ ctx-fn\n line",
            want: [
                DiffHunk(heading: "ctx-fn", rows: [
                    DiffRow(kind: .context, oldNo: 5, newNo: 7, text: "line"),
                ]),
            ]
        ),
        Case(
            name: "garbage input with no header yields no hunks",
            input: "this is not a diff at all\njust some random text\n{}",
            want: []
        ),
        Case(
            name: "empty input yields no hunks",
            input: "",
            want: []
        ),
        Case(
            name: "trailing newline emits a trailing empty context row",
            input: "@@ -1,1 +1,1 @@\n ctx\n",
            want: [
                DiffHunk(heading: "", rows: [
                    DiffRow(kind: .context, oldNo: 1, newNo: 1, text: "ctx"),
                    DiffRow(kind: .context, oldNo: 2, newNo: 2, text: ""),
                ]),
            ]
        ),
    ]

    @Test("parseDiff matches the TypeScript parser behavior", arguments: cases)
    func parses(_ testCase: Case) {
        #expect(parseDiff(testCase.input) == testCase.want, "case: \(testCase.name)")
    }
}
