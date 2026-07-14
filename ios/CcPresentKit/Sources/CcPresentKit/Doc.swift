// The typed-block document an agent composes and a client renders. These types
// mirror the Go structs in internal/doc/doc.go and the canonical TypeScript in
// web/src/schema.ts field for field, with the same camelCase JSON names. The
// document carries only agent-owned display state (card status, progress); human
// verdicts live in the separate event reduction (see Event.swift).

import Foundation

/// Doc is the document envelope: header metadata plus a flat list of top-level
/// blocks. Version is the schema version and is always 1.
public struct Doc: Codable, Equatable, Sendable {
    /// Stat is a headline metric shown in the document header.
    public struct Stat: Codable, Equatable, Sendable {
        public var label: String
        public var value: String

        public init(label: String, value: String) {
            self.label = label
            self.value = value
        }
    }

    /// Submit configures the document's submit control.
    public struct Submit: Codable, Equatable, Sendable {
        public var label: String
        public var note: String?

        public init(label: String, note: String? = nil) {
            self.label = label
            self.note = note
        }
    }

    /// Presentation is a per-push hint for the client's default view; the
    /// viewer's own toggle overrides it.
    public enum Presentation: String, Codable, Equatable, Sendable {
        case focus
        case board
    }

    public var version: Int
    public var title: String
    public var intro: String?
    public var stats: [Stat]?
    public var submit: Submit?
    public var presentation: Presentation?
    public var blocks: [Block]

    public init(
        version: Int = 1,
        title: String,
        intro: String? = nil,
        stats: [Stat]? = nil,
        submit: Submit? = nil,
        presentation: Presentation? = nil,
        blocks: [Block]
    ) {
        self.version = version
        self.title = title
        self.intro = intro
        self.stats = stats
        self.submit = submit
        self.presentation = presentation
        self.blocks = blocks
    }
}

/// Block is a node in a document. A section, a card, or any of the nine leaf
/// blocks may appear at the top level; a card nests one level of leaf blocks. The
/// JSON `type` tag is the discriminant, encoded alongside each case's fields.
public enum Block: Codable, Equatable, Sendable {
    case section(Section)
    case card(Card)
    case approval(Approval)
    case choice(Choice)
    case input(Input)
    case markdown(Markdown)
    case code(Code)
    case diff(Diff)
    case image(Image)
    case table(Table)
    case progress(Progress)
    case pack(Pack)

    /// Section is a top-level header marker with optional prose.
    public struct Section: Codable, Equatable, Sendable {
        public var id: String
        public var title: String
        public var md: String?

        public init(id: String, title: String, md: String? = nil) {
            self.id = id
            self.title = title
            self.md = md
        }
    }

    /// Chip is a small labelled tag on a card; tone is `default`, `flag`, or `demo`.
    public struct Chip: Codable, Equatable, Sendable {
        public var label: String
        public var tone: String?

        public init(label: String, tone: String? = nil) {
            self.label = label
            self.tone = tone
        }
    }

    /// Card is a top-level container nesting one level of leaf blocks. Status is
    /// agent-owned display state (`open`, `resolved`, `redrafted`); it never
    /// records a human verdict.
    public struct Card: Codable, Equatable, Sendable {
        public var id: String
        public var title: String?
        public var summary: String?
        public var chips: [Chip]?
        public var flagged: Bool?
        public var status: String?
        public var children: [Block]

        public init(
            id: String,
            title: String? = nil,
            summary: String? = nil,
            chips: [Chip]? = nil,
            flagged: Bool? = nil,
            status: String? = nil,
            children: [Block]
        ) {
            self.id = id
            self.title = title
            self.summary = summary
            self.chips = chips
            self.flagged = flagged
            self.status = status
            self.children = children
        }
    }

    /// Approval is an approve/reject control with optional free-text feedback;
    /// allowFeedback defaults to true at render time when omitted. Detail is an
    /// optional Tier-2 drill-down shown under the prompt.
    public struct Approval: Codable, Equatable, Sendable {
        public var id: String
        public var prompt: String?
        public var allowFeedback: Bool?
        public var detail: Detail?

        public init(id: String, prompt: String? = nil, allowFeedback: Bool? = nil, detail: Detail? = nil) {
            self.id = id
            self.prompt = prompt
            self.allowFeedback = allowFeedback
            self.detail = detail
        }
    }

