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
import com.slouchcat.core.api.push.PushRelay
import com.slouchcat.core.session.KeystoreSecretBox
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import java.io.File
import java.time.Duration

/**
 * Push of a graph (stage 8): the registrar over the push relay, its store and Google's token.
 * The graph holds it (`graph.push`); tests give the graph's constructor their own.
 */
object PushData {
    /** The address of the push relay of this build (`slouchcat.pushRelay`, default `https://push.slouchcat.com`). */
    val relayAddress: String get() = BuildConfig.PUSH_RELAY

    /** A client without the session's credentials: the relay never sees the server's token. */
    private val relayHttp: OkHttpClient by lazy { OkHttpClient.Builder().callTimeout(Duration.ofSeconds(RELAY_TIMEOUT_S)).build() }

    /** The real registrar of [graph]: the relay of this build, Google's token, the store sealed by the Keystore. */
    fun create(context: Context, graph: AppGraph): PushRegistrar {
        val relay = PushRelay(relayAddress.toHttpUrl(), relayHttp)
        return PushRegistrar(
            session = graph.session,
            channels = graph.media.channels,
            relay = { token, vapid -> relay.register(token, vapid) },
            platform = PushPlatforms.create(context),
            store = store(context),
            deviceName = { graph.device.name },
        )
    }

    /** The device's push record (the emulator flow registers through it with a fake Google token). */
    fun store(context: Context): PushStore = FilePushStore(File(context.noBackupFilesDir, "push.bin"), KeystoreSecretBox(STORE_KEY))

    private const val STORE_KEY = "slouchcat-push"
    private const val RELAY_TIMEOUT_S = 20L
}

/**
 * Starts push with the app (concept 27, 7.3): the system's channels, a sync whenever a session is
 * there (start, sign-in, another server), again after the account changed (a new password deletes
 * the app's subscriptions on the server) and on a new Google token; signing out forgets the record
 * (the server deleted the subscription with the session).
 */
object PushStarter {
    fun start(application: Application, graph: AppGraph) {
        // Only a build with push gets the channels of the system (code review L-4).
        if (PushPlatforms.configured) PushNotifier.ensureChannels(application)
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
                .collect { server -> if (server != null) graph.push.sync() }
        }
        // The check in the background (concept 27, 7.3; code review M-4): Android may not start the
        // app for days, and a new VAPID key or a lost subscription must not stay unnoticed.
        if (PushPlatforms.configured) PushCheckWorker.schedule(application)
    }

    /** Google gave a new token: enter the subscription again. */
    fun tokenChanged(graph: AppGraph) {
        graph.scope.launch { graph.push.sync() }
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
            bare.isEmpty() -> graph.registry.home.let { home -> graph.links.request(AppLink(home.key, listOf(home.route))) }
            bare == "/admin" || bare.startsWith("/admin/") -> graph.session.server?.let { server ->
                val url = server.resolve(clean.removePrefix("/")) ?: return
                runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url.toString())).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
            }
            bare.startsWith("/items/") -> graph.links.request("item/" + bare.removePrefix("/items/"))
            bare == "/settings/devices" -> graph.links.request(AppLink("more", listOf(MoreRoute, DevicesRoute)))
            // An address no page of the app knows opens the start.
            else -> if (!graph.links.request(clean.removePrefix("/"))) open(context, graph, "/")
        }
    }
}
