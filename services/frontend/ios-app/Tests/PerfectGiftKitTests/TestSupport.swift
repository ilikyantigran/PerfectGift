import Testing
import Foundation
@testable import PerfectGiftKit

// NOTE: This target uses **Swift Testing** (`import Testing`), not XCTest. The local
// Command Line Tools toolchain ships Swift Testing but not XCTest, and Swift Testing is
// the modern default anyway. Tests run via `swift test`.

/// Polls `condition` on the main actor until true or the timeout elapses. Used to await
/// asynchronous view-model state transitions driven by a background poll loop.
@MainActor
func waitUntil(
    timeout: TimeInterval = 2.0,
    _ condition: @MainActor () -> Bool,
    sourceLocation: SourceLocation = #_sourceLocation
) async {
    let deadline = Date().addingTimeInterval(timeout)
    while Date() < deadline {
        if condition() { return }
        await Task.yield()
        try? await Task.sleep(nanoseconds: 1_000_000) // 1ms
    }
    if !condition() {
        Issue.record("waitUntil timed out after \(timeout)s", sourceLocation: sourceLocation)
    }
}

/// A scripted transport for exercising `LiveAPIClient` end-to-end without a network.
/// Each queued stub matches the next request; use it to prove refresh-on-401.
final class FakeTransport: HTTPTransport, @unchecked Sendable {
    struct Stub {
        var status: Int
        var body: Data
        /// Optional predicate to assert on the outgoing request.
        var expect: ((URLRequest) -> Void)?
    }

    private let lock = NSLock()
    private var stubs: [Stub]
    private(set) var sentRequests: [URLRequest] = []

    init(_ stubs: [Stub]) { self.stubs = stubs }

    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        let stub: Stub = lock.withLock {
            sentRequests.append(request)
            return stubs.isEmpty ? Stub(status: 500, body: Data()) : stubs.removeFirst()
        }
        stub.expect?(request)
        let response = HTTPURLResponse(url: request.url!, statusCode: stub.status, httpVersion: nil, headerFields: nil)!
        return (stub.body, response)
    }

    func requestCount() -> Int { lock.withLock { sentRequests.count } }
    func authHeader(at index: Int) -> String? {
        lock.withLock { sentRequests[index].value(forHTTPHeaderField: "Authorization") }
    }
    func path(at index: Int) -> String? {
        lock.withLock { sentRequests[index].url?.path }
    }
}

func json(_ string: String) -> Data { string.data(using: .utf8)! }
