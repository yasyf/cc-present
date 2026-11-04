import CcPresentKit
import Charts
import IOSKitUI
import SwiftUI

/// ChartBlockView renders a chart block natively with Swift Charts: grouped bars or
/// category-anchored lines with points over a zero baseline, a legend for two or more
/// series, and value labels formatted like web chart.ts.
struct ChartBlockView: View {
    let block: Block.Chart

    static let height: CGFloat = 220

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let title = block.title, !title.isEmpty {
                Text(title)
                    .voice(.mono, size: 11)
                    .foregroundStyle(Palette.muted)
                    .lineLimit(1)
            }
            chart
                .frame(height: Self.height)
                .chartForegroundStyleScale(domain: block.series.map(\.label), range: Self.colors(count: block.series.count))
                .chartLegend(block.series.count > 1 ? .visible : .hidden)
                .chartYAxis {
                    AxisMarks { value in
                        AxisGridLine()
                        AxisValueLabel {
                            if let number = value.as(Double.self) {
                                Text(Self.formatValue(number))
                            }
                        }
                    }
                }
                .accessibilityLabel("\(block.kind) chart, \(block.categories.count) categories, \(block.series.count) series")
        }
        .receiptContent()
    }

    @ViewBuilder
    private var chart: some View {
        let points = Self.points(block)
        if block.kind == "line" {
            Chart(points) { point in
                LineMark(x: .value("Category", point.category), y: .value("Value", point.value))
                    .foregroundStyle(by: .value("Series", point.series))
                PointMark(x: .value("Category", point.category), y: .value("Value", point.value))
                    .foregroundStyle(by: .value("Series", point.series))
            }
        } else {
            Chart(points) { point in
                BarMark(x: .value("Category", point.category), y: .value("Value", point.value))
                    .foregroundStyle(by: .value("Series", point.series))
                    .position(by: .value("Series", point.series))
            }
        }
    }

    struct ChartPoint: Identifiable, Equatable {
        let category: String
        let series: String
        let value: Double

        var id: String {
            "\(series)\u{0}\(category)"
        }
    }

    static func points(_ block: Block.Chart) -> [ChartPoint] {
        block.series.flatMap { series in
            zip(block.categories, series.values).map { ChartPoint(category: $0, series: series.label, value: $1) }
        }
    }

    static func colors(count: Int) -> [Color] {
        let ramp: [Color] = [Palette.accentInk, Palette.approve, Palette.warn, Palette.reject, .purple, .teal, .pink, .brown]
        return (0 ..< count).map { ramp[$0 % ramp.count] }
    }

    static func formatValue(_ value: Double, unit: String? = nil) -> String {
        let number = value != 0 && abs(value) < 1
            ? value.formatted(.number.precision(.significantDigits(1 ... 2)).locale(Locale(identifier: "en_US")))
            : value.formatted(.number.notation(.compactName).precision(.fractionLength(0 ... 1)).locale(Locale(identifier: "en_US")))
        guard let unit, !unit.isEmpty else { return number }
        return "\(number) \(unit)"
    }
}
