package com.slouchcat.app.mobile.push

import android.content.Context
import com.slouchcat.app.mobile.AppGraph

/**
 * One arriving push message, whatever brought it (Firebase in `play`, the emulator flow's test
 * feeding what `fcmfake` received): decrypt with this device's key, show, and let the bell read
 * its number again. Returns the notification's id, `null` when nothing was shown.
 */
object PushReceiver {
    suspend fun receive(context: Context, graph: AppGraph, wp: String): Int? {
        val keys = graph.push.keysFor() ?: return null
        val (message, path) = PushNotifier.open(wp, keys, graph.session.server) ?: return null
        // The inbox has something new: the bell reads its number when the start page shows next.
        graph.media.unreadCount.set(null)
        return PushNotifier.show(context, message, path)
    }
}
