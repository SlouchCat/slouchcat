package com.slouchcat.app.mobile.push

import android.content.Context
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import com.slouchcat.app.mobile.BuildConfig
import com.slouchcat.app.mobile.SlouchcatApplication
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeoutOrNull
import kotlin.coroutines.resume

/**
 * Google's push in the bundle `play` (concept 27, 7.3): Firebase set up by hand from the four
 * values of the build ([BuildConfig], no google-services.json). Without them push is off in this
 * build ([PushPlatform.available] `false`).
 */
object PushPlatforms {
    private const val TAG = "push"
    private const val TOKEN_TIMEOUT_MS = 30_000L

    /** The Firebase values of this build are all there. */
    val configured: Boolean
        get() = listOf(BuildConfig.FIREBASE_PROJECT_ID, BuildConfig.FIREBASE_APP_ID, BuildConfig.FIREBASE_API_KEY, BuildConfig.FIREBASE_SENDER_ID).all { it.isNotBlank() }

    fun create(context: Context): PushPlatform {
        if (!configured) return Off
        val app = context.applicationContext
        val ready = runCatching {
            if (FirebaseApp.getApps(app).isEmpty()) {
                val options = FirebaseOptions.Builder()
                    .setProjectId(BuildConfig.FIREBASE_PROJECT_ID)
                    .setApplicationId(BuildConfig.FIREBASE_APP_ID)
                    .setApiKey(BuildConfig.FIREBASE_API_KEY)
                    .setGcmSenderId(BuildConfig.FIREBASE_SENDER_ID)
                    .build()
                FirebaseApp.initializeApp(app, options)
            }
        }.onFailure { Log.w(TAG, "firebase not set up", error = it) }.isSuccess
        if (!ready) return Off
        return object : PushPlatform {
            override val available = true

            // Firebase fetches its installation id and token only while push is wanted: the manifest
            // switches the automatic start off, the first wanted sync switches it on (code review m-6).
            override suspend fun token(): String? = withTimeoutOrNull(TOKEN_TIMEOUT_MS) {
                FirebaseMessaging.getInstance().isAutoInitEnabled = true
                suspendCancellableCoroutine { continuation ->
                    FirebaseMessaging.getInstance().token
                        .addOnSuccessListener { continuation.resume(it) }
                        .addOnFailureListener {
                            Log.w(TAG, "no token", error = it)
                            continuation.resume(null)
                        }
                }
            }

            override suspend fun stop() {
                FirebaseMessaging.getInstance().isAutoInitEnabled = false
                withTimeoutOrNull(TOKEN_TIMEOUT_MS) {
                    suspendCancellableCoroutine { continuation ->
                        FirebaseMessaging.getInstance().deleteToken()
                            .addOnCompleteListener { continuation.resume(Unit) }
                    }
                }
            }
        }
    }

    private object Off : PushPlatform {
        override val available = false

        override suspend fun token(): String? = null
    }
}

/**
 * The data messages of the push relay (`data.wp`, ADR 0044) and Google's new tokens. The message
 * is decrypted on the device (Web-Push, RFC 8291) and shown; a new token enters the subscription
 * again. Nothing of the message or the token reaches the log (rule 10).
 */
class SlouchcatMessagingService : FirebaseMessagingService() {
    override fun onMessageReceived(message: RemoteMessage) {
        // Only the relay's data messages: a «notification» message never leads anywhere (security review M-1).
        val wp = message.data["wp"] ?: return
        val graph = (application as SlouchcatApplication).graph
        runBlocking { PushReceiver.receive(this@SlouchcatMessagingService, graph, wp) }
    }

    override fun onNewToken(token: String) {
        PushStarter.tokenChanged((application as SlouchcatApplication).graph)
    }
}
