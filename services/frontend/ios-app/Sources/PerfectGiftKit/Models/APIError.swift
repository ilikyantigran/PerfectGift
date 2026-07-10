import Foundation

/// The uniform error envelope every gateway route returns on failure:
/// `{ "error": { "code": "string", "message": "string", "details": {...} } }`.
public struct ErrorEnvelope: Codable, Sendable, Equatable {
    public struct Body: Codable, Sendable, Equatable {
        public let code: String
        public let message: String
        public let details: [String: String]?

        public init(code: String, message: String, details: [String: String]? = nil) {
            self.code = code
            self.message = message
            self.details = details
        }

        private enum CodingKeys: String, CodingKey { case code, message, details }

        public init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            code = try c.decode(String.self, forKey: .code)
            message = try c.decode(String.self, forKey: .message)
            // `details` is a free-form object; decode leniently to [String: String].
            details = try? c.decodeIfPresent([String: JSONValue].self, forKey: .details)?
                .mapValues(\.stringValue)
        }

        public func encode(to encoder: Encoder) throws {
            var c = encoder.container(keyedBy: CodingKeys.self)
            try c.encode(code, forKey: .code)
            try c.encode(message, forKey: .message)
            try c.encodeIfPresent(details, forKey: .details)
        }
    }

    public let error: Body
    public init(error: Body) { self.error = error }
}

/// All errors the API layer can surface to a view model.
public enum APIError: Error, Sendable, Equatable {
    /// The server returned the standard error envelope with a status code.
    case server(status: Int, code: String, message: String, details: [String: String]?)
    /// A non-2xx response whose body was not a valid error envelope.
    case http(status: Int, body: String)
    /// The response body could not be decoded into the expected type.
    case decoding(String)
    /// A transport-level failure (no connectivity, timeout, TLS, …).
    case transport(String)
    /// The user is not authenticated and no refresh token is available.
    case unauthenticated
    /// The client is misconfigured (bad URL, etc.).
    case invalidRequest(String)

    /// A human-friendly message suitable for a "try again" surface.
    public var userMessage: String {
        switch self {
        case let .server(_, _, message, _): return message
        case let .http(status, _):          return "Something went wrong (HTTP \(status))."
        case .decoding:                     return "We received an unexpected response. Please try again."
        case let .transport(m):             return "Network problem: \(m)"
        case .unauthenticated:              return "Your session expired. Please sign in again."
        case let .invalidRequest(m):        return m
        }
    }

    /// The server-defined machine code when present (e.g. `not_found`, `rate_limited`).
    public var code: String? {
        if case let .server(_, code, _, _) = self { return code }
        return nil
    }
}

/// Minimal JSON value used only to leniently read the free-form `details` object.
enum JSONValue: Codable {
    case string(String), number(Double), bool(Bool), null, other

    init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if c.decodeNil() { self = .null }
        else if let s = try? c.decode(String.self) { self = .string(s) }
        else if let b = try? c.decode(Bool.self) { self = .bool(b) }
        else if let n = try? c.decode(Double.self) { self = .number(n) }
        else { self = .other }
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case let .string(s): try c.encode(s)
        case let .number(n): try c.encode(n)
        case let .bool(b):   try c.encode(b)
        case .null, .other:  try c.encodeNil()
        }
    }

    var stringValue: String {
        switch self {
        case let .string(s): return s
        case let .number(n): return n == n.rounded() ? String(Int(n)) : String(n)
        case let .bool(b):   return String(b)
        case .null, .other:  return ""
        }
    }
}
