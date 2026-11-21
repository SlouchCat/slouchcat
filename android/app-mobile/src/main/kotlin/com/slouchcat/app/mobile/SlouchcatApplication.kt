package com.slouchcat.app.mobile

import android.app.Application
import android.content.Context
import android.os.Build
import coil3.ImageLoader
import coil3.PlatformContext
import coil3.SingletonImageLoader
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import com.slouchcat.app.mobile.common.EntryPlan
import com.slouchcat.app.mobile.common.GateKeyConfirmation
import com.slouchcat.app.mobile.common.KeyConfirmation
import com.slouchcat.app.mobile.common.SessionActions
import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.app.mobile.common.ShellNotices
import com.slouchcat.app.mobile.downloads.OfflineStarter
import com.slouchcat.app.mobile.downloads.offline
import com.slouchcat.app.mobile.shell.AppLinks
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.ShellRegistry
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiServerProver
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.ServerIdentityGate
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
import com.slouchcat.core.media.LibrariesSource
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.offline.background.OfflineHost
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.AndroidTrustInspector
import com.slouchcat.core.session.ApiAuthRepository
import com.slouchcat.core.session.ApiDevicesRepository
import com.slouchcat.core.session.ApiLibrariesRepository
import com.slouchcat.core.session.ApiServerRepository
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DataStoreAccountStore
import com.slouchcat.core.session.DataStoreServerStore
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.DevicesRepository
import com.slouchcat.core.session.EncryptedFileTokenStore
import com.slouchcat.core.session.HealthServerProbe
import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.KeystoreSecretBox
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.NetworkMonitor
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerRepository
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.TokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import java.io.File
import java.time.Duration

/**
 * The dependencies of the app, built by hand once per process (no Hilt, no Koin; build log,
 * Android architecture). A page gets what it needs from here through its ViewModel factory
 * (`viewModel { XViewModel(app.graph.…) }`).
 */
