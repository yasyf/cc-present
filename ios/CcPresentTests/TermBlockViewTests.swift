@testable import CcPresentApp
import Testing

@Suite("TermBlockView fallback state")
@MainActor
struct TermBlockViewTests {
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

    @Test("with board context, a load failure falls back to the native panel")
    func failedFallsBackToFallback() {
        #expect(WebBlockPresentation.of(hasContext: true, phase: .failed) == .rawSource)
    }

    @Test("without board context, every phase renders the native fallback")
    func noContextAlwaysRendersFallback() {
        for phase in [WebViewLoadPhase.loading, .loaded, .failed] {
            #expect(WebBlockPresentation.of(hasContext: false, phase: phase) == .rawSource)
        }
    }

    @Test("stripAnsi removes SGR color codes from the fallback output")
    func stripsSgrColors() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("\(esc)[32mgreen\(esc)[0m ok") == "green ok")
    }

    @Test("stripAnsi leaves plain output untouched")
    func leavesPlainOutput() {
        #expect(TermBlockView.stripAnsi("plain output line") == "plain output line")
    }

    @Test("stripAnsi removes cursor and erase CSI sequences")
    func stripsCursorSequences() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("a\(esc)[2K\(esc)[1Ab") == "ab")
    }
}
