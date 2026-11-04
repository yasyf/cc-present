import Foundation

/// TalkEntry is one line of a Talk thread: a note the human sent, an agent reply, or
/// a block the agent posted. Mirrors web/src/talk.ts.
public enum TalkEntry: Equatable, Sendable {
    case note(Feedback)
    case reply(Reply)
    case block(Block)
}

/// talkEntries orders a Talk board as one thread: replies and blocks that answer no
/// note first, then each note followed by the replies and cards that answer it.
public func talkEntries(_ blocks: [Block], talkId: String, feedback: [Feedback], replies: [Reply]) -> [TalkEntry] {
    let noteIds = Set(feedback.map(\.id))
    let rest = blocks.filter { $0.id != talkId }
    func anchor(_ block: Block) -> String? {
        guard case let .card(card) = block, let re = card.re, noteIds.contains(re) else { return nil }
        return re
    }
    func answers(_ re: String?) -> [TalkEntry] {
        let answering = replies.filter { reply in
            guard let re else { return reply.re.map { !noteIds.contains($0) } ?? true }
            return reply.re == re
        }
        return answering.map(TalkEntry.reply) + rest.filter { anchor($0) == re }.map(TalkEntry.block)
    }
    return answers(nil) + feedback.flatMap { [TalkEntry.note($0)] + answers($0.id) }
}
