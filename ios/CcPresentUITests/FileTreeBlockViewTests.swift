@testable import CcPresentUI
import CcPresentKit
import Testing

private func outline(_ rows: [FileTreeBlockView.FileTreeRow]) -> [String] {
    rows.map { row in
        let indent = String(repeating: "  ", count: row.depth)
        if row.isDirectory {
            return "\(indent)  \(row.name)/"
        }
        let glyph = switch row.badge {
        case "added": "+"
        case "modified": "~"
        case "removed": "\u{2212}"
        default: " "
        }
        return "\(indent)\(glyph) \(row.name)\(row.note.map { "  # \($0)" } ?? "")"
    }
}

@Suite("FileTreeBlockView tree")
@MainActor
struct FileTreeBlockViewTests {
    @Test("rows fold paths into a tree: directories first, then files, lexicographic")
    func rowsOrderTree() {
        let entries = [
            Block.TreeEntry(path: "z.ts"),
            Block.TreeEntry(path: "beta/x.ts"),
            Block.TreeEntry(path: "m.ts"),
            Block.TreeEntry(path: "alpha/y.ts"),
        ]
        let lines = outline(FileTreeBlockView.rows(from: entries))
        #expect(lines == [
            "  alpha/",
            "    y.ts",
            "  beta/",
            "    x.ts",
            "  m.ts",
            "  z.ts",
        ])
    }

    @Test("shared directory prefixes collapse into one implicit directory node")
    func rowsCollapseImplicitDirs() {
        let entries = [
            Block.TreeEntry(path: "src/a.ts"),
            Block.TreeEntry(path: "src/deep/c.ts"),
            Block.TreeEntry(path: "src/b.ts"),
        ]
        let lines = outline(FileTreeBlockView.rows(from: entries))
        #expect(lines == [
            "  src/",
            "    deep/",
            "      c.ts",
            "    a.ts",
            "    b.ts",
        ])
    }

    @Test("file rows carry their badge and note")
    func fileLineBadgesAndNotes() {
        let entries = [
            Block.TreeEntry(path: "gone.ts", badge: "removed", note: "superseded"),
            Block.TreeEntry(path: "new.ts", badge: "added"),
        ]
        let lines = outline(FileTreeBlockView.rows(from: entries))
        #expect(lines == [
            "\u{2212} gone.ts  # superseded",
            "+ new.ts",
        ])
    }

    @Test("a removed file and a same-named added directory stay distinct rows")
    func fileAndDirSameNameDistinctRows() {
        let entries = [
            Block.TreeEntry(path: "a", badge: "removed"),
            Block.TreeEntry(path: "a/b", badge: "added"),
        ]
        let rows = FileTreeBlockView.rows(from: entries)
        // The implicit "a/" directory, "b" under it, and the removed "a" file all coexist.
        #expect(outline(rows) == [
            "  a/",
            "  + b",
            "\u{2212} a",
        ])
        // Distinct identities keep ForEach from collapsing the two same-named nodes.
        #expect(Set(rows.map(\.id)).count == rows.count)
    }

    @Test("directory rows carry their full path")
    func directoryPaths() {
        let rows = FileTreeBlockView.rows(from: [Block.TreeEntry(path: "src/deep/c.ts")])
        #expect(rows.map(\.path) == ["src", "src/deep", "src/deep/c.ts"])
    }

    @Test("collapsing a directory hides everything beneath it and nothing beside it")
    func collapseHidesSubtree() {
        let rows = FileTreeBlockView.rows(from: [
            Block.TreeEntry(path: "src/a.ts"),
            Block.TreeEntry(path: "src/deep/c.ts"),
            Block.TreeEntry(path: "z.ts"),
        ])
        #expect(outline(FileTreeBlockView.visibleRows(rows, collapsed: ["src/deep"])) == [
            "  src/",
            "    deep/",
            "    a.ts",
            "  z.ts",
        ])
        #expect(outline(FileTreeBlockView.visibleRows(rows, collapsed: ["src"])) == [
            "  src/",
            "  z.ts",
        ])
        #expect(FileTreeBlockView.visibleRows(rows, collapsed: []) == rows)
    }
}
