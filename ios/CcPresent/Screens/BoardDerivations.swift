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

/// showsNativeReplyThread reports whether BlockView renders a native reply thread below a
/// block. Approval and choice own their integrated threads and a pack's renders in its
/// WKWebView, so native rendering would double them; every other type shows it. The
/// exhaustive switch forces a new block type to classify itself. Mirrors BlockRenderer.tsx.
func showsNativeReplyThread(_ block: Block) -> Bool {
    switch block {
    case .approval, .choice, .pack:
        false
    case .section, .card, .draft, .triage, .input, .markdown, .code, .diff, .diagram, .image, .table, .progress,
         .chart, .term, .filetree, .record:
        true
    }
}

/// SectionGroup is one header-led run of the board feed: a section header over the
/// top-level blocks that follow it, up to the next section. `id` is namespaced —
/// "s#" + the section id for a header group, "lead" for the headerless run before
/// the first section — so a section whose own id collides with the lead sentinel
/// can't produce duplicate ForEach identity. Drives BoardScreen's pinned headers.
struct SectionGroup: Equatable {
    let id: String
    let header: Block.Section?
    let blocks: [Block]
}

/// sectionGroups splits top-level blocks into header-led runs for the pinned-header
/// board layout. Blocks before the first section form the lead group, omitted
/// entirely when empty; each section opens a group carrying the following non-section
/// blocks up to the next section or the end, so a trailing or back-to-back section
/// yields a group with empty blocks. A section nested inside a card stays in that
/// card's body — only top-level sections split the feed. Concatenating every group's
/// header and blocks reproduces the input exactly.
func sectionGroups(_ blocks: [Block]) -> [SectionGroup] {
    var groups: [SectionGroup] = []
    var lead: [Block] = []
    var header: Block.Section?
    var body: [Block] = []

    func flush() {
        if let header {
            groups.append(SectionGroup(id: "s#" + header.id, header: header, blocks: body))
        }
    }

    for block in blocks {
        if case let .section(section) = block {
            flush()
            header = section
            body = []
        } else if header == nil {
            lead.append(block)
        } else {
            body.append(block)
        }
    }
    flush()

    if !lead.isEmpty {
        groups.insert(SectionGroup(id: "lead", header: nil, blocks: lead), at: 0)
    }
    return groups
}

/// SubmitItem is one entry of the submit tally: an approval, choice, or interactive
/// pack block with its decided state. Inputs never count toward the tally.
struct SubmitItem: Equatable {
    /// Kind is which interactive block produced this tally entry.
    enum Kind: Equatable {
        case approval
        case choice
        case triage
        case pack
    }

    let id: String
    let kind: Kind
    let decided: Bool
}

/// submitItems is the tally set — approvals, choices, and interactive pack blocks in
/// document order with their decided state — driving the SubmitBar count. A pack
/// block joins the tally only when its type is in `packInteractive`, the manifest's
/// interactive set. Mirrors web/src/decide.ts `submitItems`.
func submitItems(_ blocks: [Block], _ interactions: Interactions, _ packInteractive: Set<String>) -> [SubmitItem] {
    var out: [SubmitItem] = []
    for block in flatten(blocks) {
        switch block {
        case let .approval(approval):
            out.append(SubmitItem(id: approval.id, kind: .approval, decided: isDecided(block, interactions)))
        case let .choice(choice):
            out.append(SubmitItem(id: choice.id, kind: .choice, decided: isDecided(block, interactions)))
        case let .triage(triage):
            out.append(SubmitItem(id: triage.id, kind: .triage, decided: isDecided(block, interactions)))
        case let .pack(pack):
            if packInteractive.contains(pack.packType) {
                out.append(SubmitItem(id: pack.id, kind: .pack, decided: isDecided(block, interactions)))
            }
        default:
            continue
        }
    }
    return out
}

/// isDecided mirrors the SubmitBar tally for one block: an approval with any verdict
/// (cleared decisions are removed, so presence is decidedness), a choice with at
/// least one selected option or an other write-in, or a pack block with a stored
/// interaction. Every other block is never decided. Mirrors web/src/decide.ts `isDecided`.
func isDecided(_ block: Block, _ interactions: Interactions) -> Bool {
    switch block {
    case let .approval(approval):
        interactions.decisions[approval.id] != nil
    case let .choice(choice):
        if let selection = interactions.choices[choice.id] {
            !selection.optionIds.isEmpty || selection.other != nil
        } else {
            false
        }
    case let .triage(triage):
        triage.items.allSatisfy { interactions.triage[triage.id]?[$0.id] != nil }
    case let .pack(pack):
        interactions.packs[pack.id] != nil
    default:
        false
    }
}

/// blockDecided reports whether a board row has receded to a receipt: it holds at
/// least one decidable and every one is decided (a fully-decided card counts, an
/// undecidable row never does). It drives the BoardScreen receipt dimming — the
/// native mirror of web/src/decide.ts `blockDecided`, the BoardBlocks
/// `data-decided` signal.
func blockDecided(_ block: Block, _ interactions: Interactions, _ packInteractive: Set<String>) -> Bool {
    let items = submitItems([block], interactions, packInteractive)
    return !items.isEmpty && items.allSatisfy(\.decided)
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
            if let selection = record.choices[choice.id], !selection.optionIds.isEmpty || selection.other != nil {
                picks += 1
            }
        case let .triage(triage):
            let verdicts = record.triage[triage.id] ?? [:]
            for item in triage.items {
                switch verdicts[item.id]?.verdict {
                case "approved": approved += 1
                case "rejected": rejected += 1
                default: break
                }
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
