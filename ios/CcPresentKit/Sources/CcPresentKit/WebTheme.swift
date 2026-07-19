import Foundation

public extension URL {
    /// appendingTheme adds the `theme` query item — `"dark"` or `"light"` — that the
    /// SPA's single-block page reads before first render, so a WKWebView renders in the
    /// host app's appearance instead of resolving the page's `light-dark()` to light.
    func appendingTheme(dark: Bool) -> URL {
        appending(queryItems: [URLQueryItem(name: "theme", value: dark ? "dark" : "light")])
    }
}
