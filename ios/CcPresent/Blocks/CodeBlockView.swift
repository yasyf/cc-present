import CcPresentKit
import SwiftUI

/// CodeHighlighter turns a code block's source into styled text for the active color
/// scheme; conformers are injected into `CodeBlockView` to slot in without touching it.
@MainActor
protocol CodeHighlighter {
    func highlight(_ code: String, language: String, colorScheme: ColorScheme) -> AttributedString
}

/// PlainCodeHighlighter renders the source verbatim with no coloring — the preview and
/// test fallback. The monospaced font is applied by the view, not here.
struct PlainCodeHighlighter: CodeHighlighter {
    func highlight(_ code: String, language _: String, colorScheme _: ColorScheme) -> AttributedString {
        AttributedString(code)
    }
}

/// HighlightrCodeHighlighter colors a code block through the CcPresentKit
/// `CodeSyntaxHighlighter` (highlight.js via Highlightr); an unknown language renders
/// plain, matching PlainCodeHighlighter.
struct HighlightrCodeHighlighter: CodeHighlighter {
    func highlight(_ code: String, language: String, colorScheme: ColorScheme) -> AttributedString {
        CodeSyntaxHighlighter.shared.highlight(
            code,
            language: language,
            scheme: colorScheme == .dark ? .dark : .light
        )
    }
}

/// CodeBlockView renders a code block in a horizontally scrollable monospaced panel,
/// tagged with a language chip. Highlighting is a seam: the default colors the source
/// through Highlightr; inject a `CodeHighlighter` to override it.
struct CodeBlockView: View {
    let block: Block.Code
    var highlighter: CodeHighlighter = HighlightrCodeHighlighter()

    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            header
            codePanel
        }
    }

    @ViewBuilder
    private var header: some View {
        if !block.lang.isEmpty || !(block.title ?? "").isEmpty {
            HStack(spacing: 8) {
                if !block.lang.isEmpty {
                    Text(block.lang.uppercased())
                        .font(.system(size: 10, weight: .semibold, design: .monospaced))
                        .tracking(1)
                        .foregroundStyle(BlockPalette.muted)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: 2))
                }
                if let title = block.title, !title.isEmpty {
                    Text(title)
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(BlockPalette.muted)
                        .lineLimit(1)
                }
            }
        }
    }

    private var codePanel: some View {
        ScrollView(.horizontal, showsIndicators: true) {
            Text(highlighter.highlight(block.code, language: block.lang, colorScheme: colorScheme))
                .font(.system(size: 13, design: .monospaced))
                .textSelection(.enabled)
                .fixedSize(horizontal: true, vertical: false)
                .padding(.horizontal, 14)
                .padding(.vertical, 10)
        }
        .background(BlockPalette.monoBg)
        .clipShape(RoundedRectangle(cornerRadius: 4))
        .overlay(
            RoundedRectangle(cornerRadius: 4).strokeBorder(BlockPalette.line)
        )
    }
}

#Preview("Code block") {
    ScrollView {
        VStack(alignment: .leading, spacing: 24) {
            CodeBlockView(
                block: Block.Code(
                    id: "code-swift",
                    lang: "swift",
                    code: """
                    func reduce(_ events: [Event]) -> BoardState {
                        events.reduce(into: BoardState()) { state, event in
                            state.apply(event)  // this line is deliberately long enough to force a horizontal scroll
                        }
                    }
                    """,
                    title: "Reduce.swift"
                )
            )

            CodeBlockView(
                block: Block.Code(
                    id: "code-json",
                    lang: "json",
                    code: "{\n  \"version\": 1,\n  \"title\": \"Opener board\",\n  \"blocks\": []\n}"
                )
            )
        }
        .padding()
    }
}
