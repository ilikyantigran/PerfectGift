package com.perfectgift.android.data.remote.dto

data class QuestionDto(
    val id: String? = null,
    val text: String? = null,
    /** e.g. "single" | "multi" | "text". */
    val kind: String? = null,
    val options: List<String>? = null,
)

data class AnswerDto(
    val questionId: String? = null,
    val value: String? = null,
    val values: List<String>? = null,
)

/** POST /v1/polls */
data class CreatePollRequest(
    val title: String? = null,
    val questions: List<QuestionDto> = emptyList(),
    val surpriseRequestId: String? = null,
    val ttlSeconds: Long? = null,
)

/** 201 body from POST /v1/polls — link token returned once. */
data class CreatePollResponse(
    val pollId: String? = null,
    val linkToken: String? = null,
    val linkUrl: String? = null,
    val expiresAt: String? = null,
)

/** GET /v1/polls/token/{t} — anonymous Subject view (no owner data). */
data class PollByToken(
    val pollId: String? = null,
    val title: String? = null,
    val questions: List<QuestionDto> = emptyList(),
)

/** POST /v1/polls/token/{t}/responses */
data class SubmitResponseRequest(
    val answers: List<AnswerDto> = emptyList(),
)

data class PollResponseDto(
    val id: String? = null,
    val answers: List<AnswerDto> = emptyList(),
    val submittedAt: String? = null,
)

/** GET /v1/polls/{id}/responses */
data class PollResponsesResponse(
    val responses: List<PollResponseDto> = emptyList(),
)
