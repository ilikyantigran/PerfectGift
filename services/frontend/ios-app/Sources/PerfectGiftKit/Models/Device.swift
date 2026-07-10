import Foundation

/// Body for `POST /v1/devices` — registers the APNs token for push.
public struct RegisterDeviceRequest: Codable, Sendable, Equatable {
    public let platform: DevicePlatform
    public let pushToken: String
    public let appVersion: String?

    public init(platform: DevicePlatform = .ios, pushToken: String, appVersion: String? = nil) {
        self.platform = platform
        self.pushToken = pushToken
        self.appVersion = appVersion
    }

    private enum CodingKeys: String, CodingKey {
        case platform
        case pushToken = "push_token"
        case appVersion = "app_version"
    }
}

/// Response for `POST /v1/devices`.
public struct RegisterDeviceResponse: Codable, Sendable, Equatable {
    public let deviceId: String?
    public init(deviceId: String? = nil) { self.deviceId = deviceId }
    private enum CodingKeys: String, CodingKey { case deviceId = "device_id" }
}
