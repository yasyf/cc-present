import CcPresentKit
import SwiftUI
import WebKit

/// WebViewLoadPhase is the navigation lifecycle a SingleBlockWebView reports through
/// its optional `phase` binding, so a host can overlay a skeleton or a native fallback.
enum WebViewLoadPhase: Equatable {
    case loading
    case loaded
    case failed
}

/// SingleBlockWebView hosts the SPA's single-block page in a WKWebView sized to its
/// content via the `ccPresentHeight` message (KVO fallback on the scroll content size).
/// It carries the app's appearance to the page as a `theme` query param and reloads on
/// a mid-session flip. An optional `phase` binding reports the navigation lifecycle;
/// pack blocks omit it and render exactly as before.
struct SingleBlockWebView: UIViewRepresentable {
    let url: URL
    @Binding var height: CGFloat
    var phase: Binding<WebViewLoadPhase>?
    @Environment(\.colorScheme) private var colorScheme

    private var themedURL: URL {
        url.appendingTheme(dark: colorScheme == .dark)
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(height: $height, phase: phase)
    }

    func makeUIView(context: Context) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        let controller = WKUserContentController()
        controller.add(context.coordinator, name: "ccPresentHeight")
        configuration.userContentController = controller

        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.navigationDelegate = context.coordinator
        webView.scrollView.isScrollEnabled = false
        webView.scrollView.bounces = false
        webView.isOpaque = false
        webView.backgroundColor = .clear
        webView.scrollView.backgroundColor = .clear
        context.coordinator.observe(webView)
        context.coordinator.load(webView, url: themedURL)
        return webView
    }

    func updateUIView(_ webView: WKWebView, context: Context) {
        context.coordinator.load(webView, url: themedURL)
    }

    static func dismantleUIView(_ webView: WKWebView, coordinator: Coordinator) {
        coordinator.tearDown(webView)
    }

    @MainActor
    final class Coordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
        private let height: Binding<CGFloat>
        private let phase: Binding<WebViewLoadPhase>?
        private var observation: NSKeyValueObservation?
        private var loadedURL: URL?

        init(height: Binding<CGFloat>, phase: Binding<WebViewLoadPhase>?) {
            self.height = height
            self.phase = phase
        }

        /// load points the web view at `url`, skipping the load when it already holds
        /// that URL so a SwiftUI update (a height change) never re-fetches — only an
        /// appearance flip, which flips the `theme` query, reloads.
        func load(_ webView: WKWebView, url: URL) {
            guard loadedURL != url else { return }
            loadedURL = url
            webView.load(URLRequest(url: url))
        }

        func observe(_ webView: WKWebView) {
            observation = webView.scrollView.observe(\.contentSize, options: [.new]) { [weak self] _, change in
                guard let contentHeight = change.newValue?.height else { return }
                MainActor.assumeIsolated {
                    self?.apply(contentHeight)
                }
            }
        }

        func tearDown(_ webView: WKWebView) {
            observation?.invalidate()
            observation = nil
            webView.configuration.userContentController.removeScriptMessageHandler(forName: "ccPresentHeight")
        }

        func userContentController(_: WKUserContentController, didReceive message: WKScriptMessage) {
            guard let px = Self.height(fromMessageBody: message.body) else { return }
            apply(px)
        }

        func webView(_: WKWebView, didFinish _: WKNavigation!) {
            phase?.wrappedValue = .loaded
        }

        func webView(_: WKWebView, didFail _: WKNavigation!, withError _: Error) {
            phase?.wrappedValue = .failed
        }

        func webView(_: WKWebView, didFailProvisionalNavigation _: WKNavigation!, withError _: Error) {
            phase?.wrappedValue = .failed
        }

        private func apply(_ newHeight: CGFloat) {
            guard let next = Self.clampedHeight(current: height.wrappedValue, proposed: newHeight) else { return }
            height.wrappedValue = next
        }

        /// height(fromMessageBody:) reads the `px` number a `ccPresentHeight` frame
        /// carries, or nil for a body outside the expected `{px: Number}` shape.
        nonisolated static func height(fromMessageBody body: Any) -> CGFloat? {
            guard let payload = body as? [String: Any], let px = payload["px"] as? NSNumber else { return nil }
            return CGFloat(truncating: px)
        }

        /// clampedHeight is the height to apply, or nil when the proposal is
        /// non-positive or within a point of the current — sub-point churn is dropped.
        nonisolated static func clampedHeight(current: CGFloat, proposed: CGFloat) -> CGFloat? {
            guard proposed > 0, abs(proposed - current) >= 1 else { return nil }
            return proposed
        }
    }
}
