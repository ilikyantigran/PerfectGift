package com.perfectgift.android.ui.nav

/** Central route names. Kept as plain strings; the Subject route carries a link token. */
object Routes {
    const val SIGN_IN = "signin"

    // Nested graph shared by the occasion → generating → ideas screens.
    const val GENERATION_GRAPH = "generation"
    const val OCCASION = "occasion"
    const val GENERATING = "generating"
    const val IDEAS = "ideas"

    const val POLL_CREATE = "poll_create"

    // Anonymous Subject flow (also the App Link target). Arg: the opaque link token.
    const val SUBJECT = "subject"
    const val SUBJECT_ARG_TOKEN = "token"
    const val SUBJECT_ROUTE = "$SUBJECT/{$SUBJECT_ARG_TOKEN}"

    fun subject(token: String) = "$SUBJECT/$token"
}
