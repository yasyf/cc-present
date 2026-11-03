@testable import CcPresentUI
import Testing

@Suite("TermBlockView output")
struct TermBlockViewTests {
    @Test("stripAnsi removes SGR color codes from the fallback output")
    func stripsSgrColors() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("\(esc)[32mgreen\(esc)[0m ok") == "green ok")
    }

    @Test("stripAnsi leaves plain output untouched")
    func leavesPlainOutput() {
        #expect(TermBlockView.stripAnsi("plain output line") == "plain output line")
    }

    @Test("stripAnsi removes cursor and erase CSI sequences")
    func stripsCursorSequences() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("a\(esc)[2K\(esc)[1Ab") == "ab")
    }

    @Test("stripAnsi removes colon-form (truecolor) SGR")
    func stripsColonSgr() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("\(esc)[38:2::255:0:0mred\(esc)[0m") == "red")
    }

    @Test("stripAnsi strips an OSC-8 hyperlink but keeps its visible text")
    func stripsOsc8KeepingText() {
        let esc = "\u{001B}"
        let input = "\(esc)]8;;https://example.com\(esc)\\Link text\(esc)]8;;\(esc)\\"
        #expect(TermBlockView.stripAnsi(input) == "Link text")
    }

    @Test("stripAnsi strips a BEL-terminated OSC window-title sequence")
    func stripsOscBel() {
        let esc = "\u{001B}"
        let bel = "\u{0007}"
        #expect(TermBlockView.stripAnsi("\(esc)]0;my title\(bel)rest") == "rest")
    }

    @Test("stripAnsi strips a lone ESC charset-select sequence")
    func stripsLoneEsc() {
        let esc = "\u{001B}"
        #expect(TermBlockView.stripAnsi("\(esc)(Bplain") == "plain")
    }

    @Test("SGR runs carry color and weight, and a reset returns to plain")
    func runsCarryColorAndWeight() {
        let esc = "\u{001B}"
        let runs = TermBlockView.runs("\(esc)[1;32mok\(esc)[0m done")
        #expect(runs == [
            TermBlockView.TermRun(text: "ok", style: TermBlockView.TermStyle(color: .basic(2), bold: true)),
            TermBlockView.TermRun(text: " done", style: TermBlockView.TermStyle()),
        ])
    }

    @Test("bright, 256-color, and truecolor foregrounds resolve")
    func extendedColors() {
        let plain = TermBlockView.TermStyle()
        #expect(TermBlockView.apply("91", to: plain).color == .basic(1))
        #expect(TermBlockView.apply("38;5;196", to: plain).color == .rgb(255, 0, 0))
        #expect(TermBlockView.apply("38;5;244", to: plain).color == .rgb(128, 128, 128))
        #expect(TermBlockView.apply("38;2;10;20;30", to: plain).color == .rgb(10, 20, 30))
        #expect(TermBlockView.apply("39", to: TermBlockView.TermStyle(color: .basic(3))).color == nil)
    }

    @Test("a background code consumes its arguments without touching the foreground")
    func backgroundSkipsArguments() {
        let style = TermBlockView.apply("48;5;21;31", to: TermBlockView.TermStyle())
        #expect(style.color == .basic(1))
    }

    @Test("non-SGR escapes are stripped from the runs")
    func runsDropOtherEscapes() {
        let esc = "\u{001B}"
        #expect(TermBlockView.runs("a\(esc)[2K\(esc)[1Ab").map(\.text) == ["ab"])
    }

    @Test("an empty output yields no runs")
    func emptyOutput() {
        #expect(TermBlockView.runs("").isEmpty)
    }
}
