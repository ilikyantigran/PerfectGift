package com.perfectgift.android.data.remote

import com.google.gson.Gson
import com.perfectgift.android.data.remote.dto.ErrorEnvelope
import retrofit2.Response

/** Structured API error decoded from the `{ error: { code, message, details } }` envelope. */
data class ApiError(
    val httpStatus: Int,
    val code: String?,
    override val message: String,
    val details: Map<String, Any?>? = null,
) : Exception(message)

/** A minimal Result type so ViewModels never see raw Retrofit/HTTP details. */
sealed interface ApiResult<out T> {
    data class Success<T>(val data: T) : ApiResult<T>
    data class Failure(val error: ApiError) : ApiResult<Nothing>
}

/**
 * Decode a Retrofit [Response] into an [ApiResult], parsing the standard error envelope
 * on failure. [emptyValue] supplies the value for successful responses with no body
 * (e.g. 200 { ok:true } already typed, or 204).
 */
fun <T> Response<T>.toApiResult(gson: Gson): ApiResult<T> {
    if (isSuccessful) {
        val body = body()
        return if (body != null) {
            ApiResult.Success(body)
        } else {
            ApiResult.Failure(
                ApiError(code(), "empty_body", "The server returned an empty response."),
            )
        }
    }
    return ApiResult.Failure(parseError(code(), errorBody()?.string(), gson))
}

internal fun parseError(status: Int, rawBody: String?, gson: Gson): ApiError {
    if (!rawBody.isNullOrBlank()) {
        runCatching { gson.fromJson(rawBody, ErrorEnvelope::class.java) }
            .getOrNull()
            ?.error
            ?.let { e ->
                return ApiError(
                    httpStatus = status,
                    code = e.code,
                    message = e.message ?: defaultMessageFor(status),
                    details = e.details,
                )
            }
    }
    return ApiError(status, null, defaultMessageFor(status))
}

private fun defaultMessageFor(status: Int): String = when (status) {
    401 -> "Your session expired. Please sign in again."
    403 -> "You don't have access to that."
    404 -> "Not found."
    409 -> "That conflicts with something that already exists."
    422 -> "Some of the details weren't valid."
    429 -> "Too many requests — please slow down and try again."
    in 500..599 -> "Something went wrong on our end. Please try again."
    else -> "Request failed ($status)."
}
