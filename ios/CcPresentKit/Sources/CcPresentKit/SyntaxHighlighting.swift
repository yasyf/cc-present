import Foundation
import Highlightr

/// SyntaxColorScheme selects the light or dark highlight.js theme so highlighting
/// tracks the app appearance.
public enum SyntaxColorScheme: Sendable {
    case light
    case dark
}

/// CodeSyntaxHighlighter renders source into an attributed string with highlight.js
/// grammars via Highlightr; a language Highlightr does not know returns the plain
/// source unchanged. It is main-actor confined — Highlightr wraps a single JSContext —
/// and reused through `shared`.
@MainActor
public final class CodeSyntaxHighlighter {
    public static let shared = CodeSyntaxHighlighter()

    private let highlightr: Highlightr
    private var scheme: SyntaxColorScheme

    public init(scheme: SyntaxColorScheme = .light) {
        guard let highlightr = Highlightr() else {
            fatalError("Highlightr failed to initialize — bundled highlight.js resources are missing")
        }
        self.highlightr = highlightr
        self.scheme = scheme
        highlightr.setTheme(to: Self.themeName(for: scheme))
    }

    /// highlight renders `code` with the grammar for `language`, re-theming first when
    /// `scheme` differs from the last call; an unknown or empty language returns the
    /// plain source. The per-run font is stripped so the host view owns the typeface.
    public func highlight(_ code: String, language: String, scheme: SyntaxColorScheme) -> AttributedString {
        if scheme != self.scheme {
            self.scheme = scheme
            highlightr.setTheme(to: Self.themeName(for: scheme))
        }
        let language = language.lowercased()
        guard !language.isEmpty,
              highlightr.supportedLanguages().contains(language),
              let highlighted = highlightr.highlight(code, as: language, fastRender: true)
        else {
            return AttributedString(code)
        }
        let stripped = NSMutableAttributedString(attributedString: highlighted)
        stripped.removeAttribute(.font, range: NSRange(location: 0, length: stripped.length))
        return AttributedString(stripped)
    }

    static func themeName(for scheme: SyntaxColorScheme) -> String {
        switch scheme {
        case .light: "atom-one-light"
        case .dark: "atom-one-dark"
        }
    }
}
