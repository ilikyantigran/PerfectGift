package com.perfectgift.android

import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.NetworkModule
import com.perfectgift.android.util.FakeTokenStore
import kotlinx.coroutines.runBlocking
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Exercises the real OkHttp [com.perfectgift.android.data.remote.TokenAuthenticator]
 * against a MockWebServer (in-process; no live backend): a 401 triggers a refresh and a
 * single retry with the rotated access token.
 */
class TokenRefreshTest {

    private lateinit var server: MockWebServer

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `401 triggers refresh and retries the request once with the new token`() = runBlocking {
        val session = SessionManager(FakeTokenStore())
        session.onSignedIn(access = "old-access", refresh = "old-refresh")

        val api = NetworkModule.create(
            baseUrl = server.url("/").toString(),
            session = session,
            debug = false,
        )

        // 1) /v1/me → 401  2) /v1/auth/refresh → 200 new tokens  3) /v1/me retry → 200
        server.enqueue(
            MockResponse().setResponseCode(401)
                .setBody("""{"error":{"code":"token_expired","message":"expired"}}"""),
        )
        server.enqueue(
            MockResponse().setResponseCode(200)
                .setBody("""{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}"""),
        )
        server.enqueue(
            MockResponse().setResponseCode(200)
                .setBody("""{"user":{"id":"u1","email":"a@b.com"}}"""),
        )

        val response = api.getMe()
        assertTrue("expected a successful retry", response.isSuccessful)
        assertEquals("u1", response.body()?.user?.id)

        // Three requests in order, with the right auth headers.
        val first = server.takeRequest()
        assertEquals("/v1/me", first.path)
        assertEquals("Bearer old-access", first.getHeader("Authorization"))

        val refresh = server.takeRequest()
        assertEquals("/v1/auth/refresh", refresh.path)
        assertTrue(refresh.body.readUtf8().contains("old-refresh"))

        val retried = server.takeRequest()
        assertEquals("/v1/me", retried.path)
        assertEquals("Bearer new-access", retried.getHeader("Authorization"))

        // Session was updated to the rotated tokens.
        assertEquals("new-access", session.currentAccessToken())
        assertEquals("new-refresh", session.currentRefreshToken())
    }

    @Test
    fun `refresh failure signs the session out and gives up`() = runBlocking {
        val session = SessionManager(FakeTokenStore())
        session.onSignedIn(access = "old-access", refresh = "old-refresh")

        val api = NetworkModule.create(server.url("/").toString(), session, debug = false)

        server.enqueue(MockResponse().setResponseCode(401).setBody("""{"error":{"code":"x","message":"y"}}"""))
        // Refresh itself is rejected (e.g. refresh token revoked).
        server.enqueue(MockResponse().setResponseCode(401).setBody("""{"error":{"code":"x","message":"y"}}"""))

        val response = api.getMe()
        assertEquals(401, response.code())
        assertTrue("session should be cleared", session.currentAccessToken() == null)
    }
}
