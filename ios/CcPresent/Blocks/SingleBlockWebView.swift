import CcPresentKit
import SwiftUI
import UIKit
import WebKit

/// WebViewLoadPhase is the navigation lifecycle a SingleBlockWebView reports through
/// its optional `phase` binding, so a host can overlay a skeleton or a native fallback.
enum WebViewLoadPhase: Equatable {
    case loading
    case loaded
    case failed
}

/// WebBlockPresentation is the content a webview-backed block shows for a given
/// board-context availability and load phase — the pure mapping the per-view
/// fallback tests pin. Shared by every block that embeds the SPA single-block page.
enum WebBlockPresentation: Equatable {
    case webView(showingSkeleton: Bool)
    case rawSource

    static func of(hasContext: Bool, phase: WebViewLoadPhase) -> WebBlockPresentation {
        guard hasContext else { return .rawSource }
        switch phase {
        case .loading: return .webView(showingSkeleton: true)
        case .loaded: return .webView(showingSkeleton: false)
        case .failed: return .rawSource
        }
    }

    /// showsNativeTitle is true only in the native fallback: in webview mode the embedded
    /// page draws the block title, so a native heading above it would double the title.
    var showsNativeTitle: Bool {
        self == .rawSource
    }
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
        webView.uiDelegate = context.coordinator
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
    final class Coordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate, WKUIDelegate {
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

        /// A `target="_blank"` link (a record's external links) asks the page to open a
        /// window; the single-block page never hosts one, so route an http(s) target to the
        /// system browser and return nil so no orphan web view is created.
        func webView(_: WKWebView, createWebViewWith _: WKWebViewConfiguration, for navigationAction: WKNavigationAction, windowFeatures _: WKWindowFeatures) -> WKWebView? {
            if let url = Self.externalURL(for: navigationAction.request) {
                UIApplication.shared.open(url)
            }
            return nil
        }

        /// externalURL is the http(s) target a new-window navigation hands to the system
        /// browser, or nil for a scheme that should not escape the webview.
        nonisolated static func externalURL(for request: URLRequest) -> URL? {
            guard let url = request.url, let scheme = url.scheme?.lowercased(),
                  scheme == "http" || scheme == "https" else { return nil }
            return url
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
