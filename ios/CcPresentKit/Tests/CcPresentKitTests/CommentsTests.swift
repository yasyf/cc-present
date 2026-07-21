import CcPresentKit
import Testing

/// fixtureDoc is a small document spanning the comment-bearing shapes: two top-level
/// decidables, a card wrapping a nested approval, and an input, so the feed walk and the
/// per-block predicates have a stable corpus.
private func fixtureDoc() -> Doc {
    Doc(title: "Review", blocks: [
        .approval(.init(id: "ap1", prompt: "Ship it?")),
        .choice(.init(id: "ch1", prompt: "Pick one", options: [])),
        .card(.init(id: "card1", title: "Card A", children: [
            .approval(.init(id: "ap2", prompt: "Nested?", allowFeedback: false)),
        ])),
        .input(.init(id: "in1", label: "Name")),
    ])
}

private func fixtureInteractions() -> Interactions {
    Interactions(
        feedback: [
            "ap1": [Feedback(id: "f1", text: "a"), Feedback(id: "f2", text: "b")],
            "ch1": [Feedback(id: "f3", text: "c")],
        ],
        replies: [
            "ap1": [Reply(id: "r1", md: "x")],
            "ap2": [Reply(id: "r2", md: "y")],
        ]
    )
}

@Suite("Comments projection")
struct CommentsTests {
    @Test("the feed walks the document in order, skips the active block and the uncommented")
    func feedOrderingAndFilters() {
        let feed = commentFeed(fixtureDoc(), fixtureInteractions(), activeId: "ap1")
        #expect(feed.map(\.blockId) == ["ch1", "ap2"])
        #expect(feed[0].feedbackCount == 1)
        #expect(feed[0].replyCount == 0)
        #expect(feed[0].title == "Pick one")
        #expect(feed[1].feedbackCount == 0)
        #expect(feed[1].replyCount == 1)
        #expect(feed[1].title == "Nested?")
    }

    @Test("a nil active block includes every commented block, card children inlined in order")
    func feedWithoutActive() {
        let feed = commentFeed(fixtureDoc(), fixtureInteractions(), activeId: nil)
        #expect(feed.map(\.blockId) == ["ap1", "ch1", "ap2"])
        #expect(feed[0].total == 3)
    }

    @Test("per-block and total counts sum feedback and replies")
    func counts() {
        let interactions = fixtureInteractions()
        #expect(commentCount(interactions, "ap1") == 3)
        #expect(commentCount(interactions, "ch1") == 1)
        #expect(commentCount(interactions, "in1") == 0)
        #expect(totalCommentCount(interactions) == 5)
    }

    @Test("the active block resolves chip pin over focus decidable over last-interacted")
    func activeResolution() {
        #expect(resolveActiveComment(pinned: "p", focusActive: "f", lastInteracted: "l") == "p")
        #expect(resolveActiveComment(pinned: nil, focusActive: "f", lastInteracted: "l") == "f")
        #expect(resolveActiveComment(pinned: nil, focusActive: nil, lastInteracted: "l") == "l")
        #expect(resolveActiveComment(pinned: nil, focusActive: nil, lastInteracted: nil) == nil)
    }

    @Test("only an approval that allows feedback, or a choice, shows a composer")
    func acceptsFeedback() {
        let doc = fixtureDoc()
        #expect(commentAcceptsFeedback(doc, "ap1"))
        #expect(commentAcceptsFeedback(doc, "ch1"))
        #expect(!commentAcceptsFeedback(doc, "ap2")) // allowFeedback: false
        #expect(!commentAcceptsFeedback(doc, "in1"))
        #expect(!commentAcceptsFeedback(doc, "missing"))
    }

    @Test("block titles fall back through prompt, title, and label")
    func titles() {
        let blocks = commentDocBlocks(fixtureDoc())
        #expect(commentBlockTitle(blocks[0]) == "Ship it?")
        #expect(commentBlockTitle(blocks[2]) == "Card A")
        #expect(commentBlockTitle(blocks[4]) == "Name")
    }

    @Test("the composer latch holds while focused or holding non-whitespace text")
    func composerHold() {
        #expect(!commentComposerHolds(focused: false, draft: ""))
        #expect(commentComposerHolds(focused: true, draft: ""))
        #expect(!commentComposerHolds(focused: false, draft: "   \n"))
        #expect(commentComposerHolds(focused: false, draft: "note"))
    }
}
