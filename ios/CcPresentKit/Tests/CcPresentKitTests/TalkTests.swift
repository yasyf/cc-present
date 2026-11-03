@testable import CcPresentKit
import Testing

@Suite("Talk thread order")
struct TalkTests {
    private let talk = Block.section(Block.Section(id: "talk", title: "Talk"))

    private func card(_ id: String, re: String?) -> Block {
        .card(Block.Card(id: id, re: re, children: []))
    }

    @Test("each note is followed by the replies and cards that answer it")
    func notesCarryTheirAnswers() {
        let entries = talkEntries(
            [talk, card("c1", re: "n1"), card("c2", re: nil)],
            talkId: "talk",
            feedback: [Feedback(id: "n1", text: "first"), Feedback(id: "n2", text: "second")],
            replies: [
                Reply(id: "r1", md: "to the first", re: "n1"),
                Reply(id: "r0", md: "unprompted", re: nil),
                Reply(id: "r2", md: "to the second", re: "n2"),
            ]
        )
        #expect(entries == [
            .reply(Reply(id: "r0", md: "unprompted", re: nil)),
            .block(card("c2", re: nil)),
            .note(Feedback(id: "n1", text: "first")),
            .reply(Reply(id: "r1", md: "to the first", re: "n1")),
            .block(card("c1", re: "n1")),
            .note(Feedback(id: "n2", text: "second")),
            .reply(Reply(id: "r2", md: "to the second", re: "n2")),
        ])
    }

    @Test("an answer to a note that is gone joins the unprompted ones, and the Talk section never shows")
    func orphansAndTheSectionItself() {
        let entries = talkEntries(
            [talk, card("c1", re: "gone")],
            talkId: "talk",
            feedback: [],
            replies: [Reply(id: "r1", md: "late", re: "gone")]
        )
        #expect(entries == [.reply(Reply(id: "r1", md: "late", re: "gone")), .block(card("c1", re: "gone"))])
    }
}
