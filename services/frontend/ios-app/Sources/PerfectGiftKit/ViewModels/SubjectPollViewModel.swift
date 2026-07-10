import Foundation
import Combine

/// Drives the Subject-side poll: fetch by opaque link token (no auth), collect answers,
/// and submit. Used for both the handed-over-phone case and universal-link deep opens.
@MainActor
public final class SubjectPollViewModel: ObservableObject {
    public enum Phase: Equatable {
        case loading
        case ready
        case submitting
        case submitted
        case failed(String)
    }

    @Published public private(set) var phase: Phase = .loading
    @Published public private(set) var poll: PollByToken?
    /// Working answers keyed by question id. `value` for text/single, `values` for multi.
    @Published public var textAnswers: [String: String] = [:]
    @Published public var singleChoice: [String: String] = [:]
    @Published public var multiChoice: [String: Set<String>] = [:]

    private let api: APIClient
    private let token: String

    public init(api: APIClient, token: String) {
        self.api = api
        self.token = token
    }

    public func load() async {
        phase = .loading
        do {
            poll = try await api.pollByToken(token)
            phase = .ready
        } catch {
            phase = .failed((error as? APIError)?.userMessage ?? error.localizedDescription)
        }
    }

    public func toggleMultiChoice(questionId: String, option: String) {
        var set = multiChoice[questionId] ?? []
        if set.contains(option) { set.remove(option) } else { set.insert(option) }
        multiChoice[questionId] = set
    }

    /// Assemble the answers array from the working state, in question order.
    public func buildAnswers() -> [Answer] {
        guard let poll else { return [] }
        return poll.questions.compactMap { q in
            switch q.kind {
            case .text:
                let v = (textAnswers[q.id] ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
                return v.isEmpty ? nil : Answer(questionId: q.id, value: v)
            case .singleChoice:
                guard let v = singleChoice[q.id] else { return nil }
                return Answer(questionId: q.id, value: v)
            case .multiChoice:
                let vals = Array(multiChoice[q.id] ?? [])
                return vals.isEmpty ? nil : Answer(questionId: q.id, values: vals)
            case .unknown:
                return nil
            }
        }
    }

    public func submit() async {
        phase = .submitting
        do {
            try await api.submitResponse(token: token, answers: buildAnswers())
            phase = .submitted
        } catch {
            phase = .failed((error as? APIError)?.userMessage ?? error.localizedDescription)
        }
    }
}
