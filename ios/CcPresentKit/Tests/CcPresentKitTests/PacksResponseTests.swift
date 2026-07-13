import CcPresentKit
import Foundation
import Testing

/// Decodes the /api/packs manifest fixture (docs/contract.md — PacksResponse) and
/// checks the interactive-type projection the focus deck classifies by. The fixture
/// carries the full contract shape (hostApi, per-pack bundle/version, per-block
/// schema/interaction, dropped) so the decode stays tolerant of the fields the client
/// does not model.
@Suite("Packs manifest")
struct PacksResponseTests {
    @Test("interactiveTypes selects the interactive block types across every pack")
    func decodesInteractiveTypes() throws {
        let json = #"""
        {
          "hostApi": 1,
          "packs": [
            {
              "name": "example",
              "version": "0.1.0",
              "description": "reference pack",
              "bundle": "/packs/example/dist/index.js?v=0.1.0",
              "styles": "/packs/example/dist/index.css?v=0.1.0",
              "blocks": [
                {"type": "example.rating", "interactive": true, "schema": {"type": "object"}, "interaction": {"type": "object"}},
                {"type": "example.callout", "interactive": false, "schema": {"type": "object"}}
              ]
            },
            {
              "name": "charts",
              "version": "1.2.0",
              "bundle": "/packs/charts/dist/index.js?v=1.2.0",
              "blocks": [
                {"type": "charts.vote", "interactive": true, "schema": {}}
              ]
            }
          ],
          "dropped": [{"dir": "broken", "reason": "manifest failed validation"}]
        }
        """#

        let response = try JSONDecoder().decode(PacksResponse.self, from: Data(json.utf8))

        #expect(response.interactiveTypes == ["example.rating", "charts.vote"])
    }

    @Test("an empty manifest classifies no pack as interactive")
    func emptyManifestClassifiesNothing() throws {
        let json = #"{"hostApi":1,"packs":[],"dropped":[]}"#

        let response = try JSONDecoder().decode(PacksResponse.self, from: Data(json.utf8))

        #expect(response.interactiveTypes.isEmpty)
    }
}
