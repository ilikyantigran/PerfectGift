package com.perfectgift.android.data.remote

import com.perfectgift.android.data.auth.SessionManager
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Attaches `Authorization: Bearer <access_token>` to authenticated requests.
 *
 * Public routes (sign-in, refresh) and anonymous Subject token routes must NOT carry a
 * JWT, so they are skipped by path. Requests that already set Authorization explicitly
 * are left untouched.
 */
class AuthInterceptor(private val session: SessionManager) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val path = request.url.encodedPath

        if (isPublic(path) || request.header("Authorization") != null) {
            return chain.proceed(request)
        }

        val token = session.currentAccessToken()
            ?: return chain.proceed(request) // not signed in; let the 401 surface

        val authed = request.newBuilder()
            .header("Authorization", "Bearer $token")
            .build()
        return chain.proceed(authed)
    }

    private fun isPublic(path: String): Boolean =
        path.endsWith("/v1/auth/signin") ||
            path.endsWith("/v1/auth/refresh") ||
            path.contains("/v1/polls/token/")
}
