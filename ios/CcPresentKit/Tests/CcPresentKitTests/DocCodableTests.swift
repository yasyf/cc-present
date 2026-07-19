@testable import CcPresentKit
import Foundation
import Testing

private let encoder = JSONEncoder()
private let decoder = JSONDecoder()

private func roundTrip(_ doc: Doc) throws -> Doc {
    try decoder.decode(Doc.self, from: encoder.encode(doc))
}

private func roundTrip(_ block: Block) throws -> Block {
    try decoder.decode(Block.self, from: encoder.encode(block))
}

/// repoRoot resolves the repository root from this source file's path so the
/// fixture loads regardless of the working directory `swift test` runs in.
private var repoRoot: URL {
    URL(fileURLWithPath: #filePath) // .../ios/CcPresentKit/Tests/CcPresentKitTests/DocCodableTests.swift
        .deletingLastPathComponent() // CcPresentKitTests
        .deletingLastPathComponent() // Tests
        .deletingLastPathComponent() // CcPresentKit
        .deletingLastPathComponent() // ios
        .deletingLastPathComponent() // repo root
}

@Suite("Doc Codable")
struct DocCodableTests {
    @Test("examples/opener-board.json decodes and round-trips exactly")
    func openerBoardRoundTrips() throws {
        let url = repoRoot.appendingPathComponent("examples/opener-board.json")
        let data = try Data(contentsOf: url)
        let doc = try decoder.decode(Doc.self, from: data)

        // Spot-check the decode against the fixture's known values.
        #expect(doc.version == 1)
        #expect(doc.title == "26 openers, redrafted with teeth")
        #expect(doc.stats?.count == 3)
        #expect(doc.stats?.first == Doc.Stat(label: "repos drafted", value: "26"))
        #expect(doc.submit?.label == "Approve openers")
        #expect(doc.blocks.count == 10)

        guard case let .section(firstSection) = doc.blocks[0] else {
            Issue.record("first block is not a section")
            return
        }
        #expect(firstSection.id == "sec-flagged")
        #expect(firstSection.title == "Flagged — corrected before approval")

        guard case let .card(factory) = doc.blocks[1] else {
            Issue.record("second block is not a card")
            return
        }
        #expect(factory.id == "card-cc-factory")
        #expect(factory.flagged == true)
        #expect(factory.status == "redrafted")
        #expect(factory.chips?.count == 3)
        #expect(factory.chips?[1] == Block.Chip(label: "animated", tone: "demo"))
        #expect(factory.children.count == 5)

        // A card's first child is a struck markdown leaf.
        guard case let .markdown(was) = factory.children[0] else {
            Issue.record("first child is not markdown")
            return
        }
        #expect(was.id == "cc-factory-was")
        #expect(was.struck == true)

        // Nested choice with its options survives.
        guard case let .choice(alts) = factory.children[2] else {
            Issue.record("third child is not a choice")
            return
        }
        #expect(alts.options.count == 3)
        #expect(alts.options[0].id == "pick")
        #expect(alts.options[0].hint == "command form")
        #expect(alts.options[0].md == nil)
        #expect(alts.options[0].facts?.count == 2)
        #expect(alts.options[0].facts?.first == Block.Fact(label: "frame", value: "command"))
        #expect(alts.options[0].detail?.pros?.count == 2)

        // Encode → decode → the value is identical.
        let again = try roundTrip(doc)
        #expect(again == doc)
    }

    @Test("every block type round-trips exactly at the top level")
    func everyBlockTypeRoundTrips() throws {
        let allowFeedback = true
        let blocks: [Block] = [
            .section(Block.Section(id: "s", title: "Section", md: "prose")),
            .card(Block.Card(
                id: "c",
                title: "Card",
                summary: "one line",
                chips: [Block.Chip(label: "plugin"), Block.Chip(label: "demo", tone: "demo")],
                flagged: true,
                status: "redrafted",
                children: [
                    .markdown(Block.Markdown(id: "c-md", md: "body", struck: true)),
                    .approval(Block.Approval(id: "c-ap", prompt: "ok?", allowFeedback: allowFeedback)),
                ]
            )),
            .approval(Block.Approval(id: "ap", prompt: "Approve?", allowFeedback: false)),
            .choice(Block.Choice(
                id: "ch",
                prompt: "Pick",
                multi: true,
                options: [
                    Block.Option(id: "o1", label: "One", hint: "first", md: "**one**"),
                    Block.Option(id: "o2", label: "Two"),
                ]
            )),
            .input(Block.Input(id: "in", label: "Notes", placeholder: "type…", multiline: true)),
            .markdown(Block.Markdown(id: "md", md: "hello", struck: nil)),
            .code(Block.Code(id: "co", lang: "bash", code: "echo hi", title: "Run")),
            .diff(Block.Diff(id: "df", diff: "--- a\n+++ b\n", title: "Change")),
            .diagram(Block.Diagram(id: "dg", kind: "mermaid", source: "graph TD; A-->B", title: "Flow")),
            .image(Block.Image(id: "im", src: "https://example.com/x.png", alt: "x", caption: "cap")),
            .table(Block.Table(
                id: "tb",
                columns: [Block.Column(key: "k", label: "K", align: "right"), Block.Column(key: "v", label: "V")],
                rows: [["k": "1", "v": "a"], ["k": "2", "v": "b"]]
            )),
            .progress(Block.Progress(id: "pg", label: "Working", value: 3, max: 10, state: "active")),
        ]

        for block in blocks {
            let again = try roundTrip(block)
            #expect(again == block, "block \(block.id) (\(block.type)) did not round-trip")
        }

        // The whole set inside a document round-trips too.
        let doc = Doc(title: "All blocks", blocks: blocks)
        #expect(try roundTrip(doc) == doc)
    }

    @Test("each block decodes from hand-written JSON into the right case with the right fields")
    func handWrittenPerTypeDecodes() throws {
        func block(_ json: String) throws -> Block {
            try decoder.decode(Block.self, from: Data(json.utf8))
        }

        #expect(try block(#"{"id":"s","type":"section","title":"Header","md":"note"}"#)
            == .section(Block.Section(id: "s", title: "Header", md: "note")))

        #expect(try block(#"""
        {"id":"c","type":"card","title":"T","summary":"sum","flagged":true,"status":"open",
         "chips":[{"label":"x","tone":"flag"}],
         "children":[{"id":"k","type":"input","label":"L"}]}
        """#) == .card(Block.Card(
            id: "c",
            title: "T",
            summary: "sum",
            chips: [Block.Chip(label: "x", tone: "flag")],
            flagged: true,
            status: "open",
            children: [.input(Block.Input(id: "k", label: "L"))]
        )))

        #expect(try block(#"{"id":"a","type":"approval","allowFeedback":false}"#)
            == .approval(Block.Approval(id: "a", allowFeedback: false)))

        #expect(try block(#"""
        {"id":"ch","type":"choice","multi":true,"options":[{"id":"o","label":"O","hint":"h"}]}
        """#) == .choice(Block.Choice(id: "ch", multi: true, options: [Block.Option(id: "o", label: "O", hint: "h")])))

        #expect(try block(#"{"id":"in","type":"input","label":"Name","multiline":true}"#)
            == .input(Block.Input(id: "in", label: "Name", multiline: true)))

        #expect(try block(##"{"id":"m","type":"markdown","md":"# hi","struck":true}"##)
            == .markdown(Block.Markdown(id: "m", md: "# hi", struck: true)))

        #expect(try block(#"{"id":"co","type":"code","lang":"go","code":"x := 1"}"#)
            == .code(Block.Code(id: "co", lang: "go", code: "x := 1")))

        #expect(try block(#"{"id":"d","type":"diff","diff":"-a\n+b","title":"T"}"#)
            == .diff(Block.Diff(id: "d", diff: "-a\n+b", title: "T")))

        #expect(try block(#"{"id":"dg","type":"diagram","kind":"mermaid","source":"graph TD; A-->B"}"#)
            == .diagram(Block.Diagram(id: "dg", kind: "mermaid", source: "graph TD; A-->B")))

        #expect(try block(#"{"id":"i","type":"image","src":"asset:abc","alt":"a"}"#)
            == .image(Block.Image(id: "i", src: "asset:abc", alt: "a")))

        #expect(try block(#"""
        {"id":"t","type":"table","columns":[{"key":"k","label":"K","align":"right"}],"rows":[{"k":"v"}]}
        """#) == .table(Block.Table(
            id: "t",
            columns: [Block.Column(key: "k", label: "K", align: "right")],
            rows: [["k": "v"]]
        )))

        #expect(try block(#"{"id":"p","type":"progress","label":"L","value":2,"max":5,"state":"done"}"#)
            == .progress(Block.Progress(id: "p", label: "L", value: 2, max: 5, state: "done")))
    }

    @Test("an option's recommended flag and each visual kind decode and round-trip")
    func optionRecommendedAndVisualsRoundTrip() throws {
        let choice = Block.choice(Block.Choice(
            id: "ch",
            options: [
                Block.Option(
                    id: "o1",
                    label: "Streaming",
                    recommended: true,
                    visual: .code(Block.Code(id: "v1", lang: "go", code: "stream(ctx)"))
                ),
                Block.Option(
                    id: "o2",
                    label: "Polling",
                    visual: .diagram(Block.Diagram(id: "v2", kind: "mermaid", source: "graph LR; P-->Q"))
                ),
                Block.Option(
                    id: "o3",
                    label: "Image",
                    visual: .image(Block.Image(id: "v3", src: "asset:abc", alt: "shot"))
                ),
                Block.Option(
                    id: "o4",
                    label: "Diff",
                    visual: .diff(Block.Diff(id: "v4", diff: "-a\n+b"))
                ),
            ]
        ))
        #expect(try roundTrip(choice) == choice)

        // A hand-written option carrying recommended + a diagram visual decodes.
        let decoded = try decoder.decode(Block.self, from: Data(#"""
        {"id":"ch","type":"choice","options":[
          {"id":"o1","label":"Streaming","recommended":true,
           "visual":{"id":"v1","type":"diagram","kind":"mermaid","source":"graph TD; A-->B"}}]}
        """#.utf8))
        guard case let .choice(ch) = decoded else {
            Issue.record("decoded block is not a choice")
            return
        }
        #expect(ch.options[0].recommended == true)
        #expect(ch.options[0].visual == .diagram(Block.Diagram(id: "v1", kind: "mermaid", source: "graph TD; A-->B")))
    }

    @Test("an option visual of a disallowed type is a thrown decode error")
    func optionVisualAllowlistThrows() {
        let json = Data(#"""
        {"id":"ch","type":"choice","options":[
          {"id":"o1","label":"A","visual":{"id":"v1","type":"approval"}}]}
        """#.utf8)
        #expect(throws: DecodingError.self) {
            _ = try decoder.decode(Block.self, from: json)
        }
    }

    @Test("the presentation hint decodes to its enum, is nil when absent, and rejects an unknown value")
    func presentationHintDecodes() throws {
        let withHint = try decoder.decode(
            Doc.self,
            from: Data(#"{"version":1,"title":"T","presentation":"board","blocks":[]}"#.utf8)
        )
        #expect(withHint.presentation == .board)

        let without = try decoder.decode(
            Doc.self,
            from: Data(#"{"version":1,"title":"T","blocks":[]}"#.utf8)
        )
        #expect(without.presentation == nil)

        let junk = Data(#"{"version":1,"title":"T","presentation":"carousel","blocks":[]}"#.utf8)
        #expect(throws: DecodingError.self) {
            _ = try decoder.decode(Doc.self, from: junk)
        }
    }

    @Test("an unknown block type is a thrown decode error naming the block id")
    func unknownBlockTypeThrows() throws {
        let json = Data(#"{"id":"weird","type":"bogus","x":1}"#.utf8)
        #expect(throws: DecodingError.self) {
            _ = try decoder.decode(Block.self, from: json)
        }
        // The offending id and type appear in the message (fail loud).
        do {
            _ = try decoder.decode(Block.self, from: json)
            Issue.record("expected a thrown error")
        } catch let DecodingError.dataCorrupted(context) {
            #expect(context.debugDescription.contains("weird"))
            #expect(context.debugDescription.contains("bogus"))
        }
    }

    @Test("encoding a block writes the type discriminator alongside its fields")
    func encodeWritesTypeDiscriminator() throws {
        let data = try encoder.encode(Block.progress(Block.Progress(id: "p", label: "L", value: 1, max: 4)))
        let object = try #require(try JSONSerialization.jsonObject(with: data) as? [String: Any])
        #expect(object["type"] as? String == "progress")
        #expect(object["id"] as? String == "p")
        #expect(object["value"] as? Int == 1)
        #expect(object["max"] as? Int == 4)
        // An omitempty-absent optional stays absent on the wire.
        #expect(object["state"] == nil)
    }
}
