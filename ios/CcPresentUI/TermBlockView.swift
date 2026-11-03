import CcPresentKit
import IOSKit
import IOSKitUI
import SwiftUI
import UIKit

/// TermBlockView renders a term block natively: an optional title and a copy control,
/// the command behind a prompt glyph, and the captured output with its ANSI colors
/// and weights. Copy always yields the stripped output, matching web TermView.
struct TermBlockView: View {
    let block: Block.Term

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            if let command = block.command, !command.isEmpty {
                HStack(alignment: .firstTextBaseline, spacing: Metrics.space2) {
                    Text("❯")
                        .foregroundStyle(Palette.accentInk)
                    Text(command)
                        .foregroundStyle(Palette.ink)
                        .textSelection(.enabled)
                }
                .voice(.mono, size: 13)
                .padding(.horizontal, 14)
                .padding(.vertical, Metrics.space2)
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(alignment: .bottom) { Palette.line.frame(height: 1) }
            }
            ScrollView(.horizontal) {
                Text(Self.attributed(block.output))
                    .voice(.mono, size: 13)
                    .foregroundStyle(Palette.ink)
                    .textSelection(.enabled)
                    .fixedSize(horizontal: true, vertical: false)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 10)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Palette.monoBg)
        .clipShape(RoundedRectangle(cornerRadius: Metrics.radiusMd))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusMd).strokeBorder(Palette.line))
    }

    private var header: some View {
        HStack(spacing: Metrics.space2) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.mono, size: 11)
                    .foregroundStyle(Palette.muted)
                    .lineLimit(1)
            }
            Spacer(minLength: 0)
            Button {
                UIPasteboard.general.string = Self.stripAnsi(block.output)
            } label: {
                Label("Copy", systemImage: "doc.on.doc")
                    .labelStyle(.iconOnly)
            }
            .buttonStyle(GhostButtonStyle(tint: Palette.muted))
            .accessibilityLabel("Copy output")
        }
        .padding(.leading, 14)
        .overlay(alignment: .bottom) { Palette.line.frame(height: 1) }
    }

    static func attributed(_ output: String) -> AttributedString {
        var out = AttributedString()
        for run in runs(output) {
            var piece = AttributedString(run.text)
            if let color = run.style.color {
                piece.foregroundColor = color.swiftUI
            }
            if run.style.bold {
                piece.inlinePresentationIntent = .stronglyEmphasized
            }
            if run.style.italic {
                piece.inlinePresentationIntent = (piece.inlinePresentationIntent ?? []).union(.emphasized)
            }
            if run.style.underline {
                piece.underlineStyle = .single
            }
            if run.style.dim {
                piece.foregroundColor = (run.style.color?.swiftUI ?? Palette.ink).opacity(0.6)
            }
            out += piece
        }
        return out
    }

    struct TermStyle: Equatable {
        var color: TermColor?
        var bold = false
        var dim = false
        var italic = false
        var underline = false
    }

    enum TermColor: Equatable {
        case basic(Int)
        case rgb(Int, Int, Int)

        var swiftUI: Color {
            switch self {
            case let .basic(index):
                Self.basicColors[index % 8]
            case let .rgb(red, green, blue):
                Color(red: Double(red) / 255, green: Double(green) / 255, blue: Double(blue) / 255)
            }
        }

        private static let basicColors: [Color] = [
            Palette.muted, Palette.reject, Palette.approve, Palette.warn,
            Palette.accentInk, .purple, .teal, Palette.ink,
        ]

        static func indexed(_ index: Int) -> TermColor {
            switch index {
            case 0 ..< 16:
                return .basic(index % 8)
            case 16 ..< 232:
                let cube = index - 16
                let level = { (step: Int) in step == 0 ? 0 : 55 + step * 40 }
                return .rgb(level(cube / 36), level(cube / 6 % 6), level(cube % 6))
            default:
                let gray = 8 + (index - 232) * 10
                return .rgb(gray, gray, gray)
            }
        }
    }

    struct TermRun: Equatable {
        let text: String
        let style: TermStyle
    }

    private static let sgr = /\u{001B}\[([0-9;:]*)m/

    static func runs(_ output: String) -> [TermRun] {
        var runs: [TermRun] = []
        var style = TermStyle()
        var rest = output[...]
        while let match = rest.firstMatch(of: sgr) {
            append(String(rest[..<match.range.lowerBound]), style, to: &runs)
            style = apply(String(match.output.1), to: style)
            rest = rest[match.range.upperBound...]
        }
        append(String(rest), style, to: &runs)
        return runs
    }

    private static func append(_ raw: String, _ style: TermStyle, to runs: inout [TermRun]) {
        let text = stripAnsi(raw)
        guard !text.isEmpty else { return }
        if let last = runs.last, last.style == style {
            runs[runs.count - 1] = TermRun(text: last.text + text, style: style)
        } else {
            runs.append(TermRun(text: text, style: style))
        }
    }

    static func apply(_ params: String, to current: TermStyle) -> TermStyle {
        var style = current
        let codes = params.isEmpty ? [0] : params.split(separator: ";", omittingEmptySubsequences: false).map { Int($0) ?? -1 }
        var index = 0
        while index < codes.count {
            let code = codes[index]
            switch code {
            case 0: style = TermStyle()
            case 1: style.bold = true
            case 2: style.dim = true
            case 3: style.italic = true
            case 4: style.underline = true
            case 22: style.bold = false; style.dim = false
            case 23: style.italic = false
            case 24: style.underline = false
            case 30 ... 37: style.color = .basic(code - 30)
            case 90 ... 97: style.color = .basic(code - 90)
            case 39: style.color = nil
            case 38:
                if index + 2 < codes.count, codes[index + 1] == 5 {
                    style.color = .indexed(codes[index + 2])
                    index += 2
                } else if index + 4 < codes.count, codes[index + 1] == 2 {
                    style.color = .rgb(codes[index + 2], codes[index + 3], codes[index + 4])
                    index += 4
                }
            case 48:
                index += index + 1 < codes.count && codes[index + 1] == 5 ? 2 : 4
            default: break
            }
            index += 1
        }
        return style
    }

    private static let ansiEscape =
        /\u{001B}\][^\u{0007}\u{001B}]*(?:\u{0007}|\u{001B}\\)|\u{001B}\[[\u{0030}-\u{003F}]*[\u{0020}-\u{002F}]*[\u{0040}-\u{007E}]|\u{001B}[\u{0020}-\u{002F}]*[\u{0030}-\u{007E}]/

    /// stripAnsi drops every ANSI escape a captured stream carries — SGR color runs
    /// (semicolon or colon form), cursor moves, erases, and OSC strings (an OSC-8
    /// hyperlink's visible text is kept) — leaving the plain text a terminal renders.
    static func stripAnsi(_ text: String) -> String {
        text.replacing(ansiEscape, with: "")
    }
}