    /// Fact is one scannable key/value in an option's up-front cluster; tone is
    /// `default`, `good`, `warn`, or `bad`.
    public struct Fact: Codable, Equatable, Sendable {
        public var label: String?
        public var value: String
        public var tone: String?

        public init(label: String? = nil, value: String, tone: String? = nil) {
            self.label = label
            self.value = value
            self.tone = tone
        }
    }

    /// Detail is an expandable drill-down of tradeoffs and full rationale, hidden
    /// until opened. Mode is `inline` (the default — expands in place) or `modal`
    /// (opens in an overlay).
    public struct Detail: Codable, Equatable, Sendable {
        public var pros: [String]?
        public var cons: [String]?
        public var md: String?
        public var mode: String?

        public init(pros: [String]? = nil, cons: [String]? = nil, md: String? = nil, mode: String? = nil) {
            self.pros = pros
            self.cons = cons
            self.md = md
            self.mode = mode
        }
    }

    /// Option is one selectable entry within a Choice block. Facts is the Tier-1
    /// up-front cluster; detail is the optional Tier-2 drill-down.
    public struct Option: Codable, Equatable, Sendable {
        public var id: String
        public var label: String
        public var hint: String?
        public var md: String?
        public var facts: [Fact]?
        public var detail: Detail?

        public init(
            id: String,
            label: String,
            hint: String? = nil,
            md: String? = nil,
            facts: [Fact]? = nil,
            detail: Detail? = nil
        ) {
            self.id = id
            self.label = label
            self.hint = hint
            self.md = md
            self.facts = facts
            self.detail = detail
        }
    }

    /// Choice is a single- or multi-select control.
    public struct Choice: Codable, Equatable, Sendable {
        public var id: String
        public var prompt: String?
        public var multi: Bool?
        public var options: [Option]

        public init(id: String, prompt: String? = nil, multi: Bool? = nil, options: [Option]) {
            self.id = id
            self.prompt = prompt
            self.multi = multi
            self.options = options
        }
    }

    /// Input is a free-text field.
    public struct Input: Codable, Equatable, Sendable {
        public var id: String
        public var label: String
        public var placeholder: String?
        public var multiline: Bool?

        public init(id: String, label: String, placeholder: String? = nil, multiline: Bool? = nil) {
            self.id = id
            self.label = label
            self.placeholder = placeholder
            self.multiline = multiline
        }
    }

    /// Markdown is a rendered markdown block; struck applies the "was:" treatment.
    public struct Markdown: Codable, Equatable, Sendable {
        public var id: String
        public var md: String
        public var struck: Bool?

        public init(id: String, md: String, struck: Bool? = nil) {
            self.id = id
            self.md = md
            self.struck = struck
        }
    }

    /// Code is a syntax-highlighted code block.
    public struct Code: Codable, Equatable, Sendable {
        public var id: String
        public var lang: String
        public var code: String
        public var title: String?

        public init(id: String, lang: String, code: String, title: String? = nil) {
            self.id = id
            self.lang = lang
            self.code = code
            self.title = title
        }
    }

    /// Diff is a unified-diff block.
    public struct Diff: Codable, Equatable, Sendable {
        public var id: String
        public var diff: String
        public var title: String?

        public init(id: String, diff: String, title: String? = nil) {
            self.id = id
            self.diff = diff
            self.title = title
        }
    }

    /// Image is an image reference; src is an https:, asset:<sha256>, or data: URI.
    public struct Image: Codable, Equatable, Sendable {
        public var id: String
        public var src: String
        public var alt: String
        public var caption: String?

        public init(id: String, src: String, alt: String, caption: String? = nil) {
            self.id = id
            self.src = src
            self.alt = alt
            self.caption = caption
        }
    }

    /// Column describes one column of a Table; align is `left` or `right`.
    public struct Column: Codable, Equatable, Sendable {
        public var key: String
        public var label: String
        public var align: String?

        public init(key: String, label: String, align: String? = nil) {
            self.key = key
            self.label = label
            self.align = align
        }
    }

    /// Table is a columnar block; each row maps a column key to an inline-markdown cell.
    public struct Table: Codable, Equatable, Sendable {
        public var id: String
        public var columns: [Column]
        public var rows: [[String: String]]

