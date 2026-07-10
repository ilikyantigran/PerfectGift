import Foundation
import Combine

/// Renders the ranked ideas and handles save/favorite. Ideas are supplied by the
/// generation flow; this VM tracks per-idea saved state and the save call.
@MainActor
public final class IdeasViewModel: ObservableObject {
    @Published public private(set) var ideas: [Idea]
    @Published public private(set) var savedIdeaIds: Set<String> = []
    @Published public var errorMessage: String?

    private let api: APIClient

    public init(api: APIClient, ideas: [Idea] = []) {
        self.api = api
        self.ideas = ideas
    }

    public func setIdeas(_ ideas: [Idea]) {
        self.ideas = ideas.sorted { ($0.rank ?? Int.max) < ($1.rank ?? Int.max) }
    }

    public func isSaved(_ idea: Idea) -> Bool { savedIdeaIds.contains(idea.id) }

    public func toggleSave(_ idea: Idea) async {
        // Optimistic update; roll back on failure.
        let wasSaved = savedIdeaIds.contains(idea.id)
        if wasSaved { savedIdeaIds.remove(idea.id) } else { savedIdeaIds.insert(idea.id) }
        guard !wasSaved else { return } // Only the save call exists in the API.
        do {
            try await api.saveIdea(id: idea.id)
        } catch {
            savedIdeaIds.remove(idea.id)
            errorMessage = (error as? APIError)?.userMessage ?? error.localizedDescription
        }
    }
}
