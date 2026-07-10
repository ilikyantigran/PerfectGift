package com.perfectgift.android.data.remote.dto

import com.google.gson.annotations.SerializedName

/**
 * Sign-in provider values.
 *
 * NOTE ON WIRE FORMAT: per the client build directive, enums are sent by their full
 * protobuf name (PROVIDER_EMAIL / PROVIDER_APPLE / PROVIDER_GOOGLE). If the deployed
 * gateway instead expects the short lower-case form (`email` / `apple` / `google`, as
 * the current openapi.yaml example shows), change only the [SerializedName] values
 * below — nothing else in the app references the raw strings.
 */
enum class AuthProvider {
    @SerializedName("PROVIDER_EMAIL") EMAIL,
    @SerializedName("PROVIDER_APPLE") APPLE,
    @SerializedName("PROVIDER_GOOGLE") GOOGLE,
}

/** POST /v1/auth/signin */
data class SignInRequest(
    val provider: AuthProvider,
    /** Provider-issued ID token (Google/Apple social sign-in). */
    val idToken: String? = null,
    /** Email + password fallback. */
    val email: String? = null,
    val password: String? = null,
)

data class UserDto(
    val id: String? = null,
    val email: String? = null,
    val displayName: String? = null,
    val status: String? = null,
)

/** Returned by /v1/auth/signin and /v1/auth/refresh. */
data class TokenPair(
    val accessToken: String,
    val refreshToken: String,
    val expiresIn: Long = 0,
    val user: UserDto? = null,
)

/** POST /v1/auth/refresh */
data class RefreshRequest(
    val refreshToken: String,
)

/** POST /v1/auth/revoke */
data class RevokeRequest(
    val refreshToken: String? = null,
    val sessionId: String? = null,
)

/** GET /v1/me */
data class MeResponse(
    val user: UserDto? = null,
)

/** Shared { ok: true } success body. */
data class OkResponse(
    val ok: Boolean = false,
)
