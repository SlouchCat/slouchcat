package com.slouchcat.app.mobile

import android.app.Application
import android.content.Context
import android.os.Build
import coil3.ImageLoader
import coil3.PlatformContext
import coil3.SingletonImageLoader
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import com.slouchcat.app.mobile.common.GateKeyConfirmation
import com.slouchcat.app.mobile.common.KeyConfirmation
import com.slouchcat.app.mobile.common.SessionActions
import com.slouchcat.app.mobile.common.SessionHooks
import com.slouchcat.app.mobile.shell.LibrariesSource
import com.slouchcat.app.mobile.shell.ShellRegistry
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiServerProver
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.ServerIdentityGate
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
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
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.OkHttpClient
import java.io.File
import java.time.Duration

/**
 * The dependencies of the app, built by hand once per process (no Hilt, no Koin; build log,
 * Android architecture). A page gets what it needs from here through its ViewModel factory
 * (`viewModel { XViewModel(app.graph.…) }`).
 */
class AppGraph(context: Context) {
    private val appContext = context.applicationContext

    /** Lives as long as the process; for work that outlasts a page (the session follows the server address). */
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    /** «Sitzung weg», «App zu alt», «neuer Schlüssel» from any call; SessionEventHandler reacts. */
    val sessionEvents = SessionEvents()

    /** Every call of the generated client goes through it: `graph.api.call { … }`. */
    val api = ApiCaller(sessionEvents)

    val serverStore: ServerStore = DataStoreServerStore(appContext)

    /** Token and device id in the Keystore, account and profile; the credentials of [http]. */
    val session = SessionManager(
        serverStore,
        EncryptedFileTokenStore(File(appContext.noBackupFilesDir, "session.bin"), KeystoreSecretBox()),
        DataStoreAccountStore(appContext),
    )

    /** Pooled connections belong to the network they were opened in. */
    private val network = NetworkMonitor { http.connectionPool.evictAll() }

    /** Only for `proveServer`: no gate, no credentials, a hard limit (concept 27 section 6). */
    private val proofClient: OkHttpClient = SlouchcatHttp.client(identity, builder = OkHttpClient.Builder().callTimeout(PROOF_TIMEOUT))

    /** «Server weist sich aus» in front of every request to a server over HTTP (F-38). */
    val identityGate = ServerIdentityGate(session, network, ApiServerProver(proofClient), sessionEvents, timeout = PROOF_TIMEOUT)

    /** The one HTTP client for API, images and player (concept 27, 5.2). */
    val http: OkHttpClient = SlouchcatHttp.client(identity, session, gate = identityGate)

    val serverProbe: ServerProbe = HealthServerProbe(http, AndroidTrustInspector())
    val serverRepository: ServerRepository = ApiServerRepository(api, http)
    val auth: AuthRepository = ApiAuthRepository(api, http)
    val devices: DevicesRepository = ApiDevicesRepository(api, http)
    val libraries: LibrariesRepository = ApiLibrariesRepository(api, http)
    val hostResolver: HostResolver = HostResolver.System

    /** How this device names itself at the sign-in (concept 27, 4.2). */
    val device: DeviceDescription = describeDevice(appContext)

    /** Profile changes and the end of a session clear what pages remembered (8.4). */
    val hooks = SessionHooks()
    val sessionActions = SessionActions(session, auth, hooks)
    val keyConfirmation: KeyConfirmation = GateKeyConfirmation(identityGate, session, serverStore)

    /** Images over [http]: the same headers, the same gate (build log, Android architecture). */
    val imageLoader: ImageLoader = ImageLoader.Builder(appContext)
        .components { add(OkHttpNetworkFetcherFactory(callFactory = { http })) }
        .build()

    /** The pages and shell entries of every stage (`shell/Features.kt`). */
    val registry = ShellRegistry(features)

    /** The libraries of the active profile for the media choice (loaded only when an entry needs them). */
    val librariesSource = LibrariesSource(session, libraries, registry, scope)

    init {
        network.start(appContext)
        session.start(scope)
        hooks.onProfileOrSessionEnd {
            imageLoader.memoryCache?.clear()
            imageLoader.diskCache?.clear()
            librariesSource.clear()
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

class SlouchcatApplication : Application(), SingletonImageLoader.Factory {
    lateinit var graph: AppGraph
        private set

    override fun onCreate() {
        super.onCreate()
        Log.sink = AndroidLogSink(BuildConfig.DEBUG)
        graph = AppGraph(this)
    }

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
