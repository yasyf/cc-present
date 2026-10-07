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
/// tagged with a language chip. A block grounded in a file adds a source header, a
/// numbered gutter, highlighted rows, and numbered pins. Highlighting is a seam.
struct CodeBlockView: View {
    let block: Block.Code
    var highlighter: CodeHighlighter = HighlightrCodeHighlighter()

    @Environment(\.colorScheme) private var colorScheme

    private var highlighted: AttributedString {
        highlighter.highlight(block.code, language: block.lang, colorScheme: colorScheme)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            header
            if isGrounded(block) {
                groundedPanel
                pinList
            } else {
                codePanel
            }
        }
    }

    @ViewBuilder
    private var header: some View {
        let source = sourceLabel(block)
        if !block.lang.isEmpty || !(block.title ?? "").isEmpty || source != nil {
            HStack(spacing: 8) {
                if !block.lang.isEmpty {
                    Text(block.lang.uppercased())
                        .voice(.mono, size: 10, weight: .semibold)
                        .tracking(1)
                        .foregroundStyle(BlockPalette.muted)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(BlockPalette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusSm))
                }
                if let title = block.title, !title.isEmpty {
                    Text(title)
                        .voice(.mono, size: 11)
                        .foregroundStyle(BlockPalette.muted)
                        .lineLimit(1)
                }
                if let source {
                    Text(source)
                        .voice(.mono, size: 11)
                        .foregroundStyle(BlockPalette.was)
                        .lineLimit(1)
                        .truncationMode(.middle)
                }
            }
        }
    }

    private var codePanel: some View {
        ScrollView(.horizontal, showsIndicators: true) {
            Text(highlighted)
                .voice(.mono, size: 13)
                .textSelection(.enabled)
                .fixedSize(horizontal: true, vertical: false)
                .padding(.horizontal, 14)
                .padding(.vertical, 10)
        }
        .panelChrome()
    }

    private var groundedPanel: some View {
        let lines = splitLines(highlighted)
        let start = block.start ?? 1
        let lit = litRows(block.highlight, start: start, count: lines.count)
        let marks = pinNumbers(block.pins ?? [])
        let gutterDigits = String(start + lines.count - 1).count
        return ScrollView(.horizontal, showsIndicators: true) {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(Array(lines.enumerated()), id: \.offset) { offset, line in
                    let number = start + offset
                    HStack(alignment: .firstTextBaseline, spacing: Metrics.space3) {
                        Text(String(repeating: " ", count: gutterDigits - String(number).count) + String(number))
                            .foregroundStyle(BlockPalette.was)
                            .accessibilityHidden(true)
                        Text(line)
                            .fixedSize(horizontal: true, vertical: false)
                        ForEach(marks[number] ?? [], id: \.self) { no in
                            PinMark(number: no, tone: block.pins?[no - 1].tone)
                        }
                        Spacer(minLength: 0)
                    }
                    .padding(.horizontal, 14)
                    .padding(.vertical, 1)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(lit[offset] ? BlockPalette.accentInk.opacity(0.12) : .clear)
                }
            }
            .voice(.mono, size: 13)
            .textSelection(.enabled)
            .fixedSize(horizontal: true, vertical: false)
            .padding(.vertical, 10)
        }
        .panelChrome()
    }

    @ViewBuilder
    private var pinList: some View {
        if let pins = block.pins, !pins.isEmpty {
            VStack(alignment: .leading, spacing: Metrics.space1) {
                ForEach(Array(pins.enumerated()), id: \.offset) { offset, pin in
                    HStack(alignment: .firstTextBaseline, spacing: Metrics.space2) {
                        PinMark(number: offset + 1, tone: pin.tone)
                        Text("L\(pin.line)")
                            .voice(.mono, size: 10)
                            .foregroundStyle(BlockPalette.muted)
                        Text(pin.title)
                            .font(.subheadline.weight(.semibold))
                            .foregroundStyle(BlockPalette.ink)
                        if let body = pin.body, !body.isEmpty {
                            Text(body)
                                .font(.subheadline)
                                .foregroundStyle(BlockPalette.muted)
                        }
                    }
                }
            }
        }
    }
}

