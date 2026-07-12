import CcPresentKit
import Foundation

/// flatten yields every top-level block plus every card child, so a tally or the
/// review ring spans interactive blocks wherever they nest one level deep. Mirrors
/// web/src/decide.ts `flatten`.
func flatten(_ blocks: [Block]) -> [Block] {
    var out: [Block] = []
    for block in blocks {
        out.append(block)
        if case let .card(card) = block {
            out.append(contentsOf: card.children)
        }
    }
    return out
}

/// showsNativeReplyThread reports whether BlockView renders a native agent reply
/// thread beneath a block. Approval owns its integrated thread, and a pack block's
/// thread renders inside its WKWebView (the web SingleBlockView), so native
/// rendering would double it; every other block type shows the native thread. The
/// switch is exhaustive with no default arm, so a new block type must classify
/// itself. Mirrors web/src/components/BlockRenderer.tsx.
func showsNativeReplyThread(_ block: Block) -> Bool {
    switch block {
    case .approval, .pack:
        false
    case .section, .card, .choice, .input, .markdown, .code, .diff, .image, .table, .progress:
        true
    }
}

/// SubmitItem is one entry of the submit tally: an approval or choice with its
/// decided state. Inputs never count toward the tally.
struct SubmitItem: Equatable {
    /// Kind is which interactive block produced this tally entry.
    enum Kind: Equatable {
        case approval
        case choice
    }

    let id: String
    let kind: Kind
    let decided: Bool
}

/// submitItems is the tally set — approvals and choices in document order with
/// their decided state — driving the SubmitBar count. Mirrors web/src/decide.ts.
func submitItems(_ blocks: [Block], _ interactions: Interactions) -> [SubmitItem] {
    var out: [SubmitItem] = []
    for block in flatten(blocks) {
        switch block {
        case let .approval(approval):
            out.append(SubmitItem(id: approval.id, kind: .approval, decided: isDecided(block, interactions)))
        case let .choice(choice):
            out.append(SubmitItem(id: choice.id, kind: .choice, decided: isDecided(block, interactions)))
        default:
            continue
        }
    }
    return out
}

/// isDecided mirrors the SubmitBar tally for one block: an approval with any
/// verdict (cleared decisions are removed, so presence is decidedness) or a choice
/// with at least one selected option. Every other block is never decided.
func isDecided(_ block: Block, _ interactions: Interactions) -> Bool {
    switch block {
    case let .approval(approval):
        interactions.decisions[approval.id] != nil
    case let .choice(choice):
        !(interactions.choices[choice.id]?.optionIds.isEmpty ?? true)
    default:
        false
    }
}

/// RoundTally is the header summary a closed round shows: how many approvals were
/// approved or rejected, how many choices were picked, and the note count (filled
/// inputs plus feedback entries). Mirrors web/src/components/RoundGroup.tsx.
struct RoundTally: Equatable {
    let approved: Int
    let rejected: Int
    let picks: Int
    let notes: Int
}

/// roundTally derives a closed round's header summary from its frozen snapshot.
func roundTally(_ record: RoundRecord) -> RoundTally {
    var approved = 0
    var rejected = 0
    var picks = 0
    var filledInputs = 0
    var feedbackNotes = 0
    for block in flatten(record.blocks) {
        switch block {
        case let .approval(approval):
            switch record.decisions[approval.id]?.verdict {
            case "approved": approved += 1
            case "rejected": rejected += 1
            default: break
            }
        case let .choice(choice):
            if !(record.choices[choice.id]?.optionIds.isEmpty ?? true) {
                picks += 1
            }
        case let .input(input):
            let text = record.inputs[input.id]?.text.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if !text.isEmpty {
                filledInputs += 1
            }
        case let .pack(pack):
            if record.packs[pack.id] != nil {
                picks += 1
            }
        default:
            break
        }
        feedbackNotes += record.feedback[block.id]?.count ?? 0
    }
    return RoundTally(approved: approved, rejected: rejected, picks: picks, notes: filledInputs + feedbackNotes)
}
