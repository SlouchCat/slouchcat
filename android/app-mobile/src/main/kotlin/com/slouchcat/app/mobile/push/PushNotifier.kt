package com.slouchcat.app.mobile.push

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.slouchcat.app.mobile.R as AppR
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.push.PushKeys
import com.slouchcat.core.api.push.PushMessage
import com.slouchcat.core.api.push.WebPush
import com.slouchcat.core.api.push.WebPushException
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl

/** The channels of the system the app's notifications go to (the person can mute each in Android). */
enum class PushChannel(val id: String) {
    Content("content"),
    Wishes("wishes"),
    Account("account"),
    General("general"),
    ;

    companion object {
        /** By the path the server links to: an episode, a wish, «Meine Geräte», everything else. */
        fun of(path: String?): PushChannel = when {
            path == null -> General
            path.startsWith("/wishes") || path.startsWith("/titles/") -> Wishes
            path.startsWith("/items/") -> Content
            path.startsWith("/settings/devices") -> Account
            else -> General
        }
    }
}

/**
 * An arriving push message (concept 27, 7.3 point 6): the FCM data field `wp` is the server's
 * encrypted body; the device decrypts it with its own key ([WebPush]), shows title and text the
 * server wrote (the profile's name is in the title, rule 12) and, on a tap, opens the path of
 * `url` inside the app. A foreign host is never opened; nothing of the message reaches the log.
 */
object PushNotifier {
    private const val TAG = "push"

    /** The extra of the activity's intent: the in-app path a notification leads to. */
    const val EXTRA_PATH = "com.slouchcat.app.PUSH_PATH"

    /** Decrypts [body] (the decoded `data.wp`) for [keys]; `null` when it is no message to this device. */
    fun open(body: ByteArray, keys: PushKeys, server: HttpUrl?): Pair<PushMessage, String?>? {
        val plain = try {
            WebPush.decrypt(body, keys)
        } catch (e: WebPushException) {
            Log.w(TAG, "message does not decrypt")
            return null
        }
        val message = PushMessage.parse(plain) ?: return null
        return message to PushMessage.path(message.url, server)
    }

    fun ensureChannels(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        val names = mapOf(
            PushChannel.Content to R.string.app_notifications_system_content,
            PushChannel.Wishes to R.string.app_notifications_system_wishes,
            PushChannel.Account to R.string.app_notifications_system_account,
            PushChannel.General to R.string.app_notifications_system_general,
        )
        manager.createNotificationChannels(
            names.map { (channel, name) ->
                NotificationChannel(channel.id, context.getString(name), if (channel == PushChannel.Account) NotificationManager.IMPORTANCE_HIGH else NotificationManager.IMPORTANCE_DEFAULT)
            },
        )
    }

    /** Whether Android shows the app's notifications (the permission from Android 13, and not switched off). */
    fun allowed(context: Context): Boolean {
        val granted = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        return granted && NotificationManagerCompat.from(context).areNotificationsEnabled()
    }

    /** Shows [message]; a tap opens the app at [path]. Returns the id of the notification, `null` when Android does not let it show. */
    fun show(context: Context, message: PushMessage, path: String?): Int? {
        ensureChannels(context)
        if (!allowed(context)) return null
        // One id per message (a repeat replaces, two messages never do; code review L-7).
        val id = (message.hashCode() and 0x7fffffff)
        // The tap goes to an activity of the app that nothing outside it can start (not exported,
        // no intent filter): the path of a message the app decrypted itself, never of an extra
        // someone else set (security review M-1).
        val intent = Intent(context, PushTapActivity::class.java)
            .setFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            .putExtra(EXTRA_PATH, path ?: "/")
        val tap = PendingIntent.getActivity(context, id, intent, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val notification = NotificationCompat.Builder(context, PushChannel.of(path).id)
            .setSmallIcon(AppR.drawable.ic_notification)
            .setContentTitle(message.title)
            .setContentText(message.body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(message.body))
            .setAutoCancel(true)
            .setContentIntent(tap)
            // The lock screen shows only that there is something (security review L-5).
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(
                NotificationCompat.Builder(context, PushChannel.of(path).id)
                    .setSmallIcon(AppR.drawable.ic_notification)
                    .setContentTitle(context.getString(R.string.app_name))
                    .setContentText(context.getString(R.string.app_notifications_publicText))
                    .build(),
            )
            .build()
        return try {
            NotificationManagerCompat.from(context).notify(id, notification)
            id
        } catch (e: SecurityException) {
            null
        }
    }
}
