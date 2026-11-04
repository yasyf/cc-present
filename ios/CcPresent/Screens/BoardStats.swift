import Darwin
import IOSKitUI
import Observation
import os
import SwiftUI
import UIKit
import WebKit

/// BoardStats is the hidden readout a long-press on a board's title opens: the
/// process's peak resident memory, the memory left before the system's limit, the
/// web views alive now, and the time from opening the board to its content. It
/// exists to compare hosted web blocks against native ones on the phone (spike V2).
@MainActor
@Observable
final class BoardStats {
    private let opened = ContinuousClock.now
    private(set) var contentAfter: Duration?
    private(set) var peakResident: Int64 = 0
    private(set) var available: Int64 = 0
    private(set) var webViews = 0

    func contentShown() {
        if contentAfter == nil {
            contentAfter = ContinuousClock.now - opened
        }
    }

    func sample() {
        var usage = rusage()
        getrusage(RUSAGE_SELF, &usage)
        peakResident = Int64(usage.ru_maxrss)
        available = Int64(os_proc_available_memory())
        webViews = UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .reduce(0) { $0 + Self.webViews(in: $1) }
    }

    static func webViews(in view: UIView) -> Int {
        (view is WKWebView ? 1 : 0) + view.subviews.reduce(0) { $0 + webViews(in: $1) }
    }
}

struct BoardStatsView: View {
    let stats: BoardStats

    var body: some View {
        NavigationStack {
            List {
                Section {
                    row("Peak memory", ByteCountFormatter.string(fromByteCount: stats.peakResident, countStyle: .memory))
                    row("Memory left", ByteCountFormatter.string(fromByteCount: stats.available, countStyle: .memory))
                    row("Web views alive", "\(stats.webViews)")
                    row("Open to content", stats.contentAfter.map { $0.formatted(.units(allowed: [.seconds, .milliseconds], width: .narrow)) } ?? "loading")
                } footer: {
                    Text("Peak memory counts since launch, so cold-launch the app before each board.")
                }
            }
            .navigationTitle("Board stats")
            .navigationBarTitleDisplayMode(.inline)
        }
        .task {
            while !Task.isCancelled {
                stats.sample()
                try? await Task.sleep(for: .seconds(1))
            }
        }
    }

    private func row(_ label: String, _ value: String) -> some View {
        HStack {
            Text(label)
            Spacer()
            Text(value)
                .voice(.mono, size: 15)
                .foregroundStyle(Palette.muted)
        }
    }
}
