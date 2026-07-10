package com.perfectgift.android.data.remote.dto

/** POST /v1/generations body. */
data class RequestGenerationRequest(
    val holidayId: String? = null,
    val budgetBand: String? = null,
    val preferencesText: String? = null,
    val pollId: String? = null,
    /** LLM tier: sonnet | opus | haiku. */
    val tier: String? = null,
)

/** 202 body from POST /v1/generations and /refine. */
data class GenerationAccepted(
    val requestId: String,
    val status: String? = null,
)

/** POST /v1/generations/{id}/refine body. */
data class RefineRequest(
    val refinement: String,
)

data class IdeaDto(
    val id: String? = null,
    val title: String? = null,
    val whyItFits: String? = null,
    val roughCost: String? = null,
    val howTo: String? = null,
    val rank: Int = 0,
)

/**
 * GET /v1/generations/{id}. BFF aggregation: status plus ideas once ready.
 * status ∈ { queued, running, ready, failed }.
 */
data class GenerationStatus(
    val requestId: String? = null,
    val status: String? = null,
    val progress: Int = 0,
    val ideas: List<IdeaDto>? = null,
)
