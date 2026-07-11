import CcPresentKit
import SwiftUI
import WebKit

/// PackContext is what a pack block's WKWebView needs to load its single-block page:
/// the machine's base URL, the optional bearer token, and the live board subject. It
/// is built once from the board's machine and store, never from a history round's
/// seeded fake subject.
struct PackContext {
    let baseURL: URL
    let token: String?
    let subject: String

    /// singleBlockURL is the SPA single-block route for one pack block —
    /// `/p/<subject>?block=<blockId>[&token=<token>]` (contract #7) — with the
    /// subject and query values percent-encoded by Foundation.
    func singleBlockURL(blockId: String) -> URL {
        var items = [URLQueryItem(name: "block", value: blockId)]
        if let token {
            items.append(URLQueryItem(name: "token", value: token))
        }
        return baseURL
            .appending(path: "p")
            .appending(component: subject)
            .appending(queryItems: items)
    }
}

/// PackBlockWebView renders a plugin-supplied pack block by loading the SPA's
/// single-block page in a WKWebView sized to its content. The page reports its height
/// through the `ccPresentHeight` message handler (its ResizeObserver), with a KVO
/// fallback on the scroll view's content size; the inner scroll view is disabled so
/// the block flows inline with the board. Every WKWebView shares the default web
/// content process pool automatically on this deployment target.
struct PackBlockWebView: View {
    let block: Block.Pack
    let context: PackContext

    @State private var height: CGFloat = 140

    var body: some View {
        PackWebView(url: context.singleBlockURL(blockId: block.id), height: $height)
            .frame(height: height)
            .frame(maxWidth: .infinity)
    }
}

private struct PackWebView: UIViewRepresentable {
    let url: URL
    @Binding var height: CGFloat

    func makeCoordinator() -> Coordinator {
        Coordinator(height: $height)
    }

    func makeUIView(context: Context) -> WKWebView {
        let configuration = WKWebViewConfiguration()
        let controller = WKUserContentController()
        controller.add(context.coordinator, name: "ccPresentHeight")
        configuration.userContentController = controller

        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.scrollView.isScrollEnabled = false
        webView.scrollView.bounces = false
        webView.isOpaque = false
        webView.backgroundColor = .clear
        webView.scrollView.backgroundColor = .clear
        context.coordinator.observe(webView)
        webView.load(URLRequest(url: url))
        return webView
    }

    func updateUIView(_: WKWebView, context _: Context) {}

    static func dismantleUIView(_ webView: WKWebView, coordinator: Coordinator) {
        coordinator.tearDown(webView)
    }

    @MainActor
    final class Coordinator: NSObject, WKScriptMessageHandler {
        private let height: Binding<CGFloat>
        private var observation: NSKeyValueObservation?

        init(height: Binding<CGFloat>) {
            self.height = height
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
            guard let body = message.body as? [String: Any], let px = body["px"] as? NSNumber else { return }
            apply(CGFloat(truncating: px))
        }

        private func apply(_ newHeight: CGFloat) {
            guard newHeight > 0, abs(newHeight - height.wrappedValue) >= 1 else { return }
            height.wrappedValue = newHeight
        }
    }
}
