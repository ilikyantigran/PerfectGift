package com.perfectgift.android

import com.perfectgift.android.data.auth.SessionManager
import com.perfectgift.android.data.remote.ApiResult
import com.perfectgift.android.data.remote.NetworkModule
import com.perfectgift.android.data.remote.parseError
import com.perfectgift.android.data.repository.PerfectGiftRepositoryImpl
import com.perfectgift.android.util.FakeGatewayApi
import com.perfectgift.android.util.FakeTokenStore
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ErrorEnvelopeTest {

    private val gson = NetworkModule.gson()

    @Test
    fun `decodes the standard error envelope into code, message and details`() {
        val json = """
            {"error":{"code":"not_found","message":"No such generation","details":{"id":"req-9"}}}
        """.trimIndent()

        val error = parseError(404, json, gson)

        assertEquals(404, error.httpStatus)
        assertEquals("not_found", error.code)
        assertEquals("No such generation", error.message)
        assertEquals("req-9", error.details?.get("id"))
    }

    @Test
    fun `falls back to a friendly message when the body is not an envelope`() {
        val error = parseError(500, "<html>oops</html>", gson)
        assertEquals(500, error.httpStatus)
        assertTrue(error.message.isNotBlank())
    }

    @Test
    fun `repository maps a non-2xx response to ApiResult Failure with the decoded error`() = runTest {
        val api = FakeGatewayApi().apply {
            meResult = FakeGatewayApi.error(403, """{"error":{"code":"forbidden","message":"Nope"}}""")
        }
        val repo = PerfectGiftRepositoryImpl(api, SessionManager(FakeTokenStore()), gson)

        val result = repo.getMe()

        assertTrue(result is ApiResult.Failure)
        val error = (result as ApiResult.Failure).error
        assertEquals("forbidden", error.code)
        assertEquals("Nope", error.message)
    }
}
