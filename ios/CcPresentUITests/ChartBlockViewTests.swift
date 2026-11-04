@testable import CcPresentUI
import CcPresentKit
import Testing

@Suite("ChartBlockView data")
@MainActor
struct ChartBlockViewTests {
    @Test("points pair each series value with its category, index for index")
    func pointsAlignToCategories() {
        let block = Block.Chart(
            id: "c1",
            kind: "bar",
            categories: ["mon", "tue"],
            series: [Block.Series(label: "p50", values: [1, 2]), Block.Series(label: "p95", values: [3, 4, 5])]
        )
        #expect(ChartBlockView.points(block) == [
            ChartBlockView.ChartPoint(category: "mon", series: "p50", value: 1),
            ChartBlockView.ChartPoint(category: "tue", series: "p50", value: 2),
            ChartBlockView.ChartPoint(category: "mon", series: "p95", value: 3),
            ChartBlockView.ChartPoint(category: "tue", series: "p95", value: 4),
        ])
    }

    @Test("values format like web chart.ts: compact above one, two significant digits below")
    func formatsValues() {
        #expect(ChartBlockView.formatValue(1300) == "1.3K")
        #expect(ChartBlockView.formatValue(4_200_000) == "4.2M")
        #expect(ChartBlockView.formatValue(12) == "12")
        #expect(ChartBlockView.formatValue(0.012) == "0.012")
        #expect(ChartBlockView.formatValue(0) == "0")
        #expect(ChartBlockView.formatValue(250, unit: "ms") == "250 ms")
    }

    @Test("series colors are distinct up to the ramp and cycle after it")
    func seriesColors() {
        let colors = ChartBlockView.colors(count: 9)
        #expect(colors.count == 9)
        #expect(colors[8] == colors[0])
    }
}