        public init(id: String, columns: [Column], rows: [[String: String]]) {
            self.id = id
            self.columns = columns
            self.rows = rows
        }
    }

    /// Progress is a progress bar; state is agent-owned display state
    /// (`active`, `done`, `error`).
    public struct Progress: Codable, Equatable, Sendable {
        public var id: String
        public var label: String
        public var value: Int
        public var max: Int
        public var state: String?

        public init(id: String, label: String, value: Int, max: Int, state: String? = nil) {
            self.id = id
            self.label = label
            self.value = value
            self.max = max
            self.state = state
        }
    }

    /// Pack is a plugin-supplied block whose type carries a `<pack>.<name>`
    /// namespace. The client does not model its fields; `raw` holds the entire
    /// block object (id and type included) verbatim so a pack-supplied renderer
    /// receives it byte-faithfully and re-encoding never drops unknown fields.
    public struct Pack: Codable, Equatable, Sendable {
        public var id: String
        public var packType: String
        public var raw: JSONValue

        public init(id: String, packType: String, raw: JSONValue) {
            self.id = id
            self.packType = packType
            self.raw = raw
        }
    }

    /// id is the block's globally unique identifier.
    public var id: String {
        switch self {
        case let .section(block): block.id
        case let .card(block): block.id
        case let .approval(block): block.id
        case let .choice(block): block.id
        case let .input(block): block.id
        case let .markdown(block): block.id
        case let .code(block): block.id
        case let .diff(block): block.id
        case let .image(block): block.id
        case let .table(block): block.id
        case let .progress(block): block.id
        case let .pack(block): block.id
        }
    }

    /// type is the JSON discriminator tag for this block.
    public var type: String {
        switch self {
        case .section: "section"
        case .card: "card"
        case .approval: "approval"
        case .choice: "choice"
        case .input: "input"
        case .markdown: "markdown"
        case .code: "code"
        case .diff: "diff"
        case .image: "image"
        case .table: "table"
        case .progress: "progress"
        case let .pack(block): block.packType
        }
    }

    private enum Discriminator: String, CodingKey {
        case id
        case type
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: Discriminator.self)
        let type = try container.decode(String.self, forKey: .type)
        switch type {
        case "section": self = try .section(Section(from: decoder))
        case "card": self = try .card(Card(from: decoder))
        case "approval": self = try .approval(Approval(from: decoder))
        case "choice": self = try .choice(Choice(from: decoder))
        case "input": self = try .input(Input(from: decoder))
        case "markdown": self = try .markdown(Markdown(from: decoder))
        case "code": self = try .code(Code(from: decoder))
        case "diff": self = try .diff(Diff(from: decoder))
        case "image": self = try .image(Image(from: decoder))
        case "table": self = try .table(Table(from: decoder))
        case "progress": self = try .progress(Progress(from: decoder))
        default:
            if type.contains(".") {
                let id = try container.decode(String.self, forKey: .id)
                self = try .pack(Pack(id: id, packType: type, raw: JSONValue(from: decoder)))
                return
            }
            let id = try container.decodeIfPresent(String.self, forKey: .id) ?? ""
            throw DecodingError.dataCorrupted(
                DecodingError.Context(
                    codingPath: decoder.codingPath,
                    debugDescription: "block \"\(id)\": unknown type \"\(type)\""
                )
            )
        }
    }

    public func encode(to encoder: Encoder) throws {
        // A pack block's raw JSON already carries id and type; encode it alone and
        // return before the discriminator container writes type a second time.
        if case let .pack(block) = self {
            try block.raw.encode(to: encoder)
            return
        }
        var container = encoder.container(keyedBy: Discriminator.self)
        try container.encode(type, forKey: .type)
        switch self {
        case let .section(block): try block.encode(to: encoder)
        case let .card(block): try block.encode(to: encoder)
        case let .approval(block): try block.encode(to: encoder)
        case let .choice(block): try block.encode(to: encoder)
        case let .input(block): try block.encode(to: encoder)
        case let .markdown(block): try block.encode(to: encoder)
        case let .code(block): try block.encode(to: encoder)
        case let .diff(block): try block.encode(to: encoder)
        case let .image(block): try block.encode(to: encoder)
        case let .table(block): try block.encode(to: encoder)
        case let .progress(block): try block.encode(to: encoder)
        case .pack: break
        }
    }
}
