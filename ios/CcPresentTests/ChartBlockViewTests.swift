@testable import CcPresentApp
import CcPresentKit
import Testing

@Suite("ChartBlockView fallback state")
@MainActor
struct ChartBlockViewTests {
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
    func failedFallsBackToPanel() {
        #expect(WebBlockPresentation.of(hasContext: true, phase: .failed) == .rawSource)
    }

    @Test("without board context, every phase renders the native fallback")
    func noContextAlwaysRendersFallback() {
        for phase in [WebViewLoadPhase.loading, .loaded, .failed] {
            #expect(WebBlockPresentation.of(hasContext: false, phase: phase) == .rawSource)
        }
    }

    @Test("the fallback lists a series label with its comma-joined values")
    func fallbackListsSeriesValues() {
        let series = Block.Series(label: "p95", values: [10, 20, 30])
        #expect(ChartBlockView.line(for: series) == "p95: 10.0, 20.0, 30.0")
    }
}
