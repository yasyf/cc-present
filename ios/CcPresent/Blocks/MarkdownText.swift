import CcPresentKit
import MarkdownUI
import SwiftUI

/// MarkdownText is the shared markdown primitive. It renders a markdown string in
/// one of three styles: `plain` prose, `clamped` (line-limited to 10 lines with a
/// "Show more" affordance when it overflows, mirroring web/src/components/Clamped.tsx),
/// or `struck` (the muted, strikethrough "was:" treatment, always unclamped).
struct MarkdownText: View {
    /// Style selects how the markdown is presented.
    enum Style {
        case plain
        case clamped
        case struck
    }

    let markdown: String
    var style: Style = .plain

    init(_ markdown: String, style: Style = .plain) {
        self.markdown = markdown
        self.style = style
    }

    var body: some View {
        switch style {
        case .plain:
            Markdown(markdown)
        case .clamped:
            ClampedMarkdown(markdown: markdown, lineLimit: 10)
        case .struck:
            Markdown(markdown)
                .markdownTextStyle {
                    ForegroundColor(BlockPalette.was)
                    StrikethroughStyle(.init(pattern: .solid, color: nil))
                }
        }
    }
}

/// ClampedMarkdown caps its markdown at `lineLimit` lines while collapsed, fading
/// the trailing edge and offering a toggle when the content overflows. Expansion
/// state resets whenever the markdown changes, matching the web Clamped component.
private struct ClampedMarkdown: View {
    let markdown: String
    let lineLimit: Int

    @State private var expanded = false
    @State private var contentHeight: CGFloat = 0

    private var clampHeight: CGFloat {
        UIFont.preferredFont(forTextStyle: .body).lineHeight * CGFloat(lineLimit)
    }

    private var overflowing: Bool {
        contentHeight > clampHeight + 1
    }

    private var collapsed: Bool {
        !expanded
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Markdown(markdown)
                .frame(maxHeight: collapsed ? clampHeight : nil, alignment: .top)
                .clipped()
                .mask(alignment: .top) {
                    if collapsed, overflowing {
                        LinearGradient(
                            stops: [
                                .init(color: .black, location: 0),
                                .init(color: .black, location: 0.65),
                                .init(color: .clear, location: 1),
                            ],
                            startPoint: .top,
                            endPoint: .bottom
                        )
                    } else {
                        Rectangle()
                    }
                }
                .background(alignment: .top) {
                    Markdown(markdown)
                        .fixedSize(horizontal: false, vertical: true)
                        .hidden()
                        .background {
                            GeometryReader { proxy in
                                Color.clear.preference(key: MarkdownHeightKey.self, value: proxy.size.height)
                            }
                        }
                }
                .onPreferenceChange(MarkdownHeightKey.self) { height in
                    contentHeight = height
                }

            if overflowing {
                Button(expanded ? "Show less" : "Show more") {
                    withAnimation(.easeInOut(duration: 0.15)) { expanded.toggle() }
                }
                .buttonStyle(.plain)
                .font(.system(size: 12, weight: .semibold, design: .monospaced))
                .foregroundStyle(BlockPalette.accentInk)
            }
        }
        .onChange(of: markdown) { _, _ in
            expanded = false
        }
    }
}

private struct MarkdownHeightKey: PreferenceKey {
    static let defaultValue: CGFloat = 0

    static func reduce(value: inout CGFloat, nextValue: () -> CGFloat) {
        value = max(value, nextValue())
    }
}

/// BlockPalette maps the domain.css `light-dark()` tokens the block renderers share
/// onto theme-aware SwiftUI colors, so native blocks match the web renderer.
enum BlockPalette {
    static let ink = Color(hexLight: 0x23262F, hexDark: 0xE8ECE9)
    static let monoBg = Color(hexLight: 0xF1EDE6, hexDark: 0x0C110F)
    static let chipBg = Color(hexLight: 0xEFEADF, hexDark: 0x1C231F)
    static let line = Color(hexLight: 0xE3DED4, hexDark: 0x26302C)
    static let borderStrong = Color(hexLight: 0xCDC7BA, hexDark: 0x3A453F)
    static let muted = Color(hexLight: 0x5F6672, hexDark: 0x94A09B)
    static let was = Color(hexLight: 0x9AA1AB, hexDark: 0x6C737E)
    static let approve = Color(hexLight: 0x1C7C40, hexDark: 0x58C07A)
    static let reject = Color(hexLight: 0xB3382C, hexDark: 0xE57B6D)
    static let accentInk = Color(hexLight: 0x0C6A77, hexDark: 0x4FD6C4)
}

extension Color {
    init(hexLight: UInt32, hexDark: UInt32) {
        self.init(uiColor: UIColor { traits in
            UIColor(rgb: traits.userInterfaceStyle == .dark ? hexDark : hexLight)
        })
    }
}

private extension UIColor {
    convenience init(rgb: UInt32) {
        self.init(
            red: Double((rgb >> 16) & 0xFF) / 255,
            green: Double((rgb >> 8) & 0xFF) / 255,
            blue: Double(rgb & 0xFF) / 255,
            alpha: 1
        )
    }
}

#Preview("Markdown styles") {
    ScrollView {
        VStack(alignment: .leading, spacing: 24) {
            MarkdownText("A **plain** paragraph with `inline code`, a [link](https://example.com), and _emphasis_.")

            MarkdownText(
                """
                ## Clamped block

                This block runs long enough to overflow the ten-line clamp so the fade and
                the *Show more* toggle appear.

                - First consideration worth reading
                - Second consideration
                - Third consideration
                - Fourth consideration
                - Fifth consideration
                - Sixth consideration
                - Seventh consideration
                - Eighth consideration
                - Ninth consideration
                - Tenth consideration
                - Eleventh consideration, past the clamp
                """,
                style: .clamped
            )

            MarkdownText("~~This whole block wears the was: treatment~~ — muted and struck.", style: .struck)
        }
        .padding()
    }
}
