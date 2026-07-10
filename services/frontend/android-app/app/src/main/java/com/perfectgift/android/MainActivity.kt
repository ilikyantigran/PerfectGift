package com.perfectgift.android

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.remember
import androidx.lifecycle.lifecycleScope
import androidx.navigation.compose.rememberNavController
import com.perfectgift.android.ui.nav.PerfectGiftNavHost
import com.perfectgift.android.ui.nav.Routes
import com.perfectgift.android.ui.theme.PerfectGiftTheme
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {

    private val container get() = (application as PerfectGiftApp).container

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val deepLinkToken = extractPollToken(intent)

        setContent {
            PerfectGiftTheme {
                val navController = rememberNavController()
                val startSignedIn = remember { container.session.isSignedIn }

                PerfectGiftNavHost(
                    navController = navController,
                    startSignedIn = startSignedIn,
                    onSignOut = {
                        lifecycleScope.launch { container.repository.signOut() }
                    },
                )

                // If launched from a shared poll App Link, jump straight to the Subject form.
                androidx.compose.runtime.LaunchedEffect(deepLinkToken) {
                    if (!deepLinkToken.isNullOrBlank()) {
                        navController.navigate(Routes.subject(deepLinkToken))
                    }
                }
            }
        }
    }

    /** Pull the opaque poll token out of an App Link like https://perfectgift.app/p/{token}. */
    private fun extractPollToken(intent: Intent?): String? {
        val data = intent?.data ?: return null
        val segments = data.pathSegments
        val idx = segments.indexOf("p")
        return if (idx >= 0 && idx + 1 < segments.size) segments[idx + 1] else null
    }
}
