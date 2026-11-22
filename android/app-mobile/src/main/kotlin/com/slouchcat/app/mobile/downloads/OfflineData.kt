package com.slouchcat.app.mobile.downloads

import android.app.Activity
import android.app.Application
import android.content.Context
import android.os.Bundle
import android.os.SystemClock
import android.provider.Settings
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.common.SignOutQuestion
import com.slouchcat.app.mobile.common.SignOutStep
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.offline.ApiOfflineServer
import com.slouchcat.core.offline.Contact
import com.slouchcat.core.offline.FileFetcher
import com.slouchcat.core.offline.NetworkKind
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.offline.OfflineFiles
import com.slouchcat.core.offline.OfflineIdentity
import com.slouchcat.core.offline.OfflineSession
import com.slouchcat.core.offline.OfflineStore
import com.slouchcat.core.offline.background.AndroidFetchScheduler
import com.slouchcat.core.offline.background.AndroidNetwork
import com.slouchcat.core.player.DataStoreDeviceSettingsStore
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File

/**
 * Downloads and offline of a graph (concept 27 section 9): the engine of `:core:offline`, the device
 * settings («nur im WLAN»), the profile opened offline with its PIN, whether the app is in front, and
 * how the app reaches the network. The graph builds it once (`AppGraph.offline`); tests hand their
 * own to the graph's constructor (code review m-3).
 */
class OfflineData(
    val engine: OfflineEngine,
    val settings: DeviceSettingsStore,
    /** Whether the device has a network now (the offline banner without waiting for a sync). */
    val network: () -> NetworkKind = { NetworkKind.Unknown },
) {
    /** The profile opened in the offline library with its offline PIN; `null`: the session's profile. */
    val viewer = MutableStateFlow<Long?>(null)

    /** An activity of the app is started (the system lets a transfer start only then). */
    @Volatile var visible: Boolean = false

    /** The app opened «Downloads» by itself at the start because the server did not answer. */
    @Volatile var jumpedOffline: Boolean = false

    /** The epoch whose start already decided about the offline library (once per epoch, code review M-3). */
    @Volatile private var decidedEpoch: Long = -1

    companion object {
        /** The folder of the downloads' files: `noBackupFilesDir`, outside every backup and transfer (9.4). */
        fun filesRoot(context: Context) = File(context.noBackupFilesDir, "offline")

        /**
         * The session for the engine: waits until the stores are read (a cold process woken by the
         * system, code review B-2), then the server's key (its address without one), the e-mail address
         * and the active profile.
         */
        fun session(manager: SessionManager) = OfflineSession {
            val state = manager.ready()
            val server = manager.serverKey ?: manager.server?.toString()
            when {
                server == null -> null
                state is SessionState.Ready -> OfflineIdentity(server, state.email, state.profile.id)
                state is SessionState.ChooseProfile -> OfflineIdentity(server, state.email, null)
                else -> null
            }
        }

        /** The real one: the database and the files in `noBackupFilesDir` (no backup, no device transfer; 9.4). */
        fun create(context: Context, graph: AppGraph): OfflineData {
            val app = context.applicationContext
            val settings = DataStoreDeviceSettingsStore(app)
            val network = AndroidNetwork(app)
            val files = OfflineFiles(filesRoot(app))
            val engine = OfflineEngine(
                store = OfflineStore.open(app),
                files = files,
                server = ApiOfflineServer(graph.api, graph.http) { graph.session.server },
                fetcher = FileFetcher(graph.http, files::freeBytes),
                session = session(graph.session),
                network = network::kind,
                wifiOnly = { settings.settings.first().downloadsWifiOnly },
                scope = graph.scope,
                elapsed = SystemClock::elapsedRealtime,
                boot = { Settings.Global.getInt(app.contentResolver, Settings.Global.BOOT_COUNT, -1) },
            )
            val data = OfflineData(engine, settings, network::kind)
            engine.scheduler = AndroidFetchScheduler(app) { data.visible }
            var pending: Job? = null
            network.watch { kind ->
                pending?.cancel()
                if (kind == NetworkKind.None) {
                    engine.reportOffline()
                    graph.scope.launch { engine.stopFetching() }
                } else {
                    // Settle first: a network that comes and goes in a lift syncs once (code review M-6).
                    pending = graph.scope.launch {
                        delay(NETWORK_SETTLE_MS)
                        runCatching { data.syncAndFetch(graph) }.onFailure { Log.w("offline", "sync failed", error = it) }
                    }
                }
            }
            return data
        }

        private const val NETWORK_SETTLE_MS = 2_000L
        private const val RETRY_AT_START_MS = 3_000L
    }

    /** A sync, then the fetch where Android lets it run; at the start of an epoch the offline library when the server is away. */
    suspend fun syncAndFetch(graph: AppGraph, atStart: Boolean = false) {
        var contact = engine.sync()
        // Offline counts after two failures (the engine); at the start the second comes soon.
        if (atStart && contact == Contact.Offline && engine.contact.value != Contact.Offline) {
            delay(RETRY_AT_START_MS)
            contact = engine.sync()
        }
        follow(graph, atStart)
        val pending = engine.pendingFetches()
        when {
            pending.isNotEmpty() && visible -> engine.scheduler.startTransfer()
            pending.isNotEmpty() || engine.waitsForWifiOnly() -> engine.scheduler.scheduleBackground(unmetered = pending.isEmpty())
            engine.accountDownloads().isNotEmpty() -> engine.scheduler.scheduleBackground(unmetered = false)
        }
    }

    /**
     * Without the server at the start of an epoch, with a download of the profile on the device: the
     * app opens «Downloads» by itself (9.1, ADR 0043 point 8). Only then, never later in the middle of
     * what the user does; when the server answers again the user stays where they are and the shell
     * says so (code review M-3).
     */
    private suspend fun follow(graph: AppGraph, atStart: Boolean) {
        val ready = graph.session.state.value as? SessionState.Ready ?: return
        val epoch = graph.session.epoch.value
        val contact = engine.contact.value
        if (atStart && decidedEpoch != epoch && contact != Contact.Unknown) {
            decidedEpoch = epoch
            if (contact == Contact.Offline) {
                val account = engine.account()?.key ?: return
                val playable = engine.allDownloads().any { it.account == account && it.complete && ready.profile.id in it.allowedProfileIds }
                if (playable) {
                    jumpedOffline = true
                    graph.links.request(AppLink(DownloadsFeature.TARGET, listOf(DownloadsRoute)))
                }
            }
        } else if (contact == Contact.Online && jumpedOffline) {
            jumpedOffline = false
            graph.notices.show(R.string.app_downloads_backOnline)
        }
    }
}

