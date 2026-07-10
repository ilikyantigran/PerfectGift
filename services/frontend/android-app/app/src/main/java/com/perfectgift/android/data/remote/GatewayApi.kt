package com.perfectgift.android.data.remote

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
import com.perfectgift.android.data.remote.dto.RevokeRequest
import com.perfectgift.android.data.remote.dto.SignInRequest
import com.perfectgift.android.data.remote.dto.SubmitResponseRequest
import com.perfectgift.android.data.remote.dto.TokenPair
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * The complete PerfectGift API Gateway REST contract (v1). This is the ONLY backend
 * surface the app talks to. All authenticated routes get the Bearer token attached by
 * [AuthInterceptor]; a 401 triggers [TokenAuthenticator] to refresh and retry once.
 *
 * Suspend functions return [Response] so the repository can inspect status codes and
 * decode the `{ error: { ... } }` envelope on failure.
 */
interface GatewayApi {

    // --- Auth (public) ---

    @POST("v1/auth/signin")
    suspend fun signIn(@Body body: SignInRequest): Response<TokenPair>

    @POST("v1/auth/refresh")
    suspend fun refresh(@Body body: RefreshRequest): Response<TokenPair>

    @POST("v1/auth/revoke")
    suspend fun revoke(@Body body: RevokeRequest): Response<OkResponse>

    @GET("v1/me")
    suspend fun getMe(): Response<MeResponse>

    // --- Generations (submit-then-observe) ---

    /**
     * Start a generation. Async: returns 202 with a requestId. The [idempotencyKey]
     * makes duplicate submits safe (forwarded to Surprise).
     */
    @POST("v1/generations")
    suspend fun requestGeneration(
        @Header("Idempotency-Key") idempotencyKey: String,
        @Body body: com.perfectgift.android.data.remote.dto.RequestGenerationRequest,
    ): Response<GenerationAccepted>

    /** Poll status; ideas are included once status == "ready". */
    @GET("v1/generations/{id}")
    suspend fun getGeneration(@Path("id") id: String): Response<GenerationStatus>

    @POST("v1/generations/{id}/refine")
    suspend fun refine(
        @Path("id") id: String,
        @Body body: RefineRequest,
    ): Response<GenerationAccepted>

    @POST("v1/ideas/{id}/save")
    suspend fun saveIdea(@Path("id") id: String): Response<OkResponse>

    // --- Polls (owner, JWT) ---

    @POST("v1/polls")
    suspend fun createPoll(@Body body: CreatePollRequest): Response<CreatePollResponse>

    @GET("v1/polls/{id}/responses")
    suspend fun getPollResponses(@Path("id") id: String): Response<PollResponsesResponse>

    // --- Polls (anonymous Subject, opaque link token, no JWT) ---

    @GET("v1/polls/token/{t}")
    suspend fun getPollByToken(@Path("t") token: String): Response<PollByToken>

    @POST("v1/polls/token/{t}/responses")
    suspend fun submitPollResponse(
        @Path("t") token: String,
        @Body body: SubmitResponseRequest,
    ): Response<OkResponse>

    // --- Catalog reference (JWT) ---

    @GET("v1/holidays")
    suspend fun listHolidays(
        @Query("region") region: String? = null,
        @Query("active") active: Boolean? = null,
        @Query("on_or_after") onOrAfter: String? = null,
    ): Response<HolidaysResponse>

    @GET("v1/categories")
    suspend fun getCategories(@Query("kind") kind: String? = null): Response<CategoriesResponse>

    // --- Devices / push (JWT) ---

    @POST("v1/devices")
    suspend fun registerDevice(@Body body: RegisterDeviceRequest): Response<RegisterDeviceResponse>
}
