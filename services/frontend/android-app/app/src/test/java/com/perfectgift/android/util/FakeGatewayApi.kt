package com.perfectgift.android.util

import com.perfectgift.android.data.remote.GatewayApi
import com.perfectgift.android.data.remote.dto.CategoriesResponse
import com.perfectgift.android.data.remote.dto.CreatePollRequest
import com.perfectgift.android.data.remote.dto.CreatePollResponse
import com.perfectgift.android.data.remote.dto.GenerationAccepted
import com.perfectgift.android.data.remote.dto.GenerationStatus
import com.perfectgift.android.data.remote.dto.HolidaysResponse
import com.perfectgift.android.data.remote.dto.MeResponse
import com.perfectgift.android.data.remote.dto.OkResponse
import com.perfectgift.android.data.remote.dto.PollByToken
import com.perfectgift.android.data.remote.dto.PollResponsesResponse
import com.perfectgift.android.data.remote.dto.RefineRequest
import com.perfectgift.android.data.remote.dto.RefreshRequest
import com.perfectgift.android.data.remote.dto.RegisterDeviceRequest
import com.perfectgift.android.data.remote.dto.RegisterDeviceResponse
import com.perfectgift.android.data.remote.dto.RequestGenerationRequest
import com.perfectgift.android.data.remote.dto.RevokeRequest
import com.perfectgift.android.data.remote.dto.SignInRequest
import com.perfectgift.android.data.remote.dto.SubmitResponseRequest
import com.perfectgift.android.data.remote.dto.TokenPair
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import retrofit2.Response

/**
 * Configurable fake implementing the whole [GatewayApi] in-memory. No live backend, no
 * network. Individual tests override just the behaviour they exercise.
 */
open class FakeGatewayApi : GatewayApi {

    /** Scripted statuses returned in order by [getGeneration]; the last one repeats. */
    var generationScript: ArrayDeque<GenerationStatus> = ArrayDeque()

    /** Capture the Idempotency-Key the repository generates for a submit. */
    var lastIdempotencyKey: String? = null
    var lastGenerationRequest: RequestGenerationRequest? = null

    var signInResult: Response<TokenPair> = ok(TokenPair("access-1", "refresh-1", 3600))
    var acceptedResult: Response<GenerationAccepted> = ok(GenerationAccepted("req-1", "queued"))
    var meResult: Response<MeResponse> = ok(MeResponse())

    override suspend fun signIn(body: SignInRequest): Response<TokenPair> = signInResult

    override suspend fun refresh(body: RefreshRequest): Response<TokenPair> =
        ok(TokenPair("access-2", "refresh-2", 3600))

    override suspend fun revoke(body: RevokeRequest): Response<OkResponse> = ok(OkResponse(true))

    override suspend fun getMe(): Response<MeResponse> = meResult

    override suspend fun requestGeneration(
        idempotencyKey: String,
        body: RequestGenerationRequest,
    ): Response<GenerationAccepted> {
        lastIdempotencyKey = idempotencyKey
        lastGenerationRequest = body
        return acceptedResult
    }

    override suspend fun getGeneration(id: String): Response<GenerationStatus> {
        val next = if (generationScript.size > 1) generationScript.removeFirst() else generationScript.firstOrNull()
        return ok(next ?: GenerationStatus(requestId = id, status = "ready"))
    }

    override suspend fun refine(id: String, body: RefineRequest): Response<GenerationAccepted> = acceptedResult

    override suspend fun saveIdea(id: String): Response<OkResponse> = ok(OkResponse(true))

    override suspend fun createPoll(body: CreatePollRequest): Response<CreatePollResponse> =
        ok(CreatePollResponse("poll-1", "tok-1", "https://perfectgift.app/p/tok-1", "2030-01-01T00:00:00Z"))

    override suspend fun getPollResponses(id: String): Response<PollResponsesResponse> =
        ok(PollResponsesResponse(emptyList()))

    override suspend fun getPollByToken(token: String): Response<PollByToken> =
        ok(PollByToken("poll-1", "A quick question", emptyList()))

    override suspend fun submitPollResponse(token: String, body: SubmitResponseRequest): Response<OkResponse> =
        ok(OkResponse(true))

    override suspend fun listHolidays(region: String?, active: Boolean?, onOrAfter: String?): Response<HolidaysResponse> =
        ok(HolidaysResponse(emptyList()))

    override suspend fun getCategories(kind: String?): Response<CategoriesResponse> =
        ok(CategoriesResponse(emptyList(), emptyList()))

    override suspend fun registerDevice(body: RegisterDeviceRequest): Response<RegisterDeviceResponse> =
        ok(RegisterDeviceResponse("device-1"))

    companion object {
        fun <T> ok(body: T): Response<T> = Response.success(body)
        fun <T> error(code: Int, json: String): Response<T> =
            Response.error(code, json.toResponseBody("application/json".toMediaType()))
    }
}
