import CcPresentKit
import SwiftUI
import UIKit

/// ImageBlockView renders an image block: the async-loaded image with an optional
/// caption. `src` is an `https:`, `data:`, or `asset:<sha256>` URI. Asset
/// references resolve through the paired machine with a bearer-authorized request,
/// which SwiftUI's AsyncImage cannot issue, so the load runs through
/// AuthorizedImageLoader instead.
struct ImageBlockView: View {
    let block: Block.Image
    let client: APIClient?

    @State private var phase: AuthorizedImageLoader.Phase = .loading

    init(block: Block.Image, client: APIClient? = nil) {
        self.block = block
        self.client = client
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            content
            if let caption = block.caption, !caption.isEmpty {
                Text(caption)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
        .task(id: block.src) {
            phase = await AuthorizedImageLoader.shared.load(src: block.src, client: client)
        }
    }

    @ViewBuilder private var content: some View {
        switch phase {
        case .loading:
            placeholder { ProgressView() }
        case let .image(image):
            image
                .resizable()
                .scaledToFit()
                .frame(maxWidth: .infinity)
                .clipShape(RoundedRectangle(cornerRadius: Metrics.radiusLg))
                .overlay {
                    RoundedRectangle(cornerRadius: Metrics.radiusLg)
                        .strokeBorder(Color(.separator))
                }
                .accessibilityLabel(block.alt)
        case .failure:
            placeholder {
                Label("Image unavailable", systemImage: "photo")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func placeholder(@ViewBuilder _ label: () -> some View) -> some View {
        RoundedRectangle(cornerRadius: Metrics.radiusLg)
            .fill(Color(.secondarySystemFill))
            .frame(maxWidth: .infinity, minHeight: 140)
            .overlay(label())
    }
}

/// AuthorizedImageLoader fetches image bytes for a block `src` and caches the
/// decoded image in memory. `asset:` URIs go through APIClient.assetRequest so the
/// bearer header rides along; `https:` URIs load directly and `data:` URIs decode
/// their inline payload without touching the network.
@MainActor
final class AuthorizedImageLoader {
    /// Phase is the render state a view drives its content from.
    enum Phase {
        case loading
        case image(Image)
        case failure
    }

    static let shared = AuthorizedImageLoader()

    private var cache: [String: UIImage] = [:]

    func load(src: String, client: APIClient?) async -> Phase {
        if let cached = cache[src] {
            return .image(Image(uiImage: cached))
        }
        guard let data = await fetch(src: src, client: client), let image = UIImage(data: data) else {
            return .failure
        }
        cache[src] = image
        return .image(Image(uiImage: image))
    }

    private func fetch(src: String, client: APIClient?) async -> Data? {
        if src.hasPrefix("data:") {
            return Self.decodeDataURI(src)
        }
        let request: URLRequest
        if src.hasPrefix("asset:") {
            guard let client else { return nil }
            request = client.assetRequest(sha: String(src.dropFirst("asset:".count)))
        } else if let url = URL(string: src) {
            request = URLRequest(url: url)
        } else {
            return nil
        }
        do {
            let (data, response) = try await URLSession.shared.data(for: request)
            guard let http = response as? HTTPURLResponse, (200 ..< 300).contains(http.statusCode) else {
                return nil
            }
            return data
        } catch {
            return nil
        }
    }

    private static func decodeDataURI(_ src: String) -> Data? {
        guard let comma = src.firstIndex(of: ",") else { return nil }
        let meta = src[src.index(src.startIndex, offsetBy: "data:".count) ..< comma]
        let payload = String(src[src.index(after: comma)...])
        if meta.contains(";base64") {
            return Data(base64Encoded: payload)
        }
        return payload.removingPercentEncoding?.data(using: .utf8)
    }
}

private let previewPixelPNG =
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

#Preview("Data URI + caption") {
    ImageBlockView(
        block: Block.Image(
            id: "img1",
            src: "data:image/png;base64,\(previewPixelPNG)",
            alt: "A single teal pixel",
            caption: "Figure 1 — a data-URI image with a caption."
        )
    )
    .padding()
}

#Preview("Unresolvable asset") {
    ImageBlockView(
        block: Block.Image(
            id: "img2",
            src: "asset:0000000000000000000000000000000000000000000000000000000000000000",
            alt: "Missing asset",
            caption: nil
        )
    )
    .padding()
}
