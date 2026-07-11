import Foundation

/// Body for `POST /v1/generations`.
public struct RequestGenerationRequest: Codable, Sendable, Equatable, Hashable {
    public let holidayId: String?
    public let budgetBand: String?
    public let preferencesText: String?
    public let pollId: String?
    public let tier: ModelTier?

    public init(
        holidayId: String? = nil,
        budgetBand: String? = nil,
        preferencesText: String? = nil,
        pollId: String? = nil,
        tier: ModelTier? = nil
    ) {
        self.holidayId = holidayId
        self.budgetBand = budgetBand
        self.preferencesText = preferencesText
        self.pollId = pollId
        self.tier = tier
    }

    private enum CodingKeys: String, CodingKey {
        case tier
        case holidayId = "holiday_id"
        case budgetBand = "budget_band"
        case preferencesText = "preferences_text"
        case pollId = "poll_id"
    }
}

/// `202 Accepted` response for a generation/refine — carries the request id to poll.
public struct GenerationAccepted: Codable, Sendable, Equatable {
    public let requestId: String
    public let status: GenerationStatus?

    public init(requestId: String, status: GenerationStatus? = .queued) {
        self.requestId = requestId
        self.status = status
    }

    private enum CodingKeys: String, CodingKey {
        case status
        case requestId = "request_id"
    }
}

/// Body for `POST /v1/generations/{id}/refine`.
public struct RefineRequest: Codable, Sendable, Equatable {
    public let refinement: String
    public init(refinement: String) { self.refinement = refinement }
}

/// A single ranked idea.
public struct Idea: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let title: String
    public let whyItFits: String?
    public let roughCost: String?
    public let howTo: String?
    public let rank: Int?

    public init(id: String, title: String, whyItFits: String? = nil, roughCost: String? = nil, howTo: String? = nil, rank: Int? = nil) {
        self.id = id
        self.title = title
        self.whyItFits = whyItFits
        self.roughCost = roughCost
        self.howTo = howTo
        self.rank = rank
    }

    private enum CodingKeys: String, CodingKey {
        case id, title, rank
        case whyItFits = "why_it_fits"
        case roughCost = "rough_cost"
        case howTo = "how_to"
    }
}

/// Response for `GET /v1/generations/{id}` — status plus ideas once `ready` (BFF aggregation).
public struct GenerationResult: Codable, Sendable, Equatable {
    public let requestId: String
    public let status: GenerationStatus
    public let progress: Int?
    public let ideas: [Idea]

    public init(requestId: String, status: GenerationStatus, progress: Int? = nil, ideas: [Idea] = []) {
        self.requestId = requestId
        self.status = status
        self.progress = progress
        self.ideas = ideas
    }

    private enum CodingKeys: String, CodingKey {
        case status, progress, ideas
        case requestId = "request_id"
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        requestId = try c.decodeIfPresent(String.self, forKey: .requestId) ?? ""
        status = try c.decodeIfPresent(GenerationStatus.self, forKey: .status) ?? .unknown
        progress = try c.decodeIfPresent(Int.self, forKey: .progress)
        ideas = try c.decodeIfPresent([Idea].self, forKey: .ideas) ?? []
    }

    /// Ideas ordered by rank (ascending; unranked last), for direct rendering.
    public var rankedIdeas: [Idea] {
        ideas.sorted { ($0.rank ?? Int.max) < ($1.rank ?? Int.max) }
    }
}
