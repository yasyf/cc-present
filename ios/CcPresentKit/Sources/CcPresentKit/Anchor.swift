/// Anchor creates and resolves content-based line anchors.
public enum Anchor {
    /// Ref identifies a line or line range by position and start-line hash.
    public struct Ref: Equatable {
        /// Line is the one-based position hint, or zero for a bare anchor.
        public var line: Int
        public var end: Int
        public var hash: String

        public init(line: Int, end: Int, hash: String) {
            self.line = line
            self.end = end
            self.hash = hash
        }
    }

    /// Resolution reports the resolved line range and any movement from its hint.
    public struct Resolution: Equatable {
        public var start: Int
        public var end: Int
        public var moved: Bool
        public var from: Int

        public init(start: Int, end: Int, moved: Bool, from: Int) {
            self.start = start
            self.end = end
            self.moved = moved
            self.from = from
        }
    }

    /// AnchorError reports malformed references and resolution failures.
    public enum AnchorError: Error {
        case malformed(reference: String, reason: String)
        case notFound(hash: String)
        case ambiguous(hash: String, candidates: [Int])
    }

    private static let letters = Array("abcdefghjkmnpqrstvwxyz".utf8)
    private static let alphabet = Array("0123456789abcdefghjkmnpqrstvwxyz".utf8)

    /// Of returns the four-character anchor hash for line.
    public static func of(_ line: String) -> String {
        var hash: UInt32 = 2_166_136_261
        for byte in trimmed(line).utf8 {
            hash ^= UInt32(byte)
            hash = hash &* 16_777_619
        }

        let value = Int(hash % 720_896)
        return String(decoding: [
            letters[value >> 15],
            alphabet[(value >> 10) & 31],
            alphabet[(value >> 5) & 31],
            alphabet[value & 31],
        ], as: Unicode.UTF8.self)
    }

    /// Parse parses a bare, single-line, or ranged anchor reference.
    public static func parse(_ ref: String) throws -> Ref {
        let parts = ref.split(separator: "#", omittingEmptySubsequences: false)
        if parts.count == 1 {
            guard validHash(ref) else {
                throw AnchorError.malformed(reference: ref, reason: "invalid syntax")
            }
            return Ref(line: 0, end: 0, hash: ref)
        }

        guard parts.count == 2 else {
            throw AnchorError.malformed(reference: ref, reason: "invalid syntax")
        }

        let hash = String(parts[1])
        guard validHash(hash) else {
            throw AnchorError.malformed(reference: ref, reason: "invalid hash")
        }

        let range = parts[0].split(separator: "-", omittingEmptySubsequences: false)
        guard range.count == 1 || range.count == 2, let line = decimal(range[0]) else {
            throw AnchorError.malformed(reference: ref, reason: "invalid line")
        }
        guard line > 0 else {
            throw AnchorError.malformed(reference: ref, reason: "line must be positive")
        }
        guard range.count == 2 else {
            return Ref(line: line, end: line, hash: hash)
        }
        guard let end = decimal(range[1]) else {
            throw AnchorError.malformed(reference: ref, reason: "invalid range end")
        }
        guard end >= line else {
            throw AnchorError.malformed(reference: ref, reason: "range is reversed")
        }
        return Ref(line: line, end: end, hash: hash)
    }

    /// Format returns a single-line anchor reference.
    public static func format(line: Int, hash: String) -> String {
        "\(line)#\(hash)"
    }

    /// FormatRange returns a ranged anchor reference.
    public static func formatRange(start: Int, end: Int, hash: String) -> String {
        "\(start)-\(end)#\(hash)"
    }

    /// Resolve locates ref in lines using its hash and optional line hint.
    public static func resolve(_ ref: Ref, lines: [String]) throws -> Resolution {
        if ref.line > 0, ref.line <= lines.count, of(lines[ref.line - 1]) == ref.hash {
            return resolved(ref, start: ref.line, lineCount: lines.count)
        }

        let candidates = lines.enumerated().compactMap { index, line in
            of(line) == ref.hash ? index + 1 : nil
        }
        guard !candidates.isEmpty else {
            throw AnchorError.notFound(hash: ref.hash)
        }
        if ref.line == 0 {
            guard candidates.count == 1 else {
                throw AnchorError.ambiguous(hash: ref.hash, candidates: candidates)
            }
            return resolved(ref, start: candidates[0], lineCount: lines.count)
        }

        var nearest = candidates[0]
        var nearestDistance = distance(nearest, ref.line)
        for candidate in candidates.dropFirst() {
            let candidateDistance = distance(candidate, ref.line)
            if candidateDistance < nearestDistance {
                nearest = candidate
                nearestDistance = candidateDistance
            }
        }
        return resolved(ref, start: nearest, lineCount: lines.count)
    }

    private static func validHash(_ hash: String) -> Bool {
        let bytes = Array(hash.utf8)
        return bytes.count == 4
            && letters.contains(bytes[0])
            && bytes.dropFirst().allSatisfy(alphabet.contains)
    }

    private static func decimal(_ value: Substring) -> Int? {
        guard !value.isEmpty, value.utf8.allSatisfy({ $0 >= 48 && $0 <= 57 }) else {
            return nil
        }
        return Int(value)
    }

    private static func trimmed(_ line: String) -> String {
        let scalars = line.unicodeScalars
        var start = scalars.startIndex
        while start != scalars.endIndex, isTrimSpace(scalars[start]) {
            scalars.formIndex(after: &start)
        }

        var end = scalars.endIndex
        while end != start {
            let previous = scalars.index(before: end)
            guard isTrimSpace(scalars[previous]) else {
                break
            }
            end = previous
        }
        return String(scalars[start ..< end])
    }

    private static func isTrimSpace(_ scalar: Unicode.Scalar) -> Bool {
        switch scalar.value {
        case 0x0009 ... 0x000D,
             0x0020,
             0x0085,
             0x00A0,
             0x1680,
             0x2000 ... 0x200A,
             0x2028,
             0x2029,
             0x202F,
             0x205F,
             0x3000:
            true
        default:
            false
        }
    }

    private static func resolved(_ ref: Ref, start: Int, lineCount: Int) -> Resolution {
        var end = start
        if ref.line > 0 {
            end = min(ref.end, lineCount) + start - ref.line
            end = max(end, start)
            end = min(end, lineCount)
        }
        let moved = ref.line > 0 && start != ref.line
        return Resolution(start: start, end: end, moved: moved, from: moved ? ref.line : 0)
    }

    private static func distance(_ first: Int, _ second: Int) -> Int {
        first < second ? second - first : first - second
    }
}
