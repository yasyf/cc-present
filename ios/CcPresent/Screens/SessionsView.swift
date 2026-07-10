import CcPresentKit
import Observation
import SwiftUI

/// SessionsProviding is the one call the board list needs from the network: the
/// daemon's roster of live artifacts. APIClient is the production conformer; tests
/// inject a fake so the view model runs without a socket.
protocol SessionsProviding: Sendable {
    func sessions() async throws -> [SessionSummary]
}

extension APIClient: SessionsProviding {}

/// SessionsModel loads one machine's board roster and exposes it as a phase the
/// view renders. Refresh sorts newest-first and maps failures to a display string.
@MainActor
@Observable
final class SessionsModel {
    /// Phase is the load state the list renders: the first fetch shows a spinner,
    /// an empty roster its own placeholder, a failure a retry affordance.
    enum Phase: Equatable {
        case loading
        case loaded
        case empty
        case failed(String)
    }

    private(set) var phase: Phase = .loading
    private(set) var sessions: [SessionSummary] = []

    private let client: any SessionsProviding

    init(client: any SessionsProviding) {
        self.client = client
    }

    /// refresh fetches the roster, keeping the prior list visible while a pull-to-
    /// refresh reload is in flight so the rows don't flash empty.
    func refresh() async {
        if sessions.isEmpty {
            phase = .loading
        }
        do {
            let fetched = try await client.sessions()
            sessions = fetched
                .map { (session: $0, date: parseTimestamp($0.updatedAt) ?? .distantPast) }
                .sorted { $0.date > $1.date }
                .map(\.session)
            phase = sessions.isEmpty ? .empty : .loaded
        } catch {
            phase = .failed(Self.message(for: error))
        }
    }

    private static func message(for error: Error) -> String {
        if case let APIError.status(code, _) = error {
            return "The machine returned an error (\(code))."
        }
        return "Couldn't reach this machine."
    }
}

/// SessionsView lists one machine's boards, newest first, and navigates a tap to
/// the board renderer a later stage fills in.
struct SessionsView: View {
    let machine: Machine

    @State private var model: SessionsModel

    init(machine: Machine) {
        self.machine = machine
        let token = (try? TokenStore.token(machineID: machine.id)) ?? nil
        _model = State(initialValue: SessionsModel(client: APIClient(baseURL: machine.baseURL, bearerToken: token)))
    }

    init(machine: Machine, model: SessionsModel) {
        self.machine = machine
        _model = State(initialValue: model)
    }

    var body: some View {
        content
            .navigationTitle(machine.name)
            .navigationBarTitleDisplayMode(.inline)
            .task { await model.refresh() }
            .refreshable { await model.refresh() }
    }

    @ViewBuilder
    private var content: some View {
        switch model.phase {
        case .loading:
            ProgressView("Loading boards…")
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        case .empty:
            ContentUnavailableView(
                "No Boards",
                systemImage: "rectangle.on.rectangle.slash",
                description: Text("This machine has no live boards yet.")
            )
        case let .failed(message):
            ContentUnavailableView {
                Label("Can't Load Boards", systemImage: "wifi.exclamationmark")
            } description: {
                Text(message)
            } actions: {
                Button("Try Again") { Task { await model.refresh() } }
            }
        case .loaded:
            List(model.sessions) { session in
                NavigationLink {
                    BoardScreen(machine: machine, subject: session.subject)
                } label: {
                    SessionRow(session: session)
                }
            }
        }
    }
}

private struct SessionRow: View {
    let session: SessionSummary

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(session.title.isEmpty ? session.slug : session.title)
                    .font(.headline)
                Spacer()
                if session.status != "open" {
                    Text(session.status.capitalized)
                        .font(.caption2)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(.quaternary, in: Capsule())
                }
            }
            HStack {
                Text(session.slug)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                Spacer()
                Text(relativeTimestamp(session.updatedAt))
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 2)
    }
}

/// parseTimestamp decodes an RFC 3339 instant, tolerating the fractional-seconds
/// the daemon may or may not include.
func parseTimestamp(_ raw: String) -> Date? {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let date = formatter.date(from: raw) {
        return date
    }
    formatter.formatOptions = [.withInternetDateTime]
    return formatter.date(from: raw)
}

/// relativeTimestamp renders an RFC 3339 instant as a relative phrase ("3m ago"),
/// falling back to the raw string when it doesn't parse.
func relativeTimestamp(_ raw: String, relativeTo now: Date = Date()) -> String {
    guard let date = parseTimestamp(raw) else { return raw }
    let formatter = RelativeDateTimeFormatter()
    formatter.unitsStyle = .abbreviated
    return formatter.localizedString(for: date, relativeTo: now)
}
