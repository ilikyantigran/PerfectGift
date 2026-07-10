package com.perfectgift.android

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.os.Build
import com.perfectgift.android.di.AppContainer
import kotlinx.coroutines.runBlocking

class PerfectGiftApp : Application() {

    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
        // Load any persisted tokens into memory before the first screen decides whether
        // the user is signed in. A single cheap DataStore read at cold start.
        runBlocking { container.session.bootstrap() }
        createNotificationChannel()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                getString(R.string.fcm_channel_id),
                getString(R.string.fcm_channel_name),
                NotificationManager.IMPORTANCE_DEFAULT,
            )
            getSystemService(NotificationManager::class.java)?.createNotificationChannel(channel)
        }
    }
}