class AppGraph(
    context: Context,
    /** The features of the app (`shell/Features.kt`); tests bring their own. */
    features: List<Feature> = com.slouchcat.app.mobile.shell.features,
    /** The stores of the session; tests bring stores in memory (no Keystore, no DataStore). */
    val serverStore: ServerStore = DataStoreServerStore(context.applicationContext),
    tokenStore: TokenStore = EncryptedFileTokenStore(File(context.applicationContext.noBackupFilesDir, "session.bin"), KeystoreSecretBox()),
    accountStore: AccountStore = DataStoreAccountStore(context.applicationContext),
    /** The libraries of the media choice; tests bring their own. `null`: from the server. */
    librariesRepository: LibrariesRepository? = null,
    /** Sign-in, profiles and account; the showcase brings its own. `null`: from the server. */
    authRepository: AuthRepository? = null,
    /** «Meine Geräte»; the showcase brings its own. `null`: from the server. */
    devicesRepository: DevicesRepository? = null,
    /** The data of the pages (`:core:media`); tests and the showcase bring their own. `null`: from the server. */
    private val mediaData: ((AppGraph) -> MediaData)? = null,
) {
    private val appContext = context.applicationContext

    /** Lives as long as the process; for work that outlasts a page (the session follows the server address). */
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    /** Token and device id in the Keystore, account and profile; the credentials of [http]. */
    val session = SessionManager(serverStore, tokenStore, accountStore)

    /** «Sitzung weg» and «App zu alt» from any call, with the epoch of the call; SessionEventHandler reacts. */
    val sessionEvents = SessionEvents()

    /** Every call of the generated client goes through it: `graph.api.call { … }`. */
    val api = ApiCaller(sessionEvents) { session.epoch.value }

    /** Pooled connections belong to the network they were opened in. */
    private val network = NetworkMonitor { http.connectionPool.evictAll() }

    /**
     * For what is public and comes before any trust: `getHealth` of the setup and `proveServer`.
     * No gate, no credentials, a hard limit (concept 27 section 6).
     */
    private val publicClient: OkHttpClient = SlouchcatHttp.client(identity, builder = OkHttpClient.Builder().callTimeout(PROOF_TIMEOUT))

    /** «Server weist sich aus» in front of every request to a server over HTTP (F-38). */
    val identityGate = ServerIdentityGate(session, network, ApiServerProver(publicClient), timeout = PROOF_TIMEOUT)

    /** The one HTTP client for API, images and player (concept 27, 5.2). */
    val http: OkHttpClient = SlouchcatHttp.client(identity, session, gate = identityGate)

    val serverProbe: ServerProbe = HealthServerProbe(publicClient, AndroidTrustInspector())
    val serverRepository: ServerRepository = ApiServerRepository(api, http)
    val auth: AuthRepository = authRepository ?: ApiAuthRepository(api, http)
    val devices: DevicesRepository = devicesRepository ?: ApiDevicesRepository(api, http)
    val libraries: LibrariesRepository = librariesRepository ?: ApiLibrariesRepository(api, http)
    val hostResolver: HostResolver = HostResolver.System

    /** How this device names itself at the sign-in (concept 27, 4.2). */
    val device: DeviceDescription = describeDevice(appContext)

    /** Profile changes and the end of a session clear what pages remembered (8.4). */
    val hooks = SessionHooks()

    /** Short messages across a new session epoch («Abgemeldet»). */
    val notices = ShellNotices()

    /** Where the next epoch without session starts (the address after «Server wechseln»). */
    val entry = EntryPlan()
    val sessionActions = SessionActions(session, auth, hooks, notices, entry)
    val keyConfirmation: KeyConfirmation = GateKeyConfirmation(identityGate, session)

    /** Images over [http]: the same headers, the same gate (build log, Android architecture). */
    val imageLoader: ImageLoader = ImageLoader.Builder(appContext)
        .components { add(OkHttpNetworkFetcherFactory(callFactory = { http })) }
        .build()

    /** The pages and shell entries of every stage. */
    val registry = ShellRegistry(features)

    /** In-app jumps to a page (stage 8: a notification opens a title). */
    val links = AppLinks(registry)

    /**
     * The repositories and counts of the pages (`:core:media`), the same way for the app and its tests
     * (code review m-6, L-1): built once, the counts forget themselves with the profile.
     */
    val media: MediaData by lazy {
        (mediaData?.invoke(this) ?: MediaData.api(api, http)).also { data -> hooks.onProfileOrSessionEnd { data.clearCounts() } }
    }

    /** The libraries of the active profile for the media choice (loaded only when an entry needs them). */
    val librariesSource = LibrariesSource(session, libraries, scope, enabled = registry.media.isNotEmpty())

    init {
        network.start(appContext)
        // Another server (address or key): no proof of the old one counts (security review H-2).
        session.onServerChanged = { identityGate.reset() }
        session.start(scope)
        hooks.onProfileOrSessionEnd {
            imageLoader.memoryCache?.clear()
            // Files: off the main thread (code review minor 9).
            withContext(Dispatchers.IO) { imageLoader.diskCache?.clear() }
        }
    }

    private companion object {
        val PROOF_TIMEOUT: Duration = Duration.ofSeconds(15)
    }
}

/**
 * The name and kind `appLogin` gets: the model and the Android version («Pixel 9 · Android 16»),
 * `tablet` from a smallest width of 600 dp (the line Android draws between phone and tablet).
 */
fun describeDevice(context: Context): DeviceDescription {
    val model = Build.MODEL?.takeIf { it.isNotBlank() } ?: Build.DEVICE.orEmpty()
    val name = "$model · Android ${Build.VERSION.RELEASE}".take(80)
    val tablet = context.resources.configuration.smallestScreenWidthDp >= TABLET_DP
    return DeviceDescription(name, if (tablet) AppLoginRequest.Kind.TABLET else AppLoginRequest.Kind.PHONE)
}

private const val TABLET_DP = 600

class SlouchcatApplication : Application(), SingletonImageLoader.Factory, OfflineHost {
    lateinit var graph: AppGraph
        private set

    override fun onCreate() {
        super.onCreate()
        Log.sink = AndroidLogSink(BuildConfig.DEBUG)
        graph = AppGraph(this)
        // Downloads and offline (stage 6): the sign-out step, the hooks, the triggers of the sync.
        OfflineStarter.start(this, graph)
    }

    /** The one offline engine for the background fetch and the sync (core/offline). */
    override val offlineEngine get() = graph.offline.engine

    override fun newImageLoader(context: PlatformContext): ImageLoader = graph.imageLoader
}

/**
 * Logcat. The only place of the app that calls android.util.Log (lint rule SlouchcatDirectLog);
 * what arrives here went through the redaction of [Log]. Release builds keep warnings and errors.
 */
class AndroidLogSink(private val verbose: Boolean) : LogSink {
    override fun write(priority: Int, tag: String, message: String) {
        if (!verbose && priority < Log.WARN) return
        android.util.Log.println(priority, "Slouchcat/$tag", message)
    }
}
