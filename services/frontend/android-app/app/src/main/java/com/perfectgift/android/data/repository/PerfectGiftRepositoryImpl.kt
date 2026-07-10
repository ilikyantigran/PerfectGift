package com.perfectgift.android.data.repository

import com.google.gson.Gson
import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.GatewayApi
import com.perfectgift.android.data.remote.toApiResult
import com.perfectgift.android.data.remote.dto.AnswerDto
import com.perfectgift.android.data.remote.dto.AuthProvider
import com.perfectgift.android.data.remote.dto.CategoriesResponse
import com.perfectgift.android.data.remote.dto.CreatePollRequest
import com.perfectgift.android.data.remote.dto.CreatePollResponse
import com.perfectgift.android.data.remote.dto.GenerationAccepted
import com.perfectgift.android.data.remote.dto.GenerationStatus
import com.perfectgift.android.data.remote.dto.HolidayDto
import com.perfectgift.android.data.remote.dto.PollByToken
import com.perfectgift.android.data.remote.dto.PollResponseDto
import com.perfectgift.android.data.remote.dto.QuestionDto
import com.perfectgift.android.data.remote.dto.RefineRequest
import com.perfectgift.android.data.remote.dto.RegisterDeviceRequest
import com.perfectgift.android.data.remote.dto.RequestGenerationRequest
import com.perfectgift.android.data.remote.dto.RevokeRequest
import com.perfectgift.android.data.remote.dto.SignInRequest
import com.perfectgift.android.data.remote.dto.SubmitResponseRequest
import com.perfectgift.android.data.remote.dto.TokenPair
import com.perfectgift.android.data.remote.dto.UserDto
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import java.util.UUID

class PerfectGiftRepositoryImpl(
    private val api: GatewayApi,
    private val session: SessionManager,
    private val gson: Gson,
) : PerfectGiftRepository {

    // --- Auth ---

    override suspend fun signIn(
        provider: AuthProvider,
        idToken: String?,
        email: String?,
        password: String?,
    ): ApiResult<TokenPair> {
        val result = api.signIn(SignInRequest(provider, idToken, email, password)).toApiResult(gson)
        if (result is ApiResult.Success) {
            session.onSignedIn(result.data.accessToken, result.data.refreshToken)
        }
        return result
    }

    override suspend fun getMe(): ApiResult<UserDto> =
        api.getMe().toApiResult(gson).mapData { it.user ?: UserDto() }

    override suspend fun signOut(): ApiResult<Unit> {
        val refresh = session.currentRefreshToken()
        val result = api.revoke(RevokeRequest(refreshToken = refresh)).toApiResult(gson).mapData { }
        session.signOut()
        return result
    }

    // --- Generation ---

    override suspend fun requestGeneration(request: RequestGenerationRequest): ApiResult<GenerationAccepted> =
        // Idempotency-Key makes a retried submit safe (see SERVICE.md §3).
        api.requestGeneration(UUID.randomUUID().toString(), request).toApiResult(gson)

    override fun observeGeneration(
        requestId: String,
        pollIntervalMs: Long,
    ): Flow<ApiResult<GenerationStatus>> = flow {
        while (true) {
            val result = getGeneration(requestId)
            emit(result)
            val terminal = result is ApiResult.Failure ||
                (result is ApiResult.Success && result.data.status.isTerminal())
            if (terminal) break
            kotlinx.coroutines.delay(pollIntervalMs)
        }
    }

    override suspend fun getGeneration(requestId: String): ApiResult<GenerationStatus> =
        api.getGeneration(requestId).toApiResult(gson)

    override suspend fun refine(requestId: String, refinement: String): ApiResult<GenerationAccepted> =
        api.refine(requestId, RefineRequest(refinement)).toApiResult(gson)

    override suspend fun saveIdea(ideaId: String): ApiResult<Unit> =
        api.saveIdea(ideaId).toApiResult(gson).mapData { }

    // --- Polls (owner) ---

    override suspend fun createPoll(
        title: String,
        questions: List<QuestionDto>,
        surpriseRequestId: String?,
        ttlSeconds: Long?,
    ): ApiResult<CreatePollResponse> =
        api.createPoll(CreatePollRequest(title, questions, surpriseRequestId, ttlSeconds)).toApiResult(gson)

    override suspend fun getPollResponses(pollId: String): ApiResult<List<PollResponseDto>> =
        api.getPollResponses(pollId).toApiResult(gson).mapData { it.responses }

    // --- Polls (anonymous Subject) ---

    override suspend fun getPollByToken(token: String): ApiResult<PollByToken> =
        api.getPollByToken(token).toApiResult(gson)

    override suspend fun submitPollResponse(token: String, answers: List<AnswerDto>): ApiResult<Unit> =
        api.submitPollResponse(token, SubmitResponseRequest(answers)).toApiResult(gson).mapData { }

    // --- Catalog ---

    override suspend fun listHolidays(): ApiResult<List<HolidayDto>> =
        api.listHolidays().toApiResult(gson).mapData { it.holidays }

    override suspend fun getCategories(kind: String?): ApiResult<CategoriesResponse> =
        api.getCategories(kind).toApiResult(gson)

    // --- Devices ---

    override suspend fun registerDevice(pushToken: String, appVersion: String?): ApiResult<Unit> =
        api.registerDevice(RegisterDeviceRequest(pushToken = pushToken, appVersion = appVersion))
            .toApiResult(gson).mapData { }
}

private fun String?.isTerminal(): Boolean = this == "ready" || this == "failed"

private inline fun <T, R> ApiResult<T>.mapData(transform: (T) -> R): ApiResult<R> = when (this) {
    is ApiResult.Success -> ApiResult.Success(transform(data))
    is ApiResult.Failure -> this
}
