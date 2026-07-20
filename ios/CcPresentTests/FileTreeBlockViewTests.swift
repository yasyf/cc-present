@testable import CcPresentApp
import CcPresentKit
import Testing

@Suite("FileTreeBlockView fallback state")
@MainActor
struct FileTreeBlockViewTests {
    @Test("with board context, loading shows the webview under a skeleton")
    func loadingShowsSkeletonOverWebView() {
        #expect(
            WebBlockPresentation.of(hasContext: true, phase: .loading)
                == .webView(showingSkeleton: true)
        )
    }

    @Test("with board context, a loaded page shows the webview without a skeleton")
    func loadedShowsWebViewOnly() {
        #expect(
            WebBlockPresentation.of(hasContext: true, phase: .loaded)
                == .webView(showingSkeleton: false)
        )
    }

    @Test("with board context, a load failure falls back to the native listing")
    func failedFallsBackToListing() {
        #expect(WebBlockPresentation.of(hasContext: true, phase: .failed) == .rawSource)
    }

    @Test("without board context, every phase renders the native listing")
    func noContextAlwaysRendersListing() {
        for phase in [WebViewLoadPhase.loading, .loaded, .failed] {
            #expect(WebBlockPresentation.of(hasContext: false, phase: phase) == .rawSource)
        }
    }

    @Test("badge glyphs mark the git-status change kind, blank when absent")
    func badgeGlyphs() {
        #expect(FileTreeBlockView.badgeGlyph("added") == "+")
        #expect(FileTreeBlockView.badgeGlyph("modified") == "~")
        #expect(FileTreeBlockView.badgeGlyph("removed") == "\u{2212}")
        #expect(FileTreeBlockView.badgeGlyph(nil) == " ")
    }

    @Test("rows fold paths into a tree: directories first, then files, lexicographic")
    func rowsOrderTree() {
        let entries = [
            Block.TreeEntry(path: "z.ts"),
            Block.TreeEntry(path: "beta/x.ts"),
            Block.TreeEntry(path: "m.ts"),
            Block.TreeEntry(path: "alpha/y.ts"),
        ]
        let lines = FileTreeBlockView.rows(from: entries).map(FileTreeBlockView.line(for:))
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
        let lines = FileTreeBlockView.rows(from: entries).map(FileTreeBlockView.line(for:))
        #expect(lines == [
            "  src/",
            "    deep/",
            "      c.ts",
            "    a.ts",
            "    b.ts",
        ])
    }

    @Test("file lines carry the badge glyph and an optional trailing note")
    func fileLineBadgesAndNotes() {
        let entries = [
            Block.TreeEntry(path: "gone.ts", badge: "removed", note: "superseded"),
            Block.TreeEntry(path: "new.ts", badge: "added"),
        ]
        let lines = FileTreeBlockView.rows(from: entries).map(FileTreeBlockView.line(for:))
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
        #expect(rows.map(FileTreeBlockView.line(for:)) == [
            "  a/",
            "  + b",
            "\u{2212} a",
        ])
        // Distinct identities keep ForEach from collapsing the two same-named nodes.
        #expect(Set(rows.map(\.id)).count == rows.count)
    }

    @Test("the native title shows only in the fallback, never over the webview")
    func nativeTitleOnlyInFallback() {
        #expect(WebBlockPresentation.rawSource.showsNativeTitle)
        #expect(!WebBlockPresentation.webView(showingSkeleton: true).showsNativeTitle)
        #expect(!WebBlockPresentation.webView(showingSkeleton: false).showsNativeTitle)
    }
}
