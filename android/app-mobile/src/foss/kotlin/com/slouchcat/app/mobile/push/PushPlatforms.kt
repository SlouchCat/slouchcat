package com.slouchcat.app.mobile.push

import android.content.Context

/**
 * The bundle `foss` has no Google services and so no push (concept 27, 5.2, F-33; R-5): what is
 * new appears in «Benachrichtigungen» when the app opens, and by e-mail or Telegram.
 */
object PushPlatforms {
    val configured: Boolean get() = false

    fun create(context: Context): PushPlatform = object : PushPlatform {
        override val available = false

        override suspend fun token(): String? = null
    }
}
