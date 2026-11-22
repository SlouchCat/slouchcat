package com.slouchcat.app.mobile.audio

import android.content.ComponentName
import android.content.Context
import android.os.SystemClock
import androidx.annotation.OptIn
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import com.google.common.util.concurrent.ListenableFuture
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.common.SignOutStep
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.offline.OfflinePlaybackServer
import com.slouchcat.core.player.ApiPlaybackServer
import com.slouchcat.core.player.ApiRadioServer
import com.slouchcat.core.player.AudioMemory
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DataStoreDeviceSettingsStore
import com.slouchcat.core.player.DeviceFactsReader
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.LastAudio
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.MeteredNetwork
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.security.MessageDigest

/**
 * The sound of the app (concept 27 8.3): the one [AudioPlayback] of the graph with its sources (the
 * server, or the downloads of stage 6), and what keeps it going outside a page: the connection to
 * [AudioService] while something is loaded, the title of the station in the notification, the last
 * title for the next process, and the stop at a profile change and the sign-out (8.4). The graph
 * builds it once (`AppGraph.audio`); tests hand their own to the graph's constructor.
 */
class AudioData(
    val playback: AudioPlayback,
    /** Where a title of the server plays from. */
    val online: AudioSource,
    /** Where a downloaded title plays from (`null`: the app has no downloads, a test). */
    private val offline: (() -> AudioSource?)? = null,
) {
    /** The source of a downloaded title for the profile opened now, `null` without one. */
    fun offlineSource(): AudioSource? = offline?.invoke()

    companion object {
        /** The real one: Media3 players, the server of the session, the service, the file of the last title. */
        @OptIn(UnstableApi::class)
        fun create(context: Context, graph: AppGraph): AudioData {
            val app = context.applicationContext
            val store = DataStoreDeviceSettingsStore(app)
            val tablet = app.resources.configuration.smallestScreenWidthDp >= TABLET_DP
            val playback = AudioPlayback(
                scope = graph.scope,
                engines = { kind -> Media3Engine(app, graph.http, withSession = false, kind = kind) },
                radioServer = { ApiRadioServer(graph.api, graph.http) { graph.session.server } },
                capabilities = { narrowing ->
                    withContext(Dispatchers.Default) {
                        CapabilityRules.build(DeviceFactsReader(app).read(if (tablet) DeviceType.Tablet else DeviceType.Phone), store.settings.first().audio, narrowing)
                    }
                },
                settings = store.settings,
                metered = MeteredNetwork.of(app),
                clock = SystemClock::elapsedRealtime,
                memory = FileAudioMemory(File(app.noBackupFilesDir, LAST_FILE)),
                owner = { owner(graph.session) },
            )
            val online = AudioSource("online") { ApiPlaybackServer(graph.api, graph.http) { graph.session.server } }
            val data = AudioData(playback, online) {
                val offline = graph.offline
                val profile = offline.viewer.value ?: (graph.session.state.value as? SessionState.Ready)?.profile?.id ?: return@AudioData null
                AudioSource("${AudioPlayback.OFFLINE}:$profile") {
                    // Plays only while this profile is still the one opened (as the offline video player).
                    OfflinePlaybackServer(offline.engine, { offline.engine.account()?.key?.takeIf { (offline.viewer.value ?: offline.engine.lastAccount.value?.profileId) == profile } }, profile)
                }
            }
            data.attach(graph)
            ServiceConnector(app, playback).start(graph)
            return data
        }

        /**
         * Who listens: the server's key, the account (only a hash of the address) and the profile. The last
         * title of the next process shows only to the same one.
         */
        fun owner(session: SessionManager): String? {
            val ready = session.state.value as? SessionState.Ready ?: return null
            val server = session.serverKey ?: ready.server.address.toString()
            val account = MessageDigest.getInstance("SHA-256").digest(ready.email.lowercase().toByteArray()).joinToString("") { "%02x".format(it) }.take(16)
            return "$server|$account|#${ready.profile.id}"
        }

        private const val TABLET_DP = 600
        private const val LAST_FILE = "audio-last.json"
    }

    /**
     * The hooks of the graph: the sound ends before the sign-out (its last progress still has the
     * token), at every profile change and when the session is lost (8.4); after the death of the
     * process the last title shows paused once the session is ready.
     */
    fun attach(graph: AppGraph) {
        graph.hooks.addSignOutStep(
            object : SignOutStep {
                override suspend fun beforeSignOut(answer: Boolean) = playback.stop()
            },
        )
        graph.hooks.onProfileOrSessionEnd { playback.stop() }
        playback.restoredSource = online
        graph.scope.launch {
            if (graph.session.ready() is SessionState.Ready) playback.restore()
        }
        // The station's current title in the notification and on the lock screen.
        graph.scope.launch {
            playback.now.collect { now ->
                val radio = (now as? AudioNow.Radio)?.state ?: return@collect
                val station = radio.station ?: return@collect
                val entry = radio.now
                val line = when {
                    entry?.artist != null && entry.title != null -> "${entry.artist} – ${entry.title}"
                    else -> entry?.text
                }
                (playback.engine.value as? Media3Engine)?.updateMetadata(station.name, line)
            }
        }
    }
}

/**
 * Binds the app to [AudioService] while something is loaded (a controller of the app keeps the
 * service alive; Media3 makes it a foreground service of the type `mediaPlayback` while it plays).
 * Something is loaded only after a tap of the user in the app, so the service never starts from
 * the background by itself.
 */
private class ServiceConnector(private val context: Context, private val playback: AudioPlayback) {
    private var controller: ListenableFuture<MediaController>? = null

    fun start(graph: AppGraph) {
        graph.scope.launch {
            playback.engine.collect { engine -> if (engine != null) connect() else disconnect() }
        }
    }

    private fun connect() {
        if (controller != null) return
        controller = MediaController.Builder(context, SessionToken(context, ComponentName(context, AudioService::class.java))).buildAsync()
    }

    private fun disconnect() {
        val future = controller ?: return
        controller = null
        MediaController.releaseFuture(future)
    }
}

/** [LastAudio] in a file of the app outside every backup (`noBackupFilesDir`). */
class FileAudioMemory(private val file: File) : AudioMemory {
    @Serializable
    private data class Stored(val itemId: String, val title: String, val byline: String? = null, val owner: String)

    override suspend fun read(): LastAudio? = withContext(Dispatchers.IO) {
        runCatching { Json.decodeFromString<Stored>(file.readText()) }.getOrNull()?.let { LastAudio(it.itemId, it.title, it.byline, it.owner) }
    }

    override suspend fun write(last: LastAudio?) = withContext(Dispatchers.IO) {
        if (last == null) {
            file.delete()
        } else {
            runCatching { file.writeText(Json.encodeToString(Stored.serializer(), Stored(last.itemId, last.title, last.byline, last.owner))) }
                .onFailure { Log.w("audio", "last title not kept", error = it) }
        }
        Unit
    }
}
