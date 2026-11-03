// The typed-block document an agent composes and a client renders. These types
// mirror the Go structs in internal/doc/doc.go and the canonical TypeScript in
// web/src/schema.ts field for field, with the same camelCase JSON names. The
// document carries only agent-owned display state (card status, progress); human
// verdicts live in the separate event reduction (see Event.swift).

import Foundation
import IOSKit

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

/// Block is a node in a document. A section, a card, or any of the leaf blocks
/// may appear at the top level; a card nests one level of leaf blocks. The JSON
/// `type` tag is the discriminant, encoded alongside each case's fields.
public enum Block: Codable, Equatable, Sendable {
    case section(Section)
    case card(Card)
    case approval(Approval)
    case choice(Choice)
    case draft(Draft)
    case triage(Triage)
    case input(Input)
    case markdown(Markdown)
    case code(Code)
    case diff(Diff)
    case diagram(Diagram)
    case image(Image)
    case table(Table)
    case progress(Progress)
    case chart(Chart)
    case term(Term)
    case filetree(FileTree)
    case record(Record)
    case pr(PR)
    case prGroups(PRGroups)
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
    /// records a human verdict. `re` names the Talk note the card answers.
    public struct Card: Codable, Equatable, Sendable {
        public var id: String
        public var title: String?
        public var summary: String?
        public var chips: [Chip]?
        public var flagged: Bool?
        public var status: String?
        public var re: String?
        public var children: [Block]

        public init(
            id: String,
            title: String? = nil,
            summary: String? = nil,
            chips: [Chip]? = nil,
            flagged: Bool? = nil,
            status: String? = nil,
            re: String? = nil,
            children: [Block]
        ) {
            self.id = id
            self.title = title
            self.summary = summary
            self.chips = chips
            self.flagged = flagged
            self.status = status
            self.re = re
            self.children = children
        }
    }

    /// Approval is an approve/reject control that always takes free-text feedback.
    /// Detail is an optional Tier-2 drill-down shown under the prompt.
    public struct Approval: Codable, Equatable, Sendable {
        public var id: String
        public var prompt: String?
        public var detail: Detail?

