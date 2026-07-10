package com.perfectgift.android.data.remote

import com.google.gson.FieldNamingPolicy
import com.google.gson.Gson
import com.google.gson.GsonBuilder
import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.dto.RefreshRequest
import com.perfectgift.android.data.remote.dto.TokenPair
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.TimeUnit

/**
 * Builds the networking stack by hand (no DI framework needed):
 *  - a shared Gson that maps camelCase Kotlin fields ⇄ snake_case JSON,
 *  - a "bare" client + api used only for token refresh (no authenticator → no recursion),
 *  - the main client with [AuthInterceptor] + [TokenAuthenticator], and the app's [GatewayApi].
 */
object NetworkModule {

    fun gson(): Gson = GsonBuilder()
        .setFieldNamingPolicy(FieldNamingPolicy.LOWER_CASE_WITH_UNDERSCORES)
        .create()

    fun create(baseUrl: String, session: SessionManager, debug: Boolean): GatewayApi {
        val gson = gson()

        val logging = HttpLoggingInterceptor().apply {
            level = if (debug) HttpLoggingInterceptor.Level.BODY else HttpLoggingInterceptor.Level.NONE
        }

        // Bare client for refresh only — deliberately has NO authenticator/auth interceptor.
        val bareClient = OkHttpClient.Builder()
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(20, TimeUnit.SECONDS)
            .addInterceptor(logging)
            .build()

        val refreshApi: GatewayApi = Retrofit.Builder()
            .baseUrl(baseUrl)
            .client(bareClient)
            .addConverterFactory(GsonConverterFactory.create(gson))
            .build()
            .create(GatewayApi::class.java)

        val authenticator = TokenAuthenticator(session) { req: RefreshRequest ->
            refreshSynchronously(refreshApi, req)
        }

        val mainClient = OkHttpClient.Builder()
            .connectTimeout(15, TimeUnit.SECONDS)
            // Generation status polling is quick; the app never blocks on the ~3–15 s
            // LLM work because generation is 202-async and observed via polling.
            .readTimeout(30, TimeUnit.SECONDS)
            .addInterceptor(AuthInterceptor(session))
            .authenticator(authenticator)
            .addInterceptor(logging)
            .build()

        return Retrofit.Builder()
            .baseUrl(baseUrl)
            .client(mainClient)
            .addConverterFactory(GsonConverterFactory.create(gson))
            .build()
            .create(GatewayApi::class.java)
    }

    /** Blocking refresh for the OkHttp authenticator (runs on a network thread). */
    private fun refreshSynchronously(api: GatewayApi, req: RefreshRequest): TokenPair? =
        kotlinx.coroutines.runBlocking {
            runCatching { api.refresh(req) }
                .getOrNull()
                ?.takeIf { it.isSuccessful }
                ?.body()
        }
}
