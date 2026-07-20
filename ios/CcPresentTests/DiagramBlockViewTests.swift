@testable import CcPresentApp
import Testing

@Suite("DiagramBlockView fallback state")
struct DiagramBlockViewTests {
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

    @Test("with board context, a load failure falls back to the raw source")
    func failedFallsBackToSource() {
        #expect(WebBlockPresentation.of(hasContext: true, phase: .failed) == .rawSource)
    }

    @Test("without board context, every phase renders the raw source")
    func noContextAlwaysRendersSource() {
        for phase in [WebViewLoadPhase.loading, .loaded, .failed] {
            #expect(WebBlockPresentation.of(hasContext: false, phase: phase) == .rawSource)
        }
    }

    @Test("the native title shows only in the fallback, never over the webview")
    func nativeTitleOnlyInFallback() {
        #expect(WebBlockPresentation.rawSource.showsNativeTitle)
        #expect(!WebBlockPresentation.webView(showingSkeleton: true).showsNativeTitle)
        #expect(!WebBlockPresentation.webView(showingSkeleton: false).showsNativeTitle)
    }
}
