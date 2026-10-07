@testable import CcPresentApp
import CcPresentKit
import Foundation
import Testing

@Suite("Grounded code block")
struct CodeBlockViewTests {
    private let grounded = Block.Code(
        id: "g",
        lang: "go",
        code: "a\nb\nc",
        src: "internal/state/reduce.go",
        lines: "40-42",
        start: 40,
        highlight: "41",
        pins: [
            Block.CodePin(line: 41, title: "Merge"),
            Block.CodePin(line: 41, title: "Again", tone: "bad"),
            Block.CodePin(line: 42, title: "Close"),
        ],
        sha: "abc1234"
    )

    @Test("a block with src, start, or pins is grounded; plain code is not")
    func groundedClassification() {
        #expect(isGrounded(grounded))
        #expect(isGrounded(Block.Code(id: "s", lang: "go", code: "x", start: 3)))
        #expect(isGrounded(Block.Code(id: "p", lang: "go", code: "x", pins: [Block.CodePin(line: 1, title: "t")])))
        #expect(!isGrounded(Block.Code(id: "c", lang: "go", code: "x", title: "T")))
    }

    @Test("the source label reads path:lines @ sha, dropping absent parts")
    func sourceLabelFormats() {
        #expect(sourceLabel(grounded) == "internal/state/reduce.go:40-42 @ abc1234")
        #expect(sourceLabel(Block.Code(id: "s", lang: "go", code: "x", src: "a.go")) == "a.go")
        #expect(sourceLabel(Block.Code(id: "c", lang: "go", code: "x")) == nil)
    }

    @Test("highlight ranges light rows by file line number")
    func litRowsByFileLine() {
        #expect(litRows("41", start: 40, count: 3) == [false, true, false])
        #expect(litRows("40,42-43", start: 40, count: 3) == [true, false, true])
        #expect(litRows("7-", start: 1, count: 8) == Array(repeating: false, count: 8))
        #expect(litRows(nil, start: 1, count: 2) == [false, false])
    }

    @Test("pins number in authored order and group by file line")
    func pinNumbersGroupByLine() {
        #expect(pinNumbers(grounded.pins ?? []) == [41: [1, 2], 42: [3]])
    }

    @Test("splitting highlighted source yields one row per line, keeping empty lines")
    func splitLinesKeepsEveryRow() {
        let lines = splitLines(AttributedString("a\n\nc\n"))
        #expect(lines.map { String($0.characters) } == ["a", "", "c", ""])
    }
}
