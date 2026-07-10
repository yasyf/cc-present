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
    targets: [
        .target(name: "CcPresentKit"),
        .testTarget(
            name: "CcPresentKitTests",
            dependencies: ["CcPresentKit"]
        ),
    ]
)
