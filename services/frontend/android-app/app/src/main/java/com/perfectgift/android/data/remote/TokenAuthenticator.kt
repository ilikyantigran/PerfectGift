package com.perfectgift.android.data.remote

import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.dto.RefreshRequest
import com.perfectgift.android.data.remote.dto.TokenPair
import okhttp3.Authenticator
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route

/**
 * On a 401, rotates the refresh token via `/v1/auth/refresh` and retries the failed
 * request once with the new access token. If refresh fails (or there is no refresh
 * token), it signs the session out and gives up (returns null → the 401 propagates).
 *
 * OkHttp calls authenticators on a background/network thread and serializes retries for
 * a given call, so a blocking refresh here is correct. [refresh] is a plain suspend/
 * blocking call performed on a SEPARATE OkHttp client with no authenticator, to avoid
 * infinite recursion.
 */
class TokenAuthenticator(
    private val session: SessionManager,
    private val refresh: (RefreshRequest) -> TokenPair?,
) : Authenticator {

    override fun authenticate(route: Route?, response: Response): Request? {
        // Give up after a single retry (avoid loops if the refreshed token is still rejected).
        if (responseCount(response) >= 2) {
            return null
        }

        val refreshToken = session.currentRefreshToken() ?: return null

        // If another thread already refreshed while we were queued, just retry with the
        // now-current token instead of refreshing again.
        val staleAccess = tokenFromHeader(response.request)
        val currentAccess = session.currentAccessToken()
        if (currentAccess != null && currentAccess != staleAccess) {
            return response.request.newBuilder()
                .header("Authorization", "Bearer $currentAccess")
                .build()
        }

        val newTokens = try {
            refresh(RefreshRequest(refreshToken))
        } catch (e: Exception) {
            null
        }

        if (newTokens == null || newTokens.accessToken.isBlank()) {
            session.signOutBlocking()
            return null
        }

        session.updateBlocking(newTokens.accessToken, newTokens.refreshToken)

        return response.request.newBuilder()
            .header("Authorization", "Bearer ${newTokens.accessToken}")
            .build()
    }

    private fun tokenFromHeader(request: Request): String? =
        request.header("Authorization")?.removePrefix("Bearer ")?.trim()

    private fun responseCount(response: Response): Int {
        var count = 1
        var prior = response.priorResponse
        while (prior != null) {
            count++
            prior = prior.priorResponse
        }
        return count
    }
}
