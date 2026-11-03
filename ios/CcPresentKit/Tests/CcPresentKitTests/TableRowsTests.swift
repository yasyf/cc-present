@testable import CcPresentKit
import Foundation
import Testing

@Suite("Table rows")
struct TableRowsTests {
    private let table = Block.Table(
        id: "t",
        columns: [
            Block.Column(key: "team", label: "Team", filter: true),
            Block.Column(key: "behind", label: "Behind", sort: true),
        ],
        rows: [
            ["team": "North", "behind": "10"],
            ["team": "South", "behind": "9"],
            ["team": "South", "behind": "1,200"],
        ]
    )

    @Test("with no query every row stays, in order")
    func noQueryKeepsRows() {
        #expect(table.visibleRows(TableQuery()) == table.rows)
    }

    @Test("a chip and free text both narrow the rows")
    func chipsAndText() {
        #expect(table.visibleRows(TableQuery(chips: ["team": "South"])).count == 2)
        #expect(table.visibleRows(TableQuery(text: " north ")) == [table.rows[0]])
        #expect(table.visibleRows(TableQuery(text: "nowhere")).isEmpty)
    }

    @Test("numbers sort as numbers, both ways")
    func numericSort() {
        func behind(_ descending: Bool) -> [String] {
            table.visibleRows(TableQuery(sort: TableQuery.Sort(key: "behind", descending: descending))).compactMap { $0["behind"] }
        }
        #expect(behind(false) == ["9", "10", "1,200"])
        #expect(behind(true) == ["1,200", "10", "9"])
    }

    @Test("a header press sorts ascending first, then flips")
    func headerPressFlips() {
        let once = TableQuery().pressing("behind")
        #expect(once.sort == TableQuery.Sort(key: "behind", descending: false))
        #expect(once.pressing("behind").sort == TableQuery.Sort(key: "behind", descending: true))
        #expect(once.pressing("behind").pressing("behind").sort == TableQuery.Sort(key: "behind", descending: false))
        #expect(once.pressing("team").sort == TableQuery.Sort(key: "team", descending: false))
    }

    @Test("a chip toggles on and off")
    func chipToggles() {
        let on = TableQuery().toggling(chip: "South", for: "team")
        #expect(on.chips == ["team": "South"])
        #expect(on.toggling(chip: "South", for: "team").chips.isEmpty)
        #expect(on.toggling(chip: "North", for: "team").chips == ["team": "North"])
    }

    @Test("filter choices list each value once in row order")
    func filterChoicesOnce() {
        #expect(table.filterChoices(table.columns[0]) == ["North", "South"])
    }

    @Test("a table's row key, details, and actions decode")
    func interactiveFieldsDecode() throws {
        let json = #"""
        {"id":"t2","type":"table","columns":[{"key":"pr","label":"PR","sort":true},{"key":"state","label":"State","filter":true}],
         "rows":[{"pr":"412","state":"open"}],"rowKey":"pr","details":{"412":"Waits on **review**."},
         "actions":[{"id":"nudge","label":"Nudge"},{"id":"hold","label":"Hold","target":"talk"}]}
        """#
        guard case let .table(decoded) = try JSONDecoder().decode(Block.self, from: Data(json.utf8)) else {
            Issue.record("expected a table"); return
        }
        #expect(decoded.rowKey == "pr")
        #expect(decoded.details == ["412": "Waits on **review**."])
        #expect(decoded.actions == [Block.TableAction(id: "nudge", label: "Nudge"), Block.TableAction(id: "hold", label: "Hold", target: "talk")])
        #expect(decoded.columns.map(\.sort) == [true, nil])
    }
}
