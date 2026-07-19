import CcPresentKit
import SwiftUI

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

/// PackBlockWebView renders a plugin-supplied pack block through a SingleBlockWebView
/// sized to its content. Height-sync, theming, and the disabled inner scroll all live
/// in the shared host; the pack block just points it at its single-block page.
struct PackBlockWebView: View {
    let block: Block.Pack
    let context: PackContext

    @State private var height: CGFloat = 140

    var body: some View {
        SingleBlockWebView(url: context.singleBlockURL(blockId: block.id), height: $height)
            .frame(height: height)
            .frame(maxWidth: .infinity)
    }
}
