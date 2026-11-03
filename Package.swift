// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "cc-present",
    platforms: [
        .iOS(.v26),
    ],
    products: [
        .library(name: "CcPresentKit", targets: ["CcPresentKit"]),
        .library(name: "CcPresentUI", targets: ["CcPresentUI"]),
    ],
    dependencies: [
        .package(url: "https://github.com/yasyf/ioskit", branch: "owner-apps/kit"),
        .package(url: "https://github.com/raspu/Highlightr", from: "2.3.0"),
    ],
    targets: [
        .target(
            name: "CcPresentKit",
            dependencies: [
                .product(name: "IOSKit", package: "ioskit"),
                .product(name: "Highlightr", package: "Highlightr"),
            ],
            path: "ios/CcPresentKit/Sources/CcPresentKit"
        ),
        .target(
            name: "CcPresentUI",
            dependencies: [
                "CcPresentKit",
                .product(name: "IOSKitUI", package: "ioskit"),
                .product(name: "IOSKitWeb", package: "ioskit"),
            ],
            path: "ios/CcPresentUI"
        ),
        .testTarget(
            name: "CcPresentKitTests",
            dependencies: ["CcPresentKit"],
            path: "ios/CcPresentKit/Tests/CcPresentKitTests"
        ),
        .testTarget(
            name: "CcPresentUITests",
            dependencies: ["CcPresentUI"],
            path: "ios/CcPresentUITests"
        ),
    ]
)
