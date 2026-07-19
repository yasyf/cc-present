// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "CcPresentKit",
    platforms: [
        .iOS(.v26),
        .macOS(.v26),
    ],
    products: [
        .library(name: "CcPresentKit", targets: ["CcPresentKit"]),
    ],
    dependencies: [
        .package(url: "https://github.com/raspu/Highlightr", from: "2.3.0"),
    ],
    targets: [
        .target(
            name: "CcPresentKit",
            dependencies: [
                .product(name: "Highlightr", package: "Highlightr"),
            ]
        ),
        .testTarget(
            name: "CcPresentKitTests",
            dependencies: ["CcPresentKit"]
        ),
    ]
)
