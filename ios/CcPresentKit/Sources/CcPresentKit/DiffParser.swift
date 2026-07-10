// A hand-rolled unified-diff parser. It turns unified diff text into hunks of
// rows carrying old/new line numbers and a kind, which a two-gutter table
// renders. It reads the hunk headers for line numbering and ignores the file
// headers (diff/index/---/+++) that precede the first hunk. Ported field for
// field from web/src/diff.ts.

import Foundation

/// DiffRowKind marks how a row relates the two sides: an addition, a deletion,
/// an unchanged context line, or a `\ No newline` meta marker.
public enum DiffRowKind: String, Equatable, Sendable {
    case add
    case del
    case context
    case meta
}

/// DiffRow is one line of a hunk: its kind, the old- and new-side line numbers
/// (nil on the side where the line is absent), and its text.
public struct DiffRow: Equatable, Sendable {
    public var kind: DiffRowKind
    public var oldNo: Int?
    public var newNo: Int?
    public var text: String

    public init(kind: DiffRowKind, oldNo: Int?, newNo: Int?, text: String) {
        self.kind = kind
        self.oldNo = oldNo
        self.newNo = newNo
        self.text = text
    }
}

/// DiffHunk is one contiguous change region: the heading trailing its `@@`
/// header plus the rows that follow until the next header.
public struct DiffHunk: Equatable, Sendable {
    public var heading: String
    public var rows: [DiffRow]

    public init(heading: String, rows: [DiffRow]) {
        self.heading = heading
        self.rows = rows
    }
}

/// parseDiff turns unified diff text into hunks. Lines before the first `@@`
/// header are dropped as file headers; input with no header yields no hunks.
public func parseDiff(_ diff: String) -> [DiffHunk] {
    let hunkHeader = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$/
    var hunks: [DiffHunk] = []
    var oldNo = 0
    var newNo = 0

    for rawLine in diff.split(separator: "\n", omittingEmptySubsequences: false) {
        let line = String(rawLine)
        if let header = line.wholeMatch(of: hunkHeader) {
            guard let old = Int(header.1), let new = Int(header.2) else { continue }
            oldNo = old
            newNo = new
            let heading = header.3.trimmingCharacters(in: .whitespacesAndNewlines)
            hunks.append(DiffHunk(heading: heading, rows: []))
            continue
        }
        if hunks.isEmpty {
            continue
        } // file headers before the first hunk

        let text = String(line.dropFirst())
        switch line.first {
        case "+"?:
            hunks[hunks.count - 1].rows.append(DiffRow(kind: .add, oldNo: nil, newNo: newNo, text: text))
            newNo += 1
        case "-"?:
            hunks[hunks.count - 1].rows.append(DiffRow(kind: .del, oldNo: oldNo, newNo: nil, text: text))
            oldNo += 1
        case "\\"?:
            hunks[hunks.count - 1].rows.append(DiffRow(kind: .meta, oldNo: nil, newNo: nil, text: line))
        default:
            hunks[hunks.count - 1].rows.append(DiffRow(kind: .context, oldNo: oldNo, newNo: newNo, text: text))
            oldNo += 1
            newNo += 1
        }
    }

    return hunks
}