/**
 * Starts downloads and offline with the app (concept 27 9.1, 9.6): the sign-out step (9.7), the
 * hooks of profile changes and PIN entries, and the triggers of the sync: the app comes to the
 * front, every 60 s while it is in front, the network comes back; the periodic background sync
 * runs through WorkManager (about every 6 hours, F-34).
 */
class OfflineStarter(private val application: Application, private val graph: AppGraph) {
    private var started = 0
    private var ticker: Job? = null
    private var lastEpoch = -1L

    fun start() {
        graph.hooks.addSignOutStep(DownloadsSignOutStep(graph, application))
        graph.hooks.onProfileOrSessionEnd { graph.offline.viewer.value = null }
        // The verifier of the normal PIN in the background: the profile choice does not wait for PBKDF2 (code review m-7).
        graph.hooks.onPinAccepted { profile, pin ->
            val copy = pin.copyOf()
            graph.scope.launch {
                try {
                    graph.offline.engine.pinEntered(profile, copy)
                } finally {
                    copy.fill('0')
                }
            }
        }
        application.registerActivityLifecycleCallbacks(
            object : Application.ActivityLifecycleCallbacks {
                override fun onActivityStarted(activity: Activity) {
                    started++
                    if (started != 1) return
                    graph.offline.visible = true
                    ticker?.cancel()
                    ticker = graph.scope.launch {
                        while (isActive) {
                            val epoch = graph.session.epoch.value
                            val atStart = epoch != lastEpoch
                            lastEpoch = epoch
                            runCatching { graph.offline.syncAndFetch(graph, atStart) }.onFailure { Log.w(TAG, "sync failed", error = it) }
                            delay(INTERVAL_MS)
                        }
                    }
                }

                override fun onActivityStopped(activity: Activity) {
                    started = (started - 1).coerceAtLeast(0)
                    if (started == 0) {
                        graph.offline.visible = false
                        ticker?.cancel()
                    }
                }

                override fun onActivityCreated(activity: Activity, savedInstanceState: Bundle?) = Unit

                override fun onActivityResumed(activity: Activity) = Unit

                override fun onActivityPaused(activity: Activity) = Unit

                override fun onActivitySaveInstanceState(activity: Activity, outState: Bundle) = Unit

                override fun onActivityDestroyed(activity: Activity) = Unit
            },
        )
    }

    private companion object {
        const val TAG = "offline"
        const val INTERVAL_MS = 60_000L
    }
}

/**
 * «Abmelden?» asks whether the downloads of the account on this device go (9.7, F-25: yes by
 * default). Yes: they are deleted on the device and in the server; what the server does not
 * confirm goes out at the next sign-in of the account.
 */
class DownloadsSignOutStep(private val graph: AppGraph, private val context: Context) : SignOutStep {
    override suspend fun question(): SignOutQuestion? {
        val downloads = graph.offline.engine.accountDownloads()
        if (downloads.isEmpty()) return null
        val bytes = downloads.sumOf { it.bytesWritten }
        val resources = context.resources
        val size = formatBytes(bytes, resources.configuration.locales[0])
        return SignOutQuestion(R.string.app_downloads_signOutQuestion, resources.getQuantityString(R.plurals.app_downloads_signOutSize, downloads.size, downloads.size, size), default = true)
    }

    override suspend fun beforeSignOut(answer: Boolean) = graph.offline.engine.beforeSignOut(deleteAll = answer)
}
