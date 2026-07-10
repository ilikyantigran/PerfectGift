import Foundation
#if canImport(Security)
import Security
#endif

/// The tokens we persist. Only these ever leave memory — no other PII is stored on device.
public struct StoredTokens: Codable, Sendable, Equatable {
    public var accessToken: String
    public var refreshToken: String
    /// Absolute expiry of the access token, if the server told us `expires_in`.
    public var accessExpiresAt: Date?

    public init(accessToken: String, refreshToken: String, accessExpiresAt: Date? = nil) {
        self.accessToken = accessToken
        self.refreshToken = refreshToken
        self.accessExpiresAt = accessExpiresAt
    }

    public init(pair: TokenPair, now: Date = Date()) {
        self.accessToken = pair.accessToken
        self.refreshToken = pair.refreshToken
        self.accessExpiresAt = pair.expiresIn.map { now.addingTimeInterval(TimeInterval($0)) }
    }
}

/// Persistence for the auth token. Per the brief, the ONLY thing stored on device.
public protocol TokenStore: Sendable {
    func load() -> StoredTokens?
    func save(_ tokens: StoredTokens)
    func clear()
}

/// In-memory store — used in unit tests and previews (never touches the Keychain).
public final class InMemoryTokenStore: TokenStore, @unchecked Sendable {
    private let lock = NSLock()
    private var tokens: StoredTokens?

    public init(_ initial: StoredTokens? = nil) { self.tokens = initial }

    public func load() -> StoredTokens? { lock.withLock { tokens } }
    public func save(_ tokens: StoredTokens) { lock.withLock { self.tokens = tokens } }
    public func clear() { lock.withLock { tokens = nil } }
}

#if canImport(Security)
/// Keychain-backed store. Stores a single JSON blob under one generic-password item.
public final class KeychainTokenStore: TokenStore, @unchecked Sendable {
    private let service: String
    private let account: String

    public init(service: String = "app.perfectgift.tokens", account: String = "primary") {
        self.service = service
        self.account = account
    }

    private var baseQuery: [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
    }

    public func load() -> StoredTokens? {
        var query = baseQuery
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        guard status == errSecSuccess, let data = item as? Data else { return nil }
        return try? JSONDecoder().decode(StoredTokens.self, from: data)
    }

    public func save(_ tokens: StoredTokens) {
        guard let data = try? JSONEncoder().encode(tokens) else { return }
        // Delete any existing item, then add fresh (simplest correct upsert).
        SecItemDelete(baseQuery as CFDictionary)
        var attrs = baseQuery
        attrs[kSecValueData as String] = data
        attrs[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        SecItemAdd(attrs as CFDictionary, nil)
    }

    public func clear() {
        SecItemDelete(baseQuery as CFDictionary)
    }
}
#endif

extension NSLock {
    @discardableResult
    func withLock<T>(_ body: () -> T) -> T {
        lock(); defer { unlock() }
        return body()
    }
}
