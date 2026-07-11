import Foundation
import Combine

/// Drives poll creation + sharing (owner side), and reading Subject responses.
@MainActor
public final class PollCreateViewModel: ObservableObject {
    @Published public var title: String = "Help me pick your surprise"
    @Published public var questions: [Question]
    @Published public private(set) var isWorking = false
    @Published public private(set) var created: CreatePollResponse?
    @Published public private(set) var responses: [PollResponse] = []
    @Published public var errorMessage: String?

    /// Optional link to a generation the poll should sharpen.
    public var surpriseRequestId: String?

    private let api: APIClient

    public init(api: APIClient, questions: [Question]? = nil, surpriseRequestId: String? = nil) {
        self.api = api
        self.surpriseRequestId = surpriseRequestId
        self.questions = questions ?? PollCreateViewModel.defaultQuestions
    }

    /// A sensible starter set the owner can edit.
    public static let defaultQuestions: [Question] = [
        Question(id: "q_budget", prompt: "Roughly how much should I spend?", type: .singleChoice,
                 options: [.init(id: "b1", label: "Under $50"), .init(id: "b2", label: "$50–150"),
                           .init(id: "b3", label: "$150–400"), .init(id: "b4", label: "Sky's the limit")]),
        Question(id: "q_vibe", prompt: "What's the vibe?", type: .multiChoice,
                 options: [.init(id: "v1", label: "Cozy"), .init(id: "v2", label: "Adventurous"),
                           .init(id: "v3", label: "Luxurious"), .init(id: "v4", label: "Handmade"),
                           .init(id: "v5", label: "Experiences")]),
        Question(id: "q_notes", prompt: "Anything I should absolutely avoid?", type: .text)
    ]

    public var shareURL: URL? {
        guard let created else { return nil }
        if let raw = created.linkUrl, let url = URL(string: raw) { return url }
        return nil
    }

    public func createPoll() async {
        isWorking = true
        errorMessage = nil
        defer { isWorking = false }
        let request = CreatePollRequest(
            title: title,
            questions: questions,
            surpriseRequestId: surpriseRequestId,
            ttlSeconds: 7 * 24 * 3600
        )
        do {
            created = try await api.createPoll(request)
        } catch {
            errorMessage = (error as? APIError)?.userMessage ?? error.localizedDescription
        }
    }

    public func loadResponses() async {
        guard let pollId = created?.pollId else { return }
        do {
            responses = try await api.pollResponses(pollId: pollId)
        } catch {
            errorMessage = (error as? APIError)?.userMessage ?? error.localizedDescription
        }
    }
}
