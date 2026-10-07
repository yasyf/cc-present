import CcPresentKit
import SwiftUI

/// blockViewIds is what one viewed mark covers: each block plus its card children,
/// since opening a card shows every block nested in it. Mirrors web/src/viewed.ts.
func blockViewIds(_ blocks: [Block]) -> [String] {
    flatten(blocks).map(\.id)
}

extension View {
    /// viewedOnScreen marks `block` viewed once its board row stays on screen for the
    /// board dwell; a changed block or round re-arms the wait.
    func viewedOnScreen(_ block: Block, round: Int, store: ViewedStore) -> some View {
        modifier(ViewedOnScreen(block: block, round: round, store: store))
    }

    /// viewedAfterFocus marks the step's view ids once it stays up for the focus dwell; a
    /// change to the ids, the focal block, or the round restarts the wait.
    func viewedAfterFocus(_ step: FocusStep, round: Int, store: ViewedStore) -> some View {
        modifier(ViewedAfterFocus(key: FocusDwellKey(step: step, round: round), store: store))
    }
}

private struct ViewedOnScreen: ViewModifier {
    let block: Block
    let round: Int
    let store: ViewedStore

    @State private var dwell = Dwell<ContinuousClock.Instant>(duration: ViewedStore.boardDwell)
    @State private var onScreen = false
    @State private var deadline: ContinuousClock.Instant?

    func body(content: Content) -> some View {
        content
            .onGeometryChange(for: Bool.self) { proxy in
                guard let viewport = proxy.bounds(of: .scrollView) else { return false }
                return isViewedOnScreen(row: CGRect(origin: .zero, size: proxy.size), viewport: viewport)
            } action: { visible in
                onScreen = visible
                deadline = dwell.update(active: visible, at: .now)
            }
            .onChange(of: block) { rearm() }
            .onChange(of: round) { rearm() }
            .task(id: deadline) {
                guard let deadline else { return }
                do {
                    try await Task.sleep(until: deadline, clock: .continuous)
                } catch {
                    return
                }
                if dwell.complete(at: .now) {
                    store.mark(blockViewIds([block]))
                }
            }
    }

    private func rearm() {
        dwell = Dwell(duration: ViewedStore.boardDwell)
        deadline = dwell.update(active: onScreen, at: .now)
    }
}

/// FocusDwellKey is what restarts the focus dwell, matching the web FocusCard effect's
/// dependencies: the step's view ids, its focal block's content, and the round.
struct FocusDwellKey: Equatable {
    let ids: [String]
    let block: Block
    let round: Int

    init(step: FocusStep, round: Int) {
        ids = blockViewIds(step.context + [step.block])
        block = step.block
        self.round = round
    }
}

private struct ViewedAfterFocus: ViewModifier {
    let key: FocusDwellKey
    let store: ViewedStore

    @State private var dwell = Dwell<ContinuousClock.Instant>(duration: ViewedStore.focusDwell)

    func body(content: Content) -> some View {
        content.task(id: key) {
            dwell = Dwell(duration: ViewedStore.focusDwell)
            guard let deadline = dwell.update(active: true, at: .now) else { return }
            do {
                try await Task.sleep(until: deadline, clock: .continuous)
            } catch {
                return
            }
            if dwell.complete(at: .now) {
                store.mark(key.ids)
            }
        }
    }
}
