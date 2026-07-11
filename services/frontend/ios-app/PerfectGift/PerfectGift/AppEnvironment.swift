import Foundation
import SwiftUI
import Combine
import PerfectGiftKit

/// Composition root. Builds the single object graph the app uses: config → token store →
/// token provider → API client → AuthManager. Everything downstream is created from here,
/// so swapping the base URL (or a fake client in previews) is a one-line change.
@MainActor
final class AppEnvironment: ObservableObject {
    let config: AppConfig
    let api: APIClient
    let tokens: TokenProvider
    let auth: AuthManager
    let push: PushRegistrar

    init(config: AppConfig = AppConfig.fromEnvironment()) {
        self.config = config
        let store: TokenStore
        #if canImport(Security)
        store = KeychainTokenStore()
        #else
        store = InMemoryTokenStore()
        #endif
        let provider = TokenProvider(store: store)
        let client = LiveAPIClient(baseURL: config.baseURL, transport: URLSessionTransport(), tokens: provider)
        self.tokens = provider
        self.api = client
        self.auth = AuthManager(api: client, tokens: provider)
        self.push = PushRegistrar(api: client)
    }

    /// Factory helpers so views build their own view models with the shared client.
    func makeSignInViewModel() -> SignInViewModel { SignInViewModel(auth: auth) }
    func makeOccasionViewModel() -> OccasionInputViewModel { OccasionInputViewModel(api: api) }
    func makeGenerationViewModel() -> GenerationViewModel { GenerationViewModel(api: api) }
    func makeIdeasViewModel(ideas: [Idea]) -> IdeasViewModel { IdeasViewModel(api: api, ideas: ideas) }
    func makePollCreateViewModel(surpriseRequestId: String?) -> PollCreateViewModel {
        PollCreateViewModel(api: api, surpriseRequestId: surpriseRequestId)
    }
    func makeSubjectPollViewModel(token: String) -> SubjectPollViewModel {
        SubjectPollViewModel(api: api, token: token)
    }
}

/// Registers the APNs device token with the gateway (`POST /v1/devices`). The
/// AppDelegate hands the raw token here once the OS grants it.
@MainActor
final class PushRegistrar: ObservableObject {
    private let api: APIClient
    @Published private(set) var registeredDeviceId: String?

    init(api: APIClient) { self.api = api }

    func register(deviceToken: Data, appVersion: String?) async {
        let hex = deviceToken.map { String(format: "%02x", $0) }.joined()
        do {
            let resp = try await api.registerDevice(
                RegisterDeviceRequest(platform: .ios, pushToken: hex, appVersion: appVersion)
            )
            registeredDeviceId = resp.deviceId
        } catch {
            // Push is best-effort per the spec — never block the app on it.
        }
    }
}
