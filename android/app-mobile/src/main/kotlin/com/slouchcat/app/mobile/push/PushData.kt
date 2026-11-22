package com.slouchcat.app.mobile.push

import android.app.Application
import android.content.Context
import android.content.Intent
import android.net.Uri
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.BuildConfig
import com.slouchcat.app.mobile.common.SignOutStep
import com.slouchcat.app.mobile.more.DevicesRoute
import com.slouchcat.app.mobile.more.MoreRoute
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.push.PushRelay
import com.slouchcat.core.session.KeystoreSecretBox
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import okhttp3.HttpUrl.Companion.toHttpUrl
import java.io.File
import java.util.WeakHashMap

/**
 * Push of this graph (stage 8): the registrar over the push relay, its store and Google's token.
 * Tests put their own in place ([install]).
 */
object PushData {
    private val installed = WeakHashMap<AppGraph, PushRegistrar>()

    fun install(graph: AppGraph, registrar: PushRegistrar) = synchronized(installed) { installed[graph] = registrar }

    fun of(graph: AppGraph, context: Context): PushRegistrar = synchronized(installed) {
        installed.getOrPut(graph) { create(context.applicationContext, graph) }
    }

    /** The address of the push relay of this build (`slouchcat.pushRelay`, default `https://push.slouchcat.com`). */
    val relayAddress: String get() = BuildConfig.PUSH_RELAY

    private fun create(context: Context, graph: AppGraph): PushRegistrar {
        val relay = PushRelay(relayAddress.toHttpUrl(), graph.httpForRelay)
        return PushRegistrar(
            session = graph.session,
            channels = graph.media.channels,
            relay = { token, vapid -> relay.register(token, vapid) },
            platform = PushPlatforms.create(context),
            store = FilePushStore(File(context.noBackupFilesDir, "push.bin"), KeystoreSecretBox(STORE_KEY)),
            deviceName = { graph.device.name },
        )
    }

    private const val STORE_KEY = "slouchcat-push"
}

/** A client without the session's credentials: the relay never sees the token of the server. */
private val AppGraph.httpForRelay: okhttp3.OkHttpClient get() = relayClient

private val relayClient: okhttp3.OkHttpClient by lazy { okhttp3.OkHttpClient.Builder().callTimeout(java.time.Duration.ofSeconds(20)).build() }

/** The push of this graph; the first use builds the real one. */
val AppGraph.push: PushRegistrar get() = PushData.of(this, appContextForPush ?: error("push not started"))

private var appContextForPush: Context? = null

/**
 * Starts push with the app (concept 27, 7.3): the system's channels, a sync whenever a session is
 * there (start, sign-in, another server), again after the account changed (a new password deletes
 * the app's subscriptions on the server) and on a new Google token; signing out forgets the record
 * (the server deleted the subscription with the session).
 */
object PushStarter {
    fun start(application: Application, graph: AppGraph) {
        appContextForPush = application.applicationContext
        PushNotifier.ensureChannels(application)
        graph.hooks.addSignOutStep(
            object : SignOutStep {
                override suspend fun beforeSignOut(answer: Boolean) = graph.push.forget()
            },
        )
        graph.hooks.onAccountChanged { graph.push.sync() }
        graph.scope.launch {
            graph.session.state
                .map { state -> (state as? SessionState.Ready)?.server?.address?.toString() ?: (state as? SessionState.ChooseProfile)?.server?.address?.toString() }
                .distinctUntilChanged()
                .collect { server -> if (server != null) runCatching { graph.push.sync() }.onFailure { Log.w("push", "sync failed", error = it) } }
        }
    }

    /** Google gave a new token: enter the subscription again. */
    fun tokenChanged(graph: AppGraph) {
        graph.scope.launch { runCatching { graph.push.sync() } }
    }
}

/**
 * Opens the path of a tapped notification inside the app (concept 27, 7.3 point 6): an episode or
 * a fulfilled wish its title, a wish «Meine Wünsche», a new device «Meine Geräte», the
 * administration in the browser under the own server's address; anything else the start.
 */
object PushLinks {
    fun open(context: Context, graph: AppGraph, path: String?) {
        val clean = path?.takeIf { it.startsWith("/") } ?: return
        val bare = clean.substringBefore('?').trimEnd('/')
        when {
            bare.isEmpty() -> Unit
            bare == "/admin" || bare.startsWith("/admin/") -> graph.session.server?.let { server ->
                val url = server.resolve(clean.removePrefix("/")) ?: return
                runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url.toString())).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
            }
            bare.startsWith("/items/") -> graph.links.request("item/" + bare.removePrefix("/items/"))
            bare == "/settings/devices" -> graph.links.request(AppLink("more", listOf(MoreRoute, DevicesRoute)))
            else -> graph.links.request(clean.removePrefix("/"))
        }
    }
}