/// PinMark is a pin's numbered disc, filled with its tone's signal color or the accent.
private struct PinMark: View {
    let number: Int
    let tone: String?

    var body: some View {
        Text("\(number)")
            .voice(.prose, size: 10, weight: .semibold)
            .monospacedDigit()
            .foregroundStyle(BlockPalette.accentFg)
            .padding(.horizontal, 4)
            .frame(minWidth: 16, minHeight: 16)
            .background(BlockPalette.factTone(tone, fallback: BlockPalette.accentInk), in: Capsule())
    }
}

private extension View {
    func panelChrome() -> some View {
        background(BlockPalette.monoBg)
            .clipShape(RoundedRectangle(cornerRadius: Metrics.radiusMd))
            .overlay(
                RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(BlockPalette.line)
            )
    }
}

/// isGrounded reports a block read from a file — one with `src`, a `start`, or pins —
/// which renders numbered rows. Mirrors web/src/components/Code.tsx.
func isGrounded(_ block: Block.Code) -> Bool {
    !(block.src ?? "").isEmpty || (block.start ?? 0) != 0 || !(block.pins ?? []).isEmpty
}

/// sourceLabel is the grounded header's `path:lines @ sha`, nil without a `src`.
func sourceLabel(_ block: Block.Code) -> String? {
    guard let src = block.src, !src.isEmpty else { return nil }
    let lines = (block.lines ?? "").isEmpty ? "" : ":\(block.lines ?? "")"
    let sha = (block.sha ?? "").isEmpty ? "" : " @ \(block.sha ?? "")"
    return src + lines + sha
}

/// litRows marks each of `count` rows numbered from `start` that falls in a `highlight`
/// range list such as `3,7-9`.
func litRows(_ highlight: String?, start: Int, count: Int) -> [Bool] {
    let ranges = (highlight ?? "").split(separator: ",").compactMap { range -> ClosedRange<Int>? in
        let bounds = range.split(separator: "-", omittingEmptySubsequences: false).map { Int($0) }
        guard let low = bounds.first ?? nil, let high = bounds.count > 1 ? bounds[1] : low, low <= high else {
            return nil
        }
        return low ... high
    }
    return (0 ..< count).map { offset in ranges.contains { $0.contains(start + offset) } }
}

/// pinNumbers maps each file line to the 1-based numbers of the pins on it.
func pinNumbers(_ pins: [Block.CodePin]) -> [Int: [Int]] {
    var out: [Int: [Int]] = [:]
    for (offset, pin) in pins.enumerated() {
        out[pin.line, default: []].append(offset + 1)
    }
    return out
}

/// splitLines cuts highlighted source at each LF scalar, as web's `split('\n')` does, so
/// CRLF source keeps its row count; a row's trailing CR is dropped from display.
func splitLines(_ text: AttributedString) -> [AttributedString] {
    let scalars = text.unicodeScalars
    var lines: [AttributedString] = []
    var lineStart = text.startIndex
    var index = text.startIndex
    while index < text.endIndex {
        let next = scalars.index(after: index)
        if scalars[index] == "\n" {
            lines.append(row(text, from: lineStart, to: index))
            lineStart = next
        }
        index = next
    }
    lines.append(row(text, from: lineStart, to: text.endIndex))
    return lines
}

private func row(_ text: AttributedString, from start: AttributedString.Index, to end: AttributedString.Index) -> AttributedString {
    guard start < end, text.unicodeScalars[text.unicodeScalars.index(before: end)] == "\r" else {
        return AttributedString(text[start ..< end])
    }
    return AttributedString(text[start ..< text.unicodeScalars.index(before: end)])
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
                    id: "code-grounded",
                    lang: "go",
                    code: "func reduce() {\n\tmerge(viewed)\n\tcloseRound()\n}",
                    src: "internal/state/reduce.go",
                    lines: "480-483",
                    start: 480,
                    highlight: "481",
                    pins: [
                        Block.CodePin(line: 481, title: "Merge viewed ids"),
                        Block.CodePin(line: 482, title: "Snapshot, then empty", body: "round-scoped", tone: "warn"),
                    ],
                    sha: "e634c50"
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
