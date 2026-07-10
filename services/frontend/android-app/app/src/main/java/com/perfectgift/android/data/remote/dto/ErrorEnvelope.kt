package com.perfectgift.android.data.remote.dto

/**
 * Uniform error envelope returned by the gateway on any non-2xx:
 * `{ "error": { "code": "...", "message": "...", "details": { ... } } }`.
 */
data class ErrorEnvelope(
    val error: ErrorBody? = null,
)

data class ErrorBody(
    val code: String? = null,
    val message: String? = null,
    val details: Map<String, Any?>? = null,
)
