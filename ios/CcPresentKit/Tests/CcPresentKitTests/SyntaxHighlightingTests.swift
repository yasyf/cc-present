@testable import CcPresentKit
import Foundation
import Testing

@MainActor
@Suite("Syntax highlighting")
struct SyntaxHighlightingTests {
    @Test("a known language yields multiple styled runs over the same characters")
    func knownLanguageHighlights() {
        let highlighter = CodeSyntaxHighlighter(scheme: .light)
        let source = "let x = 42 // note"
        let result = highlighter.highlight(source, language: "swift", scheme: .light)

        #expect(String(result.characters) == source)
        #expect(result.runs.count > 1)
    }

    @Test("an uppercase language name still resolves its grammar")
    func languageNameIsCaseInsensitive() {
        let highlighter = CodeSyntaxHighlighter(scheme: .light)
        let result = highlighter.highlight("let x = 42 // note", language: "SWIFT", scheme: .light)

        #expect(result.runs.count > 1)
    }

    @Test("an unknown language falls back to a single plain run")
    func unknownLanguageIsPlain() {
        let highlighter = CodeSyntaxHighlighter(scheme: .light)
        let source = "zzz yyy xxx"
        let result = highlighter.highlight(source, language: "not-a-real-language", scheme: .light)

        #expect(String(result.characters) == source)
        #expect(result.runs.count == 1)
    }

    @Test("an empty language falls back to a single plain run")
    func emptyLanguageIsPlain() {
        let highlighter = CodeSyntaxHighlighter(scheme: .light)
        let source = "plain text"
        let result = highlighter.highlight(source, language: "", scheme: .light)

        #expect(String(result.characters) == source)
        #expect(result.runs.count == 1)
    }

    @Test("re-theming to dark still preserves the source characters")
    func rethemeToDarkPreservesCharacters() {
        let highlighter = CodeSyntaxHighlighter(scheme: .light)
        let source = "let x = 42 // note"
        let result = highlighter.highlight(source, language: "swift", scheme: .dark)

        #expect(String(result.characters) == source)
        #expect(result.runs.count > 1)
    }

    @Test("theme name maps by color scheme")
    func themeNameMapsByScheme() {
        #expect(CodeSyntaxHighlighter.themeName(for: .light) == "atom-one-light")
        #expect(CodeSyntaxHighlighter.themeName(for: .dark) == "atom-one-dark")
    }
}
