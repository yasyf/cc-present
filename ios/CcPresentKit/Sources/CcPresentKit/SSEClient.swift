// cc-present's GET /events plane over ioskit's EventSource. The server (cc-interact's
// sse package) replays a subject's log from seq 0, marks the replay/live boundary
// with a named `caught-up` event carrying {seq}, then streams live frames.

import Foundation
import IOSKit

/// SSEClient streams a cc-present subject's event log. One `connect()` yields a
/// message stream (the caught-up marker plus live frames) and a connection-state
/// stream; cancelling the task consuming the messages tears the connection down.
/// The client resumes with Last-Event-ID and reconnects on its own, resetting its
/// backoff once a connection catches up.
public struct SSEClient: Sendable {
    /// Message is one decoded item from the stream: the replay/live boundary marker,
    /// or a live event frame.
    public enum Message: Equatable, Sendable {
        case caughtUp(seq: Int64)
        case frame(Event)
    }

    public typealias ConnectionState = EventSource.ConnectionState

    /// Connection bundles the two streams a `connect()` produces. Consume `messages`
    /// to drive the connection; cancelling that consumer's task tears it down.
    public struct Connection: Sendable {
        public let messages: AsyncStream<Message>
        public let states: AsyncStream<ConnectionState>
    }

    private let source: EventSource

    /// Creates a client for the subject `session` at `baseURL` (e.g.
    /// `http://127.0.0.1:8765`), resuming after `lastEventID` when set.
    public init(
        baseURL: URL,
        session: String,
        lastEventID: Int64? = nil,
        bearerToken: String? = nil,
        urlSession: URLSession? = nil,
        requestTimeout: TimeInterval = 60
    ) {
        source = EventSource(
            url: baseURL
                .appending(path: "events")
                .appending(queryItems: [URLQueryItem(name: "session", value: session)]),
            lastEventID: lastEventID.map(String.init),
            bearerToken: bearerToken,
            urlSession: urlSession,
            requestTimeout: requestTimeout,
            resetsBackoff: { $0.event == "caught-up" }
        )
    }

    public func connect() async -> Connection {
        let upstream = await source.connect()
        let (messages, continuation) = AsyncStream<Message>.makeStream(bufferingPolicy: .unbounded)
        let task = Task {
            for await event in upstream.events {
                if let message = SSEClient.message(from: event) {
                    continuation.yield(message)
                }
            }
            continuation.finish()
        }
        continuation.onTermination = { _ in task.cancel() }
        return Connection(messages: messages, states: upstream.states)
    }

    /// message maps one parsed SSE event onto the client's Message enum: a
    /// `caught-up` named event decodes to its boundary seq, a default frame lifts to
    /// an Event stamped with the frame's id, and anything else is dropped.
    static func message(from event: SSEEvent) -> Message? {
        switch event.event {
        case "caught-up":
            guard let marker = try? JSONDecoder().decode(CaughtUp.self, from: Data(event.data.utf8)) else {
                return nil
            }
            return .caughtUp(seq: marker.seq)
        case "":
            let seq = event.lastEventID.flatMap { Int64($0) }
            guard let frame = try? Event.wireFrame(Data(event.data.utf8), seq: seq) else {
                return nil
            }
            return .frame(frame)
        default:
            return nil
        }
    }

    private struct CaughtUp: Decodable {
        let seq: Int64
    }
}
