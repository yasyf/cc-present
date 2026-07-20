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

    @Test("stripAnsi removes colon-form (truecolor) SGR")
    func stripsColonSgr() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("\(esc)[38:2::255:0:0mred\(esc)[0m") == "red")
    }

    @Test("stripAnsi strips an OSC-8 hyperlink but keeps its visible text")
    func stripsOsc8KeepingText() {
        let esc = "\u{001B}"
        let input = "\(esc)]8;;https://example.com\(esc)\\Link text\(esc)]8;;\(esc)\\"
        #expect(TermBlockView.stripAnsi(input) == "Link text")
    }

    @Test("stripAnsi strips a BEL-terminated OSC window-title sequence")
    func stripsOscBel() {
        let esc = "\u{001B}"
        let bel = "\u{0007}"
        #expect(TermBlockView.stripAnsi("\(esc)]0;my title\(bel)rest") == "rest")
    }

    @Test("stripAnsi strips a lone ESC charset-select sequence")
    func stripsLoneEsc() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("\(esc)(Bplain") == "plain")
    }

    @Test("the native title shows only in the fallback, never over the webview")
    func nativeTitleOnlyInFallback() {
        #expect(WebBlockPresentation.rawSource.showsNativeTitle)
        #expect(!WebBlockPresentation.webView(showingSkeleton: true).showsNativeTitle)
        #expect(!WebBlockPresentation.webView(showingSkeleton: false).showsNativeTitle)
    }
}
