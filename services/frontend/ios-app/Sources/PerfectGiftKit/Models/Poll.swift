import Foundation

/// A single poll question. `kind` carries the full proto enum name.
public struct Question: Codable, Sendable, Equatable, Identifiable {
    public let id: String
    public let text: String
    public let kind: QuestionKind
    public let options: [String]?

    public init(id: String, text: String, kind: QuestionKind, options: [String]? = nil) {
        self.id = id
        self.text = text
        self.kind = kind
        self.options = options
    }

    private enum CodingKeys: String, CodingKey { case id, text, kind, options }
}

/// A single answer to a question. `value` for text/single-choice, `values` for multi-choice.
public struct Answer: Codable, Sendable, Equatable {
    public let questionId: String
    public let value: String?
    public let values: [String]?

    public init(questionId: String, value: String? = nil, values: [String]? = nil) {
        self.questionId = questionId
        self.value = value
        self.values = values
    }

    private enum CodingKeys: String, CodingKey {
        case value, values
        case questionId = "question_id"
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
