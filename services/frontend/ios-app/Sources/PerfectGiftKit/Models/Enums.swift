import Foundation

// The gateway forwards internal gRPC/proto enums using their FULL proto value names
// (e.g. "PROVIDER_APPLE", "GENERATION_STATUS_READY", "MODEL_TIER_SONNET"). The
// published openapi.yaml shows shortened aliases (apple/ready/sonnet); to be safe the
// enums below ENCODE to the full proto name and DECODE leniently, accepting the full
// name, the short alias, and any unknown value (mapped to `.unknown`) so a new server
// enum never crashes the client.

/// Sign-in provider. Encodes as the full proto name the Identity service expects.
public enum AuthProvider: String, Codable, CaseIterable, Sendable {
    case apple  = "PROVIDER_APPLE"
    case google = "PROVIDER_GOOGLE"
    case email  = "PROVIDER_EMAIL"

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        switch raw.uppercased() {
        case "PROVIDER_APPLE", "APPLE":   self = .apple
        case "PROVIDER_GOOGLE", "GOOGLE": self = .google
        case "PROVIDER_EMAIL", "EMAIL":   self = .email
        default: self = .email
        }
    }
}

/// Lifecycle of an async generation. Drives the submit-then-observe UI.
public enum GenerationStatus: String, Codable, Sendable {
    case queued  = "GENERATION_STATUS_QUEUED"
    case running = "GENERATION_STATUS_RUNNING"
    case ready   = "GENERATION_STATUS_READY"
    case failed  = "GENERATION_STATUS_FAILED"
    case unknown = "GENERATION_STATUS_UNSPECIFIED"

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = GenerationStatus.parse(raw)
    }

    public static func parse(_ raw: String) -> GenerationStatus {
        switch raw.uppercased() {
        case "GENERATION_STATUS_QUEUED", "QUEUED":   return .queued
        case "GENERATION_STATUS_RUNNING", "RUNNING": return .running
        case "GENERATION_STATUS_READY", "READY":     return .ready
        case "GENERATION_STATUS_FAILED", "FAILED":   return .failed
        default: return .unknown
        }
    }

    /// True once the generation has stopped advancing (ready or failed).
    public var isTerminal: Bool { self == .ready || self == .failed }
}

/// Model tier selectable for a generation. Haiku is internal-only (moderation) and not offered.
public enum ModelTier: String, Codable, CaseIterable, Sendable {
    case sonnet = "MODEL_TIER_SONNET"
    case opus   = "MODEL_TIER_OPUS"

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        switch raw.uppercased() {
        case "MODEL_TIER_OPUS", "OPUS": self = .opus
        default: self = .sonnet
        }
    }

    public var displayName: String {
        switch self {
        case .sonnet: return "Standard"
        case .opus:   return "Deep (premium)"
        }
    }
}

/// Push platform for device registration.
public enum DevicePlatform: String, Codable, Sendable {
    case ios     = "PLATFORM_IOS"
    case android = "PLATFORM_ANDROID"

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        switch raw.uppercased() {
        case "PLATFORM_ANDROID", "ANDROID": self = .android
        default: self = .ios
        }
    }
}

/// Poll question kind. openapi calls the field `kind`; values are the full proto names.
public enum QuestionKind: String, Codable, Sendable {
    case text         = "QUESTION_TYPE_TEXT"
    case singleChoice = "QUESTION_TYPE_SINGLE_CHOICE"
    case multiChoice  = "QUESTION_TYPE_MULTI_CHOICE"
    case unknown      = "QUESTION_TYPE_UNSPECIFIED"

    public init(from decoder: Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        switch raw.uppercased() {
        case "QUESTION_TYPE_TEXT", "TEXT":                   self = .text
        case "QUESTION_TYPE_SINGLE_CHOICE", "SINGLE_CHOICE": self = .singleChoice
        case "QUESTION_TYPE_MULTI_CHOICE", "MULTI_CHOICE":   self = .multiChoice
        default: self = .unknown
        }
    }

    /// Whether this kind expects one or more option ids (vs. free text).
    public var isChoice: Bool { self == .singleChoice || self == .multiChoice }
}
