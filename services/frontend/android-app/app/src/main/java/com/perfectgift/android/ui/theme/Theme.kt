package com.perfectgift.android.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val Purple = Color(0xFF6750A4)
private val PurpleLight = Color(0xFFD0BCFF)
private val Pink = Color(0xFF7D5260)

private val LightColors = lightColorScheme(
    primary = Purple,
    secondary = Pink,
)

private val DarkColors = darkColorScheme(
    primary = PurpleLight,
    secondary = Color(0xFFEFB8C8),
)

@Composable
fun PerfectGiftTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        typography = Typography(),
        content = content,
    )
}
