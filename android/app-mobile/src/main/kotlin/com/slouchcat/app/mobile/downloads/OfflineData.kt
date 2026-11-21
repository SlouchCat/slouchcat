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
import com.slouchcat.core.offline.OfflineAccount
import com.slouchcat.core.offline.OfflineEngine
import com.slouchcat.core.offline.OfflineFiles
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
import java.util.WeakHashMap

/**
 * Downloads and offline of this graph (concept 27 section 9): the engine of `:core:offline`, the
 * device settings («nur im WLAN»), the profile chosen offline in the offline library, and how the
 * app reaches the network. Tests put their own in place ([install]).
 */
class OfflineData(
    val engine: OfflineEngine,
    val settings: DeviceSettingsStore,
    /** Whether the device has a network now (the offline banner without waiting for a sync). */
    val network: () -> NetworkKind = { NetworkKind.Unknown },
) {
    /** The profile opened in the offline library with its offline PIN; `null`: the session's profile. */
    val viewer = MutableStateFlow<Long?>(null)

    /** The app opened the downloads by itself because the server did not answer (9.1 «Start ohne Netz»). */
    @Volatile var jumpedOffline: Boolean = false

    companion object {
        private val installed = WeakHashMap<AppGraph, OfflineData>()

        fun of(graph: AppGraph, create: () -> OfflineData): OfflineData = synchronized(installed) { installed.getOrPut(graph, create) }

        fun install(graph: AppGraph, data: OfflineData) = synchronized(installed) { installed[graph] = data }

        /** The account key of the downloads: the server's address and the e-mail address. */
        fun accountKey(server: String, email: String) = "$server|${email.lowercase()}"

        fun session(manager: SessionManager) = OfflineSession {
            when (val s = manager.state.value) {
                is SessionState.Ready -> OfflineAccount(accountKey(s.server.address.toString(), s.email), s.profile.id)
                is SessionState.ChooseProfile -> OfflineAccount(accountKey(s.server.address.toString(), s.email), null)
                else -> null
            }
        }

        /** The folder of the downloads' files: `noBackupFilesDir`, outside every backup and transfer (9.4). */
        fun filesRoot(context: Context) = File(context.noBackupFilesDir, "offline")

        /** The real one: the database and the files in `noBackupFilesDir` (no backup, no device transfer; 9.4). */
        fun create(context: Context, graph: AppGraph, visible: () -> Boolean): OfflineData {
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
            engine.scheduler = AndroidFetchScheduler(app, visible)
            return OfflineData(engine, settings, network::kind).also { data ->
                network.watch { kind ->
                    if (kind == NetworkKind.None) {
                        engine.reportOffline()
                        graph.scope.launch { engine.stopFetching() }
                    } else {
                        graph.scope.launch { data.syncAndFetch(graph, visible()) }
                    }
                }
            }
        }
    }

    /** A sync, then the fetch where Android lets it run; the app follows the contact (offline library and back). */
    suspend fun syncAndFetch(graph: AppGraph, visible: Boolean) {
        val contact = engine.sync()
        follow(graph, contact)
        val pending = engine.pendingFetches()
        when {
            pending.isNotEmpty() && visible -> engine.scheduler.startTransfer()
            pending.isNotEmpty() || engine.waitsForWifiOnly() -> engine.scheduler.scheduleBackground(unmetered = pending.isEmpty())
            engine.accountDownloads().isNotEmpty() -> engine.scheduler.scheduleBackground(unmetered = false)
        }
    }

    /**
     * No answer at the start with downloads of the profile on the device: the app opens «Downloads»
     * by itself (the offline library); once the server answers again it returns to the start (9.1).
     */
    private suspend fun follow(graph: AppGraph, contact: Contact) {
        val ready = graph.session.state.value as? SessionState.Ready ?: return
        if (contact == Contact.Offline && !jumpedOffline) {
            val account = accountKey(ready.server.address.toString(), ready.email)
            val playable = engine.allDownloads().any { it.account == account && it.complete && ready.profile.id in it.allowedProfileIds }
            if (playable) {
                jumpedOffline = true
                graph.links.request(AppLink(DownloadsFeature.TARGET, listOf(DownloadsRoute)))
            }
        } else if (contact == Contact.Online && jumpedOffline) {
            jumpedOffline = false
            val home = graph.registry.home
            graph.links.request(AppLink(home.key, listOf(home.route)))
        }
    }
}

/** The data of this graph; the first use builds the real one. */
val AppGraph.offline: OfflineData
    get() = OfflineData.of(this) { OfflineData.create(appContextForOffline ?: error("offline not started"), this) { OfflineStarter.visible } }

/** The application context, set by [OfflineStarter.start] (the graph keeps its own private). */
private var appContextForOffline: Context? = null

/**
 * Starts downloads and offline with the app (concept 27 9.1, 9.6): the sign-out step (9.7), the
 * hooks of profile changes and PIN entries, and the triggers of the sync: the app comes to the
 * front, every 60 s while it is in front, the network comes back; the periodic background sync
 * runs through WorkManager (about every 6 hours, F-34).
 */
object OfflineStarter {
    private const val TAG = "offline"
    private const val INTERVAL_MS = 60_000L

    @Volatile var visible: Boolean = false
        private set

    private var started = 0
    private var ticker: Job? = null

    fun start(application: Application, graph: AppGraph) {
        appContextForOffline = application.applicationContext
        graph.hooks.addSignOutStep(DownloadsSignOutStep(graph))
        graph.hooks.onProfileOrSessionEnd { graph.offline.viewer.value = null }
        graph.hooks.onPinAccepted { profile, pin -> graph.offline.engine.pinEntered(profile, pin) }
        application.registerActivityLifecycleCallbacks(
            object : Application.ActivityLifecycleCallbacks {
                override fun onActivityStarted(activity: Activity) {
                    started++
                    if (started != 1) return
                    visible = true
                    ticker?.cancel()
                    ticker = graph.scope.launch {
                        while (isActive) {
                            runCatching { graph.offline.syncAndFetch(graph, visible) }.onFailure { Log.w(TAG, "sync failed", error = it) }
                            delay(INTERVAL_MS)
                        }
                    }
                }

                override fun onActivityStopped(activity: Activity) {
                    started = (started - 1).coerceAtLeast(0)
                    if (started == 0) {
                        visible = false
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
}

/**
 * «Abmelden?» asks whether the downloads of the account on this device go (9.7, F-25: yes by
 * default). Yes: they are deleted in the server before the session ends, then on the device.
 */
class DownloadsSignOutStep(private val graph: AppGraph) : SignOutStep {
    override suspend fun question(): SignOutQuestion? {
        val downloads = graph.offline.engine.accountDownloads()
        if (downloads.isEmpty()) return null
        val bytes = downloads.sumOf { it.bytesWritten }
        val resources = (appContextForOffline ?: return null).resources
        val size = android.text.format.Formatter.formatShortFileSize(appContextForOffline, bytes)
        return SignOutQuestion(R.string.app_downloads_signOutQuestion, resources.getQuantityString(R.plurals.app_downloads_signOutSize, downloads.size, downloads.size, size), default = true)
    }

    override suspend fun beforeSignOut(answer: Boolean) = graph.offline.engine.beforeSignOut(deleteAll = answer)
}
