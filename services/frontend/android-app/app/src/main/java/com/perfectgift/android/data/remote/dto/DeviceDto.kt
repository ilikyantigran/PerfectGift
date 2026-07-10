package com.perfectgift.android.data.remote.dto

/** POST /v1/devices — register an FCM device token. */
data class RegisterDeviceRequest(
    /** ios | android. */
    val platform: String = "android",
    val pushToken: String,
    val appVersion: String? = null,
)

data class RegisterDeviceResponse(
    val deviceId: String? = null,
)
