import Foundation

/// One selectable option for a choice question: a stable `id` (echoed back as a
/// choice id) plus a human-readable `label`.
public struct QuestionOption: Codable, Sendable, Equatable, Hashable, Identifiable {
    public let id: String
    public let label: String

    public init(id: String, label: String) {
        self.id = id
        self.label = label
    }
}

/// A single poll question. `type` carries the full proto enum name; choice questions
/// carry `options`. Matches the gateway contract: `prompt` / `type` / `options` / `required`.
public struct Question: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let prompt: String
    public let type: QuestionKind
    public let options: [QuestionOption]?
    public let required: Bool

    public init(id: String, prompt: String, type: QuestionKind, options: [QuestionOption]? = nil, required: Bool = false) {
        self.id = id
        self.prompt = prompt
        self.type = type
        self.options = options
        self.required = required
    }

    private enum CodingKeys: String, CodingKey { case id, prompt, type, options, required }

    // Lenient: the gateway omits `type`/`options`/`required` when empty.
    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        prompt = try c.decodeIfPresent(String.self, forKey: .prompt) ?? ""
        type = try c.decodeIfPresent(QuestionKind.self, forKey: .type) ?? .unknown
        options = try c.decodeIfPresent([QuestionOption].self, forKey: .options)
        required = try c.decodeIfPresent(Bool.self, forKey: .required) ?? false
    }
}

/// A single answer. `text` for TEXT questions; `choiceIds` for choice questions.
/// Matches the gateway contract: `question_id` / `text` / `choice_ids`.
public struct Answer: Codable, Sendable, Equatable {
    public let questionId: String
    public let text: String?
    public let choiceIds: [String]?

    public init(questionId: String, text: String? = nil, choiceIds: [String]? = nil) {
        self.questionId = questionId
        self.text = text
        self.choiceIds = choiceIds
    }

    private enum CodingKeys: String, CodingKey {
        case text
        case questionId = "question_id"
        case choiceIds = "choice_ids"
    }
}

/// Body for `POST /v1/polls`.
public struct CreatePollRequest: Codable, Sendable, Equatable {
    public let title: String
    public let questions: [Question]
    public let surpriseRequestId: String?
    public let ttlSeconds: Int64?

    public init(title: String, questions: [Question], surpriseRequestId: String? = nil, ttlSeconds: Int64? = nil) {
        self.title = title
        self.questions = questions
        self.surpriseRequestId = surpriseRequestId
        self.ttlSeconds = ttlSeconds
    }

    private enum CodingKeys: String, CodingKey {
        case title, questions
        case surpriseRequestId = "surprise_request_id"
        case ttlSeconds = "ttl_seconds"
    }
}

/// Response for `POST /v1/polls` — the raw link token is returned once.
public struct CreatePollResponse: Codable, Sendable, Equatable {
    public let pollId: String
    public let linkToken: String
    public let linkUrl: String?
    public let expiresAt: String?

    public init(pollId: String, linkToken: String, linkUrl: String? = nil, expiresAt: String? = nil) {
        self.pollId = pollId
        self.linkToken = linkToken
        self.linkUrl = linkUrl
        self.expiresAt = expiresAt
    }

    private enum CodingKeys: String, CodingKey {
        case pollId = "poll_id"
        case linkToken = "link_token"
        case linkUrl = "link_url"
        case expiresAt = "expires_at"
    }
}

/// Poll fetched anonymously by the Subject via link token (`GET /v1/polls/token/{t}`).
public struct PollByToken: Codable, Sendable, Equatable {
    public let pollId: String
    public let title: String
    public let questions: [Question]

    public init(pollId: String, title: String, questions: [Question]) {
        self.pollId = pollId
        self.title = title
        self.questions = questions
    }

    private enum CodingKeys: String, CodingKey {
        case title, questions
        case pollId = "poll_id"
    }
}

/// Body for `POST /v1/polls/token/{t}/responses`.
public struct SubmitResponseRequest: Codable, Sendable, Equatable {
    public let answers: [Answer]
    public init(answers: [Answer]) { self.answers = answers }
}

/// A Subject's submitted response as read by the owner (`GET /v1/polls/{id}/responses`).
public struct PollResponse: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let answers: [Answer]
    public let submittedAt: String?

    public init(id: String, answers: [Answer], submittedAt: String? = nil) {
        self.id = id
        self.answers = answers
        self.submittedAt = submittedAt
    }

    private enum CodingKeys: String, CodingKey {
        case id, answers
        case submittedAt = "submitted_at"
    }
}

/// Wrapper for `GET /v1/polls/{id}/responses`.
public struct PollResponsesResponse: Codable, Sendable, Equatable {
    public let responses: [PollResponse]
    public init(responses: [PollResponse]) { self.responses = responses }
}
