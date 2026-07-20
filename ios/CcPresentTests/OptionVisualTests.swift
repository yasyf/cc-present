@testable import CcPresentApp
import CcPresentKit
import Testing

/// OptionVisualTests pins the option-visual disclosure title per kind, exercising the
/// exhaustive dispatch switch OptionVisualView renders each visual through.
@Suite("Option visual dispatch")
struct OptionVisualTests {
    @Test("each visual kind titles its disclosure, falling back to a type name")
    func titlePerKind() {
        let code = OptionVisual.code(Block.Code(id: "c", lang: "go", code: "x := 1", title: "Sketch"))
        let codeNoTitle = OptionVisual.code(Block.Code(id: "c", lang: "swift", code: "let x = 1"))
        let diagram = OptionVisual.diagram(Block.Diagram(id: "d", kind: "mermaid", source: "graph LR", title: "Flow"))
        let diagramNoTitle = OptionVisual.diagram(Block.Diagram(id: "d", kind: "mermaid", source: "graph LR"))
        let image = OptionVisual.image(Block.Image(id: "i", src: "https://x/y.png", alt: "alt text", caption: "A caption"))
        let imageNoCaption = OptionVisual.image(Block.Image(id: "i", src: "https://x/y.png", alt: "alt text"))
        let diff = OptionVisual.diff(Block.Diff(id: "df", diff: "@@ -1 +1 @@", title: "Patch"))
        let diffNoTitle = OptionVisual.diff(Block.Diff(id: "df", diff: "@@ -1 +1 @@"))
        let series = [Block.Series(label: "s", values: [1])]
        let chart = OptionVisual.chart(Block.Chart(id: "ch", kind: "bar", title: "Latency", categories: ["a"], series: series))
        let chartNoTitle = OptionVisual.chart(Block.Chart(id: "ch", kind: "bar", categories: ["a"], series: series))
        let term = OptionVisual.term(Block.Term(id: "t", output: "ok", title: "Build"))
        let termNoTitle = OptionVisual.term(Block.Term(id: "t", output: "ok"))
        let filetree = OptionVisual.filetree(Block.FileTree(id: "f", title: "Changed", entries: [Block.TreeEntry(path: "a")]))
        let filetreeNoTitle = OptionVisual.filetree(Block.FileTree(id: "f", entries: [Block.TreeEntry(path: "a")]))
        let record = OptionVisual.record(Block.Record(id: "r", title: "AA123", facts: [Block.Fact(value: "x")]))
        let recordNoTitle = OptionVisual.record(Block.Record(id: "r", facts: [Block.Fact(value: "x")]))

        #expect(optionVisualTitle(code) == "Sketch")
        #expect(optionVisualTitle(codeNoTitle) == "swift")
        #expect(optionVisualTitle(diagram) == "Flow")
        #expect(optionVisualTitle(diagramNoTitle) == "Diagram")
        #expect(optionVisualTitle(image) == "A caption")
        #expect(optionVisualTitle(imageNoCaption) == "alt text")
        #expect(optionVisualTitle(diff) == "Patch")
        #expect(optionVisualTitle(diffNoTitle) == "Diff")
        #expect(optionVisualTitle(chart) == "Latency")
        #expect(optionVisualTitle(chartNoTitle) == "Chart")
        #expect(optionVisualTitle(term) == "Build")
        #expect(optionVisualTitle(termNoTitle) == "Terminal")
        #expect(optionVisualTitle(filetree) == "Changed")
        #expect(optionVisualTitle(filetreeNoTitle) == "Files")
        #expect(optionVisualTitle(record) == "AA123")
        #expect(optionVisualTitle(recordNoTitle) == "Record")
    }
}