        public init(id: String, prompt: String? = nil, detail: Detail? = nil) {
            self.id = id
            self.prompt = prompt
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
    /// up-front cluster; detail is the optional Tier-2 drill-down. Recommended marks
    /// the author's suggested pick; visual is the option's restricted visual leaf.
    public struct Option: Codable, Equatable, Sendable {
        public var id: String
        public var label: String
        public var hint: String?
        public var md: String?
        public var facts: [Fact]?
        public var detail: Detail?
        public var recommended: Bool?
        public var visual: OptionVisual?

        public init(
            id: String,
            label: String,
            hint: String? = nil,
            md: String? = nil,
            facts: [Fact]? = nil,
            detail: Detail? = nil,
            recommended: Bool? = nil,
            visual: OptionVisual? = nil
        ) {
            self.id = id
            self.label = label
            self.hint = hint
            self.md = md
            self.facts = facts
            self.detail = detail
            self.recommended = recommended
            self.visual = visual
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

    /// Draft is a document under line-anchored markup, rendered as numbered source
    /// lines a human annotates by content-addressed anchor. Lang selects highlighting;
    /// title is an optional single-line caption. It carries no human state — the
    /// annotations live in the separate event reduction (see Reduce.swift).
    public struct Draft: Codable, Equatable, Sendable {
        public var id: String
        public var lang: String
        public var text: String
        public var title: String?

        public init(id: String, lang: String, text: String, title: String? = nil) {
            self.id = id
            self.lang = lang
            self.text = text
            self.title = title
        }
    }

    /// Item is one entry in a Triage block: a labelled row a human accepts or rejects.
    /// It carries the same optional rich body as a Choice Option — hint, markdown,
    /// facts, a Tier-2 detail, and a restricted visual — minus the author's
    /// recommendation, which has no meaning per item.
    public struct Item: Codable, Equatable, Sendable {
        public var id: String
        public var label: String
        public var hint: String?
        public var md: String?
        public var facts: [Fact]?
        public var detail: Detail?
        public var visual: OptionVisual?

        public init(
            id: String,
            label: String,
            hint: String? = nil,
            md: String? = nil,
            facts: [Fact]? = nil,
            detail: Detail? = nil,
            visual: OptionVisual? = nil
        ) {
            self.id = id
            self.label = label
            self.hint = hint
            self.md = md
            self.facts = facts
            self.detail = detail
            self.visual = visual
        }
    }

    /// Triage is a multi-item accept/reject control: each Item takes its own verdict,
    /// with optional per-item notes. AllowNotes defaults to true at render time when
    /// omitted; prompt is an optional heading.
    public struct Triage: Codable, Equatable, Sendable {
        public var id: String
        public var prompt: String?
        public var allowNotes: Bool?
        public var items: [Item]

        public init(id: String, prompt: String? = nil, allowNotes: Bool? = nil, items: [Item]) {
            self.id = id
            self.prompt = prompt
            self.allowNotes = allowNotes
            self.items = items
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

    /// Code is a syntax-highlighted code block. One with `src` is grounded in a repository
    /// file: the CLI fills `code`, `start`, `lang`, and `sha` from the `lines` slice of `src`.
    public struct Code: Codable, Equatable, Sendable {
        public var id: String
        public var lang: String
        public var code: String
        public var title: String?
        public var src: String?
        public var lines: String?
        public var start: Int?
        public var highlight: String?
        public var pins: [CodePin]?
        public var sha: String?

        public init(
            id: String,
            lang: String,
            code: String,
            title: String? = nil,
            src: String? = nil,
            lines: String? = nil,
            start: Int? = nil,
            highlight: String? = nil,
            pins: [CodePin]? = nil,
            sha: String? = nil
        ) {
            self.id = id
            self.lang = lang
            self.code = code
            self.title = title
            self.src = src
            self.lines = lines
            self.start = start
            self.highlight = highlight
            self.pins = pins
            self.sha = sha
        }
    }

    /// CodePin is a numbered callout on one file line of a code block; tone is
    /// `default`, `good`, `warn`, or `bad`.
    public struct CodePin: Codable, Equatable, Sendable {
        public var line: Int
        public var title: String
        public var body: String?
        public var tone: String?

        public init(line: Int, title: String, body: String? = nil, tone: String? = nil) {
            self.line = line
            self.title = title
            self.body = body
            self.tone = tone
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

    /// Diagram is a text-to-diagram block rendered client-side; kind is `mermaid`.
    public struct Diagram: Codable, Equatable, Sendable {
        public var id: String
        public var kind: String
        public var source: String
        public var title: String?

        public init(id: String, kind: String, source: String, title: String? = nil) {
            self.id = id
            self.kind = kind
            self.source = source
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

    /// Series is one data series in a Chart; values align index-for-index to the
    /// chart's categories.
    public struct Series: Codable, Equatable, Sendable {
        public var label: String
        public var values: [Double]

        public init(label: String, values: [Double]) {
            self.label = label
            self.values = values
        }
    }

    /// Chart is a categorical bar or line chart rendered client-side into themed
    /// SVG; kind is `bar` or `line`.
    public struct Chart: Codable, Equatable, Sendable {
        public var id: String
        public var kind: String
        public var title: String?
        public var unit: String?
        public var categories: [String]
        public var series: [Series]

        public init(
            id: String,
            kind: String,
            title: String? = nil,
            unit: String? = nil,
            categories: [String],
            series: [Series]
        ) {
            self.id = id
            self.kind = kind
            self.title = title
            self.unit = unit
            self.categories = categories
            self.series = series
        }
    }

    /// Term is a terminal panel: an optional command line above its captured output.
    public struct Term: Codable, Equatable, Sendable {
        public var id: String
        public var command: String?
        public var output: String
        public var title: String?

        public init(id: String, command: String? = nil, output: String, title: String? = nil) {
            self.id = id
            self.command = command
            self.output = output
            self.title = title
        }
    }

    /// TreeEntry is one file path in a FileTree; directories are implicit from the
    /// path segments. Badge is a git-status change marker; note is an inline
    /// annotation.
    public struct TreeEntry: Codable, Equatable, Sendable {
        public var path: String
        public var badge: String?
        public var note: String?

        public init(path: String, badge: String? = nil, note: String? = nil) {
            self.path = path
            self.badge = badge
            self.note = note
        }
    }

    /// FileTree is a set of relative file paths rendered as a collapsible tree
    /// with change badges.
    public struct FileTree: Codable, Equatable, Sendable {
        public var id: String
        public var title: String?
        public var entries: [TreeEntry]

        public init(id: String, title: String? = nil, entries: [TreeEntry]) {
            self.id = id
            self.title = title
            self.entries = entries
        }
    }

    /// Link is a labelled https link in a Record.
    public struct Link: Codable, Equatable, Sendable {
        public var label: String
        public var url: String

        public init(label: String, url: String) {
            self.label = label
            self.url = url
        }
    }

    /// Record is one entity's labelled profile: tone chips, keyed facts, and
    /// https links. It reuses the shared Chip and Fact types.
    public struct Record: Codable, Equatable, Sendable {
        public var id: String
        public var title: String?
        public var chips: [Chip]?
        public var facts: [Fact]
        public var links: [Link]?

        public init(id: String, title: String? = nil, chips: [Chip]? = nil, facts: [Fact], links: [Link]? = nil) {
            self.id = id
            self.title = title
            self.chips = chips
            self.facts = facts
            self.links = links
        }
    }

    /// PR is a GitHub pull request the daemon reads live through ccx; `repo` is
    /// owner/name, `title` overrides the PR's own, `files` narrows its changed files,
    /// and `head` is the commit a card asks about.
    public struct PR: Codable, Equatable, Sendable {
        public var id: String
        public var repo: String
        public var number: Int
        public var title: String?
        public var summary: String?
        public var files: [String]?
        public var head: String?

        public init(id: String, repo: String, number: Int, title: String? = nil, summary: String? = nil, files: [String]? = nil, head: String? = nil) {
            self.id = id
            self.repo = repo
            self.number = number
            self.title = title
            self.summary = summary
            self.files = files
            self.head = head
        }
    }

    /// PRGroups is a set of pull requests grouped by what they change, each with a
    /// head-pinned verdict. `raw` holds the whole block object verbatim, like a pack
    /// block, so re-encoding never drops a field the client does not model.
    public struct PRGroups: Codable, Equatable, Sendable {
        public var id: String
        public var repo: String
        public var prompt: String?
        public var groups: [Group]
        public var raw: JSONValue

        public var rows: [Row] {
            groups.flatMap(\.prs)
        }

        public struct Group: Decodable, Equatable, Sendable {
            public var id: String
            public var title: String
            public var summary: String
            public var prs: [Row]

            public init(id: String, title: String, summary: String, prs: [Row]) {
                self.id = id
                self.title = title
                self.summary = summary
                self.prs = prs
            }
        }

        /// Row is one pull request: its verdict key, the heads its verdict stands at,
        /// and the merge facts the human decides on.
        public struct Row: Decodable, Equatable, Sendable, Identifiable {
            public var number: Int
            public var head: String
            public var title: String
            public var change: String
            public var plan: String
            public var risk: String
            public var review: String
            public var blocker: String
            public var md: String?
            public var carried: [String]?
            public var files: [String]?
            public var stack: [StackMember]?

            public var id: String {
                String(number)
            }

            public init(
                number: Int,
                head: String,
                title: String,
                change: String,
                plan: String,
                risk: String,
                review: String,
                blocker: String,
                md: String? = nil,
                carried: [String]? = nil,
                files: [String]? = nil,
                stack: [StackMember]? = nil
            ) {
                self.number = number
                self.head = head
                self.title = title
                self.change = change
                self.plan = plan
                self.risk = risk
                self.review = review
                self.blocker = blocker
                self.md = md
                self.carried = carried
                self.files = files
                self.stack = stack
            }
        }

        public struct StackMember: Decodable, Equatable, Sendable {
            public var number: Int
            public var head: String

            public init(number: Int, head: String) {
                self.number = number
                self.head = head
            }
        }

        public enum RowVerdict: Equatable, Sendable {
            case undecided
            case current(Decision, carried: Bool)
            case stale(Decision)
        }

        public struct Tally: Equatable, Sendable {
            public var approved: Int
            public var held: Int
            public var open: Int

            public init(approved: Int = 0, held: Int = 0, open: Int = 0) {
                self.approved = approved
                self.held = held
                self.open = open
            }
        }

        private enum CodingKeys: String, CodingKey {
            case id, repo, prompt, groups
        }

        public init(from decoder: Decoder) throws {
            let container = try decoder.container(keyedBy: CodingKeys.self)
            id = try container.decode(String.self, forKey: .id)
            repo = try container.decode(String.self, forKey: .repo)
            prompt = try container.decodeIfPresent(String.self, forKey: .prompt)
            groups = try container.decode([Group].self, forKey: .groups)
            raw = try JSONValue(from: decoder)
        }

        public func encode(to encoder: Encoder) throws {
            try raw.encode(to: encoder)
        }

        /// stands reports whether a recorded verdict holds for the row's current head:
        /// given at that head or at one the agent carried. Mirrors web/src/prgroups.ts.
        public static func stands(_ decision: Decision?, for row: Row) -> Bool {
            if case .current = rowVerdict(row, decision) {
                return true
            }
            return false
        }

        /// sameHead mirrors doc.SameHead: the shorter sha is a prefix of the longer.
        public static func sameHead(_ a: String, _ b: String) -> Bool {
            guard !a.isEmpty, !b.isEmpty else { return false }
            return a.count <= b.count ? b.hasPrefix(a) : a.hasPrefix(b)
        }

        public static func shortHead(_ head: String) -> String {
            String(head.prefix(7))
        }

        public static func rowVerdict(_ row: Row, _ decision: Decision?) -> RowVerdict {
            guard let decision else { return .undecided }
            let head = decision.head ?? ""
            if sameHead(head, row.head) {
                return .current(decision, carried: false)
            }
            if (row.carried ?? []).contains(where: { sameHead($0, head) }) {
                return .current(decision, carried: true)
            }
            return .stale(decision)
        }

        public static func tally(_ rows: [Row], _ verdicts: [String: Decision]) -> Tally {
            var out = Tally()
            for row in rows {
                guard case let .current(decision, _) = rowVerdict(row, verdicts[row.id]) else {
                    out.open += 1
                    continue
                }
                if decision.verdict == "approved" {
                    out.approved += 1
                } else {
                    out.held += 1
                }
            }
            return out
        }

        public static func toggleEntry(_ row: Row, _ decision: Decision?, target: String) -> TriageVerdict {
            if case let .current(standing, _) = rowVerdict(row, decision), standing.verdict == target {
                return TriageVerdict(verdict: "cleared")
            }
            return TriageVerdict(verdict: target, note: decision?.note, head: row.head)
        }

        public static func approveGroup(_ group: Group, _ verdicts: [String: Decision]) -> [String: TriageVerdict] {
            var out: [String: TriageVerdict] = [:]
            for row in group.prs {
                if case let .current(decision, _) = rowVerdict(row, verdicts[row.id]), decision.verdict == "approved" {
                    continue
                }
                out[row.id] = TriageVerdict(verdict: "approved", note: verdicts[row.id]?.note, head: row.head)
            }
            return out
        }

        public static func staleLine(_ row: Row, _ decision: Decision) -> String {
            let given = (decision.carriedFrom ?? decision.head).map { " at \(shortHead($0))" } ?? ""
            return "\(verdictWord(decision.verdict))\(given) on \(formatAt(decision.at)); new head \(shortHead(row.head)) needs your tap."
        }

        public static func givenLine(_ row: Row, _ decision: Decision, carried: Bool) -> String {
            let at = decision.at.map { " on \(formatAt($0))" } ?? ""
            let head = shortHead(decision.head ?? row.head)
            if let tapped = decision.carriedFrom {
                return "Your tap at \(shortHead(tapped))\(at) carries to \(head): \(decision.carryNote ?? "")"
            }
            let carry = carried ? "; the agent carried it to \(shortHead(row.head))" : ""
            return "\(verdictWord(decision.verdict)) at \(head)\(at)\(carry)"
        }

        private static func verdictWord(_ verdict: String) -> String {
            verdict == "approved" ? "Approved" : "Held"
        }

        private static func formatAt(_ iso: String?) -> String {
            guard let date = iso.flatMap(parseTimestamp) else { return "an earlier time" }
            return date.formatted(.dateTime.month(.abbreviated).day().hour().minute())
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
        case let .draft(block): block.id
        case let .triage(block): block.id
        case let .input(block): block.id
        case let .markdown(block): block.id
        case let .code(block): block.id
        case let .diff(block): block.id
        case let .diagram(block): block.id
        case let .image(block): block.id
        case let .table(block): block.id
        case let .progress(block): block.id
        case let .chart(block): block.id
        case let .term(block): block.id
        case let .filetree(block): block.id
        case let .record(block): block.id
        case let .pr(block): block.id
        case let .prGroups(block): block.id
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
        case .draft: "draft"
        case .triage: "triage"
        case .input: "input"
        case .markdown: "markdown"
        case .code: "code"
        case .diff: "diff"
        case .diagram: "diagram"
        case .image: "image"
        case .table: "table"
        case .progress: "progress"
        case .chart: "chart"
        case .term: "term"
        case .filetree: "filetree"
        case .record: "record"
        case .pr: "pr"
        case .prGroups: "pr-groups"
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
        case "draft": self = try .draft(Draft(from: decoder))
        case "triage": self = try .triage(Triage(from: decoder))
        case "input": self = try .input(Input(from: decoder))
        case "markdown": self = try .markdown(Markdown(from: decoder))
        case "code": self = try .code(Code(from: decoder))
        case "diff": self = try .diff(Diff(from: decoder))
        case "diagram": self = try .diagram(Diagram(from: decoder))
        case "image": self = try .image(Image(from: decoder))
        case "table": self = try .table(Table(from: decoder))
        case "progress": self = try .progress(Progress(from: decoder))
        case "chart": self = try .chart(Chart(from: decoder))
        case "term": self = try .term(Term(from: decoder))
        case "filetree": self = try .filetree(FileTree(from: decoder))
        case "record": self = try .record(Record(from: decoder))
        case "pr": self = try .pr(PR(from: decoder))
        case "pr-groups": self = try .prGroups(PRGroups(from: decoder))
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
        if case let .prGroups(block) = self {
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
        case let .draft(block): try block.encode(to: encoder)
        case let .triage(block): try block.encode(to: encoder)
        case let .input(block): try block.encode(to: encoder)
        case let .markdown(block): try block.encode(to: encoder)
        case let .code(block): try block.encode(to: encoder)
        case let .diff(block): try block.encode(to: encoder)
        case let .diagram(block): try block.encode(to: encoder)
        case let .image(block): try block.encode(to: encoder)
        case let .table(block): try block.encode(to: encoder)
        case let .progress(block): try block.encode(to: encoder)
        case let .chart(block): try block.encode(to: encoder)
        case let .term(block): try block.encode(to: encoder)
        case let .filetree(block): try block.encode(to: encoder)
        case let .record(block): try block.encode(to: encoder)
        case let .pr(block): try block.encode(to: encoder)
        case .prGroups, .pack: break
        }
    }
}

/// OptionVisual is the restricted leaf visual an option may carry. The cases
/// encode the allowlist in the type system: a recursive `Block?` cannot compile
/// (Block → Choice → Option → Block), so the visual is this narrow enum instead.
/// Its discriminator init/encode mirror Block's, but only these types decode.
public enum OptionVisual: Codable, Equatable, Sendable {
    case code(Block.Code)
    case diagram(Block.Diagram)
    case image(Block.Image)
    case diff(Block.Diff)
    case chart(Block.Chart)
    case term(Block.Term)
    case filetree(Block.FileTree)
    case record(Block.Record)

    private enum Discriminator: String, CodingKey {
        case type
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: Discriminator.self)
        let type = try container.decode(String.self, forKey: .type)
        switch type {
        case "code": self = try .code(Block.Code(from: decoder))
        case "diagram": self = try .diagram(Block.Diagram(from: decoder))
        case "image": self = try .image(Block.Image(from: decoder))
        case "diff": self = try .diff(Block.Diff(from: decoder))
        case "chart": self = try .chart(Block.Chart(from: decoder))
        case "term": self = try .term(Block.Term(from: decoder))
        case "filetree": self = try .filetree(Block.FileTree(from: decoder))
        case "record": self = try .record(Block.Record(from: decoder))
        default:
            throw DecodingError.dataCorrupted(
                DecodingError.Context(
                    codingPath: decoder.codingPath,
                    debugDescription: "option visual: type \"\(type)\" is not an allowed visual (code, diagram, image, diff, chart, term, filetree, record)"
                )
            )
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: Discriminator.self)
        switch self {
        case let .code(block):
            try container.encode("code", forKey: .type)
            try block.encode(to: encoder)
        case let .diagram(block):
            try container.encode("diagram", forKey: .type)
            try block.encode(to: encoder)
        case let .image(block):
            try container.encode("image", forKey: .type)
            try block.encode(to: encoder)
        case let .diff(block):
            try container.encode("diff", forKey: .type)
            try block.encode(to: encoder)
        case let .chart(block):
            try container.encode("chart", forKey: .type)
            try block.encode(to: encoder)
        case let .term(block):
            try container.encode("term", forKey: .type)
            try block.encode(to: encoder)
        case let .filetree(block):
            try container.encode("filetree", forKey: .type)
            try block.encode(to: encoder)
        case let .record(block):
            try container.encode("record", forKey: .type)
            try block.encode(to: encoder)
        }
    }
}
