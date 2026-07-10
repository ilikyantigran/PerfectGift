// swift-tools-version:6.0
import PackageDescription

// PerfectGiftKit is the platform-neutral core of the iOS app: models, the API client,
// the auth/token store, and the MVVM view models. It contains NO SwiftUI/UIKit so it
// builds and unit-tests on the command line (`swift test`) against the macOS SDK,
// exactly the way it runs inside the iOS app target. The SwiftUI views, the @main App
// entry, APNs and Sign in with Apple live in ../App (the Xcode iOS target) and depend
// on this package.
let package = Package(
    name: "PerfectGiftKit",
    platforms: [
        .iOS(.v16),
        .macOS(.v13)
    ],
    products: [
        .library(name: "PerfectGiftKit", targets: ["PerfectGiftKit"])
    ],
    targets: [
        .target(
            name: "PerfectGiftKit",
            path: "Sources/PerfectGiftKit",
            swiftSettings: [.swiftLanguageMode(.v5)]
        ),
        .testTarget(
            name: "PerfectGiftKitTests",
            dependencies: ["PerfectGiftKit"],
            path: "Tests/PerfectGiftKitTests",
            swiftSettings: [.swiftLanguageMode(.v5)]
        )
    ]
)
