package com.slouchcat.app.mobile.push

import android.content.Context
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.core.browse.notifications.badge
import kotlinx.coroutines.withTimeoutOrNull
import java.security.MessageDigest
import java.util.ArrayDeque

/**
 * One arriving push message, whatever brought it (Firebase in `play`, the emulator flow's test
 * feeding what `fcmfake` received): decrypt with this device's key, show, and let the bell read
 * its number again. Returns the notification's id, `null` when nothing was shown.
 */
object PushReceiver {
    /** The time a push that woke the process waits for the stored session (code review m-4). */
    private const val SESSION_WAIT_MS = 3_000L

    suspend fun receive(context: Context, graph: AppGraph, wp: String): Int? {
        // Woken by a push, the session is still being read: its server makes absolute links paths.
        withTimeoutOrNull(SESSION_WAIT_MS) { graph.session.ready() }
        val keys = graph.push.keysFor() ?: return null
        val body = com.slouchcat.core.api.push.PushMessage.body(wp) ?: return null
        // The same message again (kept and sent once more by relay, proxy or Google): shown once only
        // (security review L-4; the server's payload carries no time or id to check against).
        if (RecentMessages.file == null) RecentMessages.file = java.io.File(context.noBackupFilesDir, "push-seen.txt")
        if (!RecentMessages.firstTime(body)) return null
        val (message, path) = PushNotifier.open(body, keys, graph.session.server) ?: return null
        // The inbox has something new: the bell reads its number again, also while «Start» is open
        // (code review M-2); a failed read keeps the shown number.
        graph.session.server?.let { server -> graph.media.unreadCount.refresh { graph.media.notifications.badge(server) } }
        return PushNotifier.show(context, message, path)
    }
}

/**
 * The hashes of the last messages shown (bounded, kept in [file] across restarts of the process): a
 * replayed body is dropped. The hash of the encrypted body tells nothing about the message.
 */
internal object RecentMessages {
    private const val KEEP = 64
    private val seen = ArrayDeque<String>()
    var file: java.io.File? = null
    private var loaded = false

    @Synchronized
    fun firstTime(body: ByteArray): Boolean {
        if (!loaded) {
            loaded = true
            runCatching { file?.takeIf { it.exists() }?.readLines()?.takeLast(KEEP)?.forEach { seen.addLast(it) } }
        }
        val hash = MessageDigest.getInstance("SHA-256").digest(body).joinToString("") { "%02x".format(it) }
        if (hash in seen) return false
        seen.addLast(hash)
        while (seen.size > KEEP) seen.removeFirst()
        runCatching { file?.writeText(seen.joinToString("\n")) }
        return true
    }

    @Synchronized
    fun reset() {
        seen.clear()
        loaded = false
    }
}
