import Foundation

/// Runtime configuration for the client. The gateway base URL is configurable per
/// environment (the local Docker stack defaults to http://localhost:8080). The
/// Apple/Google client IDs are read by the app target for the native sign-in SDKs.
public struct AppConfig: Sendable {
    /// Base URL of the API Gateway, e.g. `http://localhost:8080` or `https://api.perfectgift.app`.
    public var baseURL: URL
    /// Universal-link host used to detect a shared poll link (e.g. `perfectgift.app`).
    public var universalLinkHost: String
    /// Apple Sign In service identifier (consumed by the app target).
    public var appleClientID: String?
    /// Google Sign-In client identifier (consumed by the app target).
    public var googleClientID: String?

    public init(
        baseURL: URL,
        universalLinkHost: String = "perfectgift.app",
        appleClientID: String? = nil,
        googleClientID: String? = nil
    ) {
        self.baseURL = baseURL
        self.universalLinkHost = universalLinkHost
        self.appleClientID = appleClientID
        self.googleClientID = googleClientID
    }

    /// Default configuration pointed at the local Docker stack.
    public static let localDefault = AppConfig(
        baseURL: URL(string: "http://localhost:8080")!
    )

    /// Builds a config from the process environment / Info.plist-style overrides.
    /// Recognised keys: `PG_BASE_URL`, `PG_UNIVERSAL_LINK_HOST`,
    /// `PG_APPLE_CLIENT_ID`, `PG_GOOGLE_CLIENT_ID`.
    public static func fromEnvironment(_ env: [String: String] = ProcessInfo.processInfo.environment) -> AppConfig {
        var config = AppConfig.localDefault
        if let raw = env["PG_BASE_URL"], let url = URL(string: raw) {
            config.baseURL = url
        }
        if let host = env["PG_UNIVERSAL_LINK_HOST"], !host.isEmpty {
            config.universalLinkHost = host
        }
        config.appleClientID = env["PG_APPLE_CLIENT_ID"]
        config.googleClientID = env["PG_GOOGLE_CLIENT_ID"]
        return config
    }
}
