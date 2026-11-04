import CcPresentKit
import IOSKit
import IOSKitUI
import IOSKitWeb
import SwiftUI

/// WebBlockPresentation is the content a webview-backed block shows for a given
/// board-context availability and load phase — the pure mapping the per-view
/// fallback tests pin. Shared by every block that embeds the SPA single-block page.
enum WebBlockPresentation: Equatable {
    case webView(showingSkeleton: Bool)
    case rawSource

    static func of(hasContext: Bool, phase: WebLoadPhase) -> WebBlockPresentation {
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

/// SingleBlockWebView hosts the SPA's single-block page in a WebHost sized to its
/// content via the `ccPresentHeight` message. `onInteraction` receives the block id
/// of each interaction the page posts to `ccPresentInteraction`.
struct SingleBlockWebView: View {
    let url: URL
    @Binding var height: CGFloat
    var phase: Binding<WebLoadPhase>?
    var onInteraction: (@MainActor (String) -> Void)?

    static let heightMessage = "ccPresentHeight"
    static let interactionMessage = "ccPresentInteraction"

    var body: some View {
        WebHost(url: url, height: $height, phase: phase, heightMessage: Self.heightMessage, handlers: handlers)
    }

    private var handlers: [String: @MainActor (Any) -> Void] {
        guard let onInteraction else { return [:] }
        return [Self.interactionMessage: { body in
            if let blockId = Self.blockId(fromMessageBody: body) {
                onInteraction(blockId)
            }
        }]
    }

    /// blockId(fromMessageBody:) reads the `blockId` a `ccPresentInteraction` frame
    /// carries, or nil for a body outside the expected `{blockId: String}` shape.
    static func blockId(fromMessageBody body: Any) -> String? {
        guard let payload = body as? [String: Any], let blockId = payload["blockId"] as? String else { return nil }
        return blockId
    }
}
