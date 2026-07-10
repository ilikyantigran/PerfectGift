import Foundation

/// Authenticated user profile (from `GET /v1/me` and `TokenPair.user`).
public struct User: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let email: String?
    public let displayName: String?
    public let status: String?

    public init(id: String, email: String? = nil, displayName: String? = nil, status: String? = nil) {
        self.id = id
        self.email = email
        self.displayName = displayName
        self.status = status
    }

    private enum CodingKeys: String, CodingKey {
        case id, email, status
        case displayName = "display_name"
    }
}

/// Wrapper for `GET /v1/me` which returns `{ "user": {...} }`.
public struct MeResponse: Codable, Sendable, Equatable {
    public let user: User
    public init(user: User) { self.user = user }
}

/// Body for `POST /v1/auth/signin`.
public struct SignInRequest: Codable, Sendable, Equatable {
    public let provider: AuthProvider
    /// For social sign-in: the provider-issued ID token (JWT).
    public let idToken: String?
    /// For email fallback.
    public let email: String?
    public let password: String?

    public init(provider: AuthProvider, idToken: String? = nil, email: String? = nil, password: String? = nil) {
        self.provider = provider
        self.idToken = idToken
        self.email = email
        self.password = password
    }

    private enum CodingKeys: String, CodingKey {
        case provider, email, password
        case idToken = "id_token"
    }

    public static func apple(idToken: String) -> SignInRequest {
        SignInRequest(provider: .apple, idToken: idToken)
    }
    public static func google(idToken: String) -> SignInRequest {
        SignInRequest(provider: .google, idToken: idToken)
    }
    public static func email(_ email: String, password: String) -> SignInRequest {
        SignInRequest(provider: .email, email: email, password: password)
    }
}

/// Response for signin/refresh: `access_token`, `refresh_token`, `expires_in`, `user`.
public struct TokenPair: Codable, Sendable, Equatable {
    public let accessToken: String
    public let refreshToken: String
    public let expiresIn: Int64?
    public let user: User?

    public init(accessToken: String, refreshToken: String, expiresIn: Int64? = nil, user: User? = nil) {
        self.accessToken = accessToken
        self.refreshToken = refreshToken
        self.expiresIn = expiresIn
        self.user = user
    }

    private enum CodingKeys: String, CodingKey {
        case user
        case accessToken = "access_token"
        case refreshToken = "refresh_token"
        case expiresIn = "expires_in"
    }
}

/// Body for `POST /v1/auth/refresh`.
public struct RefreshRequest: Codable, Sendable, Equatable {
    public let refreshToken: String
    public init(refreshToken: String) { self.refreshToken = refreshToken }
    private enum CodingKeys: String, CodingKey { case refreshToken = "refresh_token" }
}

/// Body for `POST /v1/auth/revoke`.
public struct RevokeRequest: Codable, Sendable, Equatable {
    public let refreshToken: String?
    public let sessionId: String?
    public init(refreshToken: String? = nil, sessionId: String? = nil) {
        self.refreshToken = refreshToken
        self.sessionId = sessionId
    }
    private enum CodingKeys: String, CodingKey {
        case refreshToken = "refresh_token"
        case sessionId = "session_id"
    }
}

/// Generic `{ "ok": true }` success envelope used by revoke/submit/save routes.
public struct OKResponse: Codable, Sendable, Equatable {
    public let ok: Bool?
    public init(ok: Bool? = true) { self.ok = ok }
}
