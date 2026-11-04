import CcPresentKit
import IOSKit
import IOSKitUI
import SwiftUI

struct PRBlockView: View {
    let block: Block.PR
    var context: PackContext?

    @State private var pr: PRLive?
    @State private var failure: String?
    @State private var fetching = false
    @State private var reload = 0

    private var url: URL? {
        pr?.url ?? URL(string: "https://github.com/\(block.repo)/pull/\(block.number)")
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space3) {
            header
            if let summary = block.summary, !summary.isEmpty {
                MarkdownText(summary, style: .clamped)
            }
            if context != nil {
                live
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(Metrics.space3)
        .background(Palette.chipBg, in: RoundedRectangle(cornerRadius: Metrics.radiusLg))
        .overlay(RoundedRectangle(cornerRadius: Metrics.radiusLg).stroke(Palette.line, lineWidth: 1))
        .task(id: reload) {
            await load()
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Metrics.space1) {
            HStack(spacing: Metrics.space2) {
                if let pr {
                    PRPill(text: pr.state.rawValue, tint: prStateTint(pr.state))
                }
                if let url {
                    Link("\(block.repo)#\(block.number)", destination: url)
                        .voice(.mono, size: 12, weight: .semibold)
                        .foregroundStyle(Palette.accentInk)
                }
                if let merged = pr?.mergedSha, let commit = URL(string: "https://github.com/\(block.repo)/commit/\(merged)") {
                    Link(String(merged.prefix(10)), destination: commit)
                        .voice(.mono, size: 11)
                        .foregroundStyle(Palette.muted)
                }
            }
            Text(block.title ?? pr?.title ?? "Pull request #\(block.number)")
                .voice(.prose, size: 15, weight: .semibold)
                .foregroundStyle(Palette.ink)
            if let pr {
                meta(pr)
                if let asked = block.head, !Block.PRGroups.sameHead(asked, pr.headSha) {
                    Text("Asked at \(Block.PRGroups.shortHead(asked)); the head is now \(Block.PRGroups.shortHead(pr.headSha)).")
                        .voice(.prose, size: 12, weight: .semibold)
                        .foregroundStyle(Palette.warn)
                }
            }
        }
    }

    private func meta(_ pr: PRLive) -> some View {
        let sha = Text(Block.PRGroups.shortHead(pr.headSha)).foregroundStyle(Palette.accentInk)
        let added = Text("+\(pr.additions)").foregroundStyle(Palette.approve)
        let deleted = Text("−\(pr.deletions)").foregroundStyle(Palette.reject)
        let files = pr.changedFiles == 1 ? "1 file" : "\(pr.changedFiles) files"
        return Text("\(pr.author) · \(pr.base) ← \(pr.head) \(sha) · \(added) \(deleted) · \(files)")
            .voice(.mono, size: 11)
            .foregroundStyle(Palette.muted)
    }

    @ViewBuilder
    private var live: some View {
        if let failure {
            Text(failure)
                .voice(.prose, size: 12)
                .foregroundStyle(Palette.reject)
                .textSelection(.enabled)
        }
        if let pr {
            PRFacts(pr: pr)
            PRFiles(pr: pr, filtered: !(block.files ?? []).isEmpty)
        } else if failure == nil {
            Text("Loading from GitHub…")
                .voice(.prose, size: 12)
                .foregroundStyle(Palette.muted)
        }
        footer
    }

    private var footer: some View {
        HStack(spacing: Metrics.space2) {
            if let pr, let fetched = parseTimestamp(pr.fetchedAt) {
                TimelineView(.everyMinute) { timeline in
                    Text("fetched \(prFetchedAgo(fetched, now: timeline.date))")
                        .voice(.prose, size: 11)
                        .foregroundStyle(Palette.muted)
                }
            }
            Spacer(minLength: Metrics.space2)
            Button(fetching ? "Refreshing…" : "Refresh") {
                reload += 1
            }
            .buttonStyle(GhostButtonStyle())
            .disabled(fetching)
            if let url {
                Link("Open on GitHub", destination: url)
                    .voice(.prose, size: 14, weight: .semibold)
                    .foregroundStyle(Palette.accentInk)
            }
        }
    }

    private func load() async {
        guard let context else { return }
        fetching = true
        do {
            pr = try await HTTPClient(baseURL: context.baseURL, bearerToken: context.token)
                .pr(subject: context.subject, blockId: block.id, repo: block.repo, number: block.number)
            failure = nil
        } catch {
            failure = prFailureText(error)
        }
        fetching = false
    }
}

private struct PRFacts: View {
    let pr: PRLive

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space2) {
            if pr.checks.total > 0 {
                fact("CI") {
                    HStack(spacing: Metrics.space2) {
                        PRPill(text: pr.checks.state, tint: prToneColor(pr.checks.state))
                        Text(prChecksSummary(pr.checks))
                            .voice(.prose, size: 13)
                            .foregroundStyle(Palette.ink)
                    }
                    if let failing = pr.checks.failing, !failing.isEmpty {
                        Text(failing.joined(separator: ", "))
                            .voice(.mono, size: 11)
                            .foregroundStyle(Palette.reject)
                    }
                }
            }
            if pr.reviewDecision != "none" || !pr.reviews.isEmpty {
                fact("Reviews") {
                    ScrollView(.horizontal) {
                        HStack(spacing: Metrics.space2) {
                            if pr.reviewDecision != "none" {
                                PRPill(text: prWords(pr.reviewDecision), tint: prToneColor(pr.reviewDecision))
                            }
                            ForEach(pr.reviews, id: \.author) { review in
                                Text("\(review.author) \(prWords(review.state))")
                                    .voice(.prose, size: 13)
                                    .foregroundStyle(prToneColor(review.state))
                            }
                        }
                    }
                    .scrollIndicators(.hidden)
                }
            }
            if !pr.labels.isEmpty {
                fact("Labels") {
                    ScrollView(.horizontal) {
                        HStack(spacing: Metrics.space1) {
                            ForEach(pr.labels, id: \.self) { label in
                                PRPill(text: label, tint: Palette.muted)
                            }
                        }
                    }
                    .scrollIndicators(.hidden)
                }
            }
        }
    }

    private func fact(_ label: String, @ViewBuilder content: () -> some View) -> some View {
        HStack(alignment: .top, spacing: Metrics.space3) {
            Text(label)
                .voice(.stamp, size: 11, weight: .semibold)
                .foregroundStyle(Palette.muted)
                .frame(width: 64, alignment: .leading)
            VStack(alignment: .leading, spacing: Metrics.space1) {
                content()
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

private struct PRFiles: View {
    let pr: PRLive
    let filtered: Bool

    var body: some View {
        if pr.files.isEmpty {
            Text(filtered ? "No changed file matches this block’s files filter." : "No changed files.")
                .voice(.prose, size: 12)
                .foregroundStyle(Palette.muted)
        } else {
            VStack(alignment: .leading, spacing: Metrics.space1) {
                Text(filtered ? "\(pr.files.count) of \(pr.changedFiles) files" : "\(pr.files.count) files")
                    .voice(.stamp, size: 11, weight: .semibold)
                    .foregroundStyle(Palette.muted)
                ForEach(pr.files, id: \.path) { file in
                    PRFileRow(file: file)
                }
            }
        }
    }
}

private struct PRFileRow: View {
    let file: PRLive.File

    @State private var open = false

    var body: some View {
        VStack(alignment: .leading, spacing: Metrics.space1) {
            Button {
                open.toggle()
            } label: {
                HStack(spacing: Metrics.space2) {
                    Image(systemName: open ? "chevron.down" : "chevron.right")
                        .font(.system(size: 10, weight: .semibold))
                        .foregroundStyle(Palette.muted)
                        .opacity(file.patch == nil ? 0 : 1)
                    Text(prFileStatusLetter(file.status))
                        .voice(.mono, size: 12, weight: .bold)
                        .foregroundStyle(file.status == "added" ? Palette.approve : file.status == "removed" ? Palette.reject : Palette.muted)
                    Text(file.previousPath.map { "\($0) → \(file.path)" } ?? file.path)
                        .voice(.mono, size: 12)
                        .foregroundStyle(Palette.ink)
                        .lineLimit(1)
                        .truncationMode(.middle)
                    if file.patch == nil {
                        Text("no text diff")
                            .voice(.prose, size: 11)
                            .foregroundStyle(Palette.muted)
                    }
                    Spacer(minLength: Metrics.space2)
                    Text("+\(file.additions)")
                        .voice(.mono, size: 11)
                        .foregroundStyle(Palette.approve)
                    Text("−\(file.deletions)")
                        .voice(.mono, size: 11)
                        .foregroundStyle(Palette.reject)
                }
                .frame(minHeight: 32)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .disabled(file.patch == nil)
            if open {
                DiffBlockView(block: Block.Diff(id: "pr-file:\(file.path)", diff: prFilePatch(file)))
            }
        }
    }
}

private struct PRPill: View {
    let text: String
    let tint: Color

    var body: some View {
        Text(text)
            .voice(.stamp, size: 10, weight: .semibold)
            .foregroundStyle(tint)
            .padding(.horizontal, 6)
            .padding(.vertical, 2)
            .background(tint.opacity(0.1), in: Capsule())
            .overlay(Capsule().strokeBorder(tint.opacity(0.45)))
            .lineLimit(1)
    }
}

private func prStateTint(_ state: PRLive.State) -> Color {
    switch state {
    case .open: Palette.accentInk
    case .draft: Palette.muted
    case .merged, .landed: Palette.approve
    case .closed: Palette.reject
    }
}

private func prToneColor(_ verdict: String) -> Color {
    Palette.factTone(prTone(verdict), fallback: Palette.muted)
}

private func prWords(_ state: String) -> String {
    state.replacingOccurrences(of: "_", with: " ")
}

func prTone(_ verdict: String) -> String {
    switch verdict {
    case "approved", "success": "good"
    case "changes_requested", "failure", "error": "bad"
    case "pending", "expected", "review_required": "warn"
    default: "dim"
    }
}

func prChecksSummary(_ checks: PRLive.Checks) -> String {
    var parts = ["\(checks.passed)/\(checks.total) passed"]
    if checks.failed > 0 {
        parts.append("\(checks.failed) failed")
    }
    if checks.pending > 0 {
        parts.append("\(checks.pending) pending")
    }
    if checks.skipped > 0 {
        parts.append("\(checks.skipped) skipped")
    }
    return parts.joined(separator: " · ")
}

func prFetchedAgo(_ fetched: Date, now: Date) -> String {
    let seconds = max(0, Int(now.timeIntervalSince(fetched).rounded()))
    if seconds < 60 {
        return "just now"
    }
    let minutes = Int((Double(seconds) / 60).rounded())
    if minutes < 60 {
        return "\(minutes)m ago"
    }
    return "\(Int((Double(minutes) / 60).rounded()))h ago"
}

func prFilePatch(_ file: PRLive.File) -> String {
    "--- a/\(file.previousPath ?? file.path)\n+++ b/\(file.path)\n\(file.patch ?? "")"
}

func prFileStatusLetter(_ status: String) -> String {
    switch status {
    case "added": "A"
    case "removed": "D"
    case "modified", "changed": "M"
    case "renamed": "R"
    case "copied": "C"
    case "unchanged": "="
    default: status
    }
}

func prFailureText(_ error: Error) -> String {
    guard case let APIError.status(code, body) = error else {
        return error.localizedDescription
    }
    let text = body.trimmingCharacters(in: .whitespacesAndNewlines)
    return text.isEmpty ? "GitHub fetch failed (\(code))" : text
}
