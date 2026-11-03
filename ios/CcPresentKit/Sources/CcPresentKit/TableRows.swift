import Foundation

/// TableQuery is what a reader has asked of a table: free text, one chosen value per
/// filter column, and an optional sort. Mirrors web/src/tableRows.ts.
public struct TableQuery: Equatable, Sendable {
    public struct Sort: Equatable, Sendable {
        public var key: String
        public var descending: Bool

        public init(key: String, descending: Bool) {
            self.key = key
            self.descending = descending
        }
    }

    public var text: String
    public var chips: [String: String]
    public var sort: Sort?

    public init(text: String = "", chips: [String: String] = [:], sort: Sort? = nil) {
        self.text = text
        self.chips = chips
        self.sort = sort
    }

    /// pressing a sortable header sorts ascending, then flips on each press.
    public func pressing(_ key: String) -> TableQuery {
        var next = self
        next.sort = Sort(key: key, descending: sort?.key == key && sort?.descending == false)
        return next
    }

    public func toggling(chip value: String, for key: String) -> TableQuery {
        var next = self
        next.chips[key] = chips[key] == value ? nil : value
        return next
    }
}

public extension Block.Table {
    /// filterChoices lists each non-empty value of a column once, in row order.
    func filterChoices(_ column: Block.Column) -> [String] {
        var seen: Set<String> = []
        return rows.compactMap { row in
            guard let value = row[column.key], !value.isEmpty, seen.insert(value).inserted else { return nil }
            return value
        }
    }

    /// visibleRows keeps rows matching every chip and, when given, the text in any
    /// column, then sorts numbers as numbers and anything else as text.
    func visibleRows(_ query: TableQuery) -> [[String: String]] {
        let text = query.text.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        let matched = rows.filter { row in
            query.chips.allSatisfy { row[$0.key] == $0.value }
                && (text.isEmpty || columns.contains { (row[$0.key] ?? "").lowercased().contains(text) })
        }
        guard let sort = query.sort else { return matched }
        return matched.sorted { lhs, rhs in
            let order = Self.compare(lhs[sort.key] ?? "", rhs[sort.key] ?? "")
            return sort.descending ? order == .orderedDescending : order == .orderedAscending
        }
    }

    private static func compare(_ lhs: String, _ rhs: String) -> ComparisonResult {
        if let left = number(lhs), let right = number(rhs) {
            return left < right ? .orderedAscending : left > right ? .orderedDescending : .orderedSame
        }
        return lhs.localizedCompare(rhs)
    }

    private static func number(_ cell: String) -> Double? {
        guard cell.wholeMatch(of: /-?[\d,.]+%?/) != nil else { return nil }
        return Double(cell.replacing(",", with: "").replacing("%", with: ""))
    }
}
