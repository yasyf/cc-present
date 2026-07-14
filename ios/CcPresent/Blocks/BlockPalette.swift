import SwiftUI

/// BlockPalette maps the shared "Blue Pencil" design tokens — the `light-dark()`
/// pairs the web renderer typesets with (web/src/styles/tokens.css) — onto
/// theme-aware SwiftUI colors, so native blocks match the web renderer token for
/// token. The names track the domain token aliases: `accentInk` is `--pencil`,
/// `accentFg` is `--accent-fg`, `was` is the struck-ghost `--was`.
enum BlockPalette {
    static let ink = Color(hexLight: 0x1F2430, hexDark: 0xE7EAF1)
    static let monoBg = Color(hexLight: 0xF1F1EF, hexDark: 0x12141A)
    static let chipBg = Color(hexLight: 0xEEEEED, hexDark: 0x23262D)
    static let line = Color(hexLight: 0xDCDDDD, hexDark: 0x34373E)
    static let borderStrong = Color(hexLight: 0xBDBFC1, hexDark: 0x51545B)
    static let muted = Color(hexLight: 0x5C6472, hexDark: 0x98A0AF)
    static let was = Color(hexLight: 0x747B86, hexDark: 0x858C9A)
    static let approve = Color(hexLight: 0x1E7B4F, hexDark: 0x5BC489)
    static let reject = Color(hexLight: 0xBF3B2F, hexDark: 0xEE8273)
    static let warn = Color(hexLight: 0x8F6400, hexDark: 0xDCA847)
    static let accentInk = Color(hexLight: 0x3D56C5, hexDark: 0x91A3F2)
    static let accentFg = Color(hexLight: 0xFFFFFF, hexDark: 0x14161C)
}

private extension Color {
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
