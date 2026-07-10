package com.perfectgift.android.data.repository

import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.dto.AnswerDto
import com.perfectgift.android.data.remote.dto.AuthProvider
import com.perfectgift.android.data.remote.dto.CategoriesResponse
import com.perfectgift.android.data.remote.dto.CreatePollResponse
import com.perfectgift.android.data.remote.dto.GenerationAccepted
import com.perfectgift.android.data.remote.dto.GenerationStatus
import com.perfectgift.android.data.remote.dto.HolidayDto
import com.perfectgift.android.data.remote.dto.PollByToken
import com.perfectgift.android.data.remote.dto.PollResponseDto
import com.perfectgift.android.data.remote.dto.QuestionDto
import com.perfectgift.android.data.remote.dto.RequestGenerationRequest
import com.perfectgift.android.data.remote.dto.TokenPair
import com.perfectgift.android.data.remote.dto.UserDto
import kotlinx.coroutines.flow.Flow

/**
 * The single boundary the ViewModels depend on. Everything returns [ApiResult] so the
 * error envelope is already decoded; nothing above this layer sees Retrofit or HTTP.
 */
interface PerfectGiftRepository {

    // Auth
    suspend fun signIn(
        provider: AuthProvider,
        idToken: String? = null,
        email: String? = null,
        password: String? = null,
    ): ApiResult<TokenPair>

    suspend fun getMe(): ApiResult<UserDto>
    suspend fun signOut(): ApiResult<Unit>

    // Generation (submit-then-observe)
    suspend fun requestGeneration(request: RequestGenerationRequest): ApiResult<GenerationAccepted>

    /**
     * Poll [getGeneration] on an interval, emitting each status update, and completing
     * once status is "ready" or "failed" (or on a transport error).
     */
    fun observeGeneration(requestId: String, pollIntervalMs: Long = 2_000): Flow<ApiResult<GenerationStatus>>

    suspend fun getGeneration(requestId: String): ApiResult<GenerationStatus>
    suspend fun refine(requestId: String, refinement: String): ApiResult<GenerationAccepted>
    suspend fun saveIdea(ideaId: String): ApiResult<Unit>

    // Polls (owner)
    suspend fun createPoll(
        title: String,
        questions: List<QuestionDto>,
        surpriseRequestId: String? = null,
        ttlSeconds: Long? = null,
    ): ApiResult<CreatePollResponse>

    suspend fun getPollResponses(pollId: String): ApiResult<List<PollResponseDto>>

    // Polls (anonymous Subject)
    suspend fun getPollByToken(token: String): ApiResult<PollByToken>
    suspend fun submitPollResponse(token: String, answers: List<AnswerDto>): ApiResult<Unit>

    // Catalog
    suspend fun listHolidays(): ApiResult<List<HolidayDto>>
    suspend fun getCategories(kind: String? = null): ApiResult<CategoriesResponse>

    // Devices / push
    suspend fun registerDevice(pushToken: String, appVersion: String? = null): ApiResult<Unit>
}
