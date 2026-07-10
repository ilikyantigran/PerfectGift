package com.perfectgift.android.push

import android.Manifest
import android.content.pm.PackageManager
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import com.perfectgift.android.PerfectGiftApp
import com.perfectgift.android.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

/**
 * Receives "Ideas ready" / "Poll done" pushes (submit-then-observe can advance on a push
 * instead of polling) and registers the FCM device token with the backend.
 *
 * Compiles without a google-services.json; it only becomes active at runtime once
 * Firebase is configured (see README "Push (FCM)").
 */
class PerfectGiftMessagingService : FirebaseMessagingService() {

    private val scope = CoroutineScope(Dispatchers.IO)

    override fun onNewToken(token: String) {
        // Best-effort: register the new token if the user is signed in.
        val container = (application as? PerfectGiftApp)?.container ?: return
        if (container.session.isSignedIn) {
            scope.launch {
                runCatching {
                    container.repository.registerDevice(pushToken = token, appVersion = "1.0")
                }
            }
        }
    }

    override fun onMessageReceived(message: RemoteMessage) {
        val title = message.notification?.title ?: message.data["title"] ?: "PerfectGift"
        val body = message.notification?.body ?: message.data["body"] ?: "You have an update."
        showNotification(title, body)
    }

    private fun showNotification(title: String, body: String) {
        val builder = NotificationCompat.Builder(this, getString(R.string.fcm_channel_id))
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle(title)
            .setContentText(body)
            .setAutoCancel(true)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)

        val granted = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.POST_NOTIFICATIONS,
        ) == PackageManager.PERMISSION_GRANTED

        if (granted) {
            NotificationManagerCompat.from(this).notify(System.currentTimeMillis().toInt(), builder.build())
        }
    }
}
