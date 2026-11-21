package com.slouchcat.app.tv

import android.app.Application
import android.content.Context
import android.os.Build
import coil3.ImageLoader
import coil3.PlatformContext
import coil3.SingletonImageLoader
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import com.slouchcat.app.tv.common.DataStoreRelaySettings
import com.slouchcat.app.tv.common.Loadable
import com.slouchcat.app.tv.common.RelaySettings
import com.slouchcat.app.tv.common.TvSessionActions
import com.slouchcat.app.tv.common.TvSessionHooks
import com.slouchcat.app.tv.common.toLoadable
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvRegistry
import com.slouchcat.app.tv.ui.TvToasts
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiServerProver
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.ServerIdentityGate
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.TrustedServer
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
import com.slouchcat.core.pairing.ApiRedeemer
import com.slouchcat.core.pairing.Redeemer
import com.slouchcat.core.pairing.RelayClient
import com.slouchcat.core.pairing.TvPairing
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.AndroidTrustInspector
import com.slouchcat.core.session.ApiAuthRepository
import com.slouchcat.core.session.ApiLibrariesRepository
import com.slouchcat.core.session.AuthRepository
import com.slouchcat.core.session.DataStoreAccountStore
import com.slouchcat.core.session.DataStoreServerStore
import com.slouchcat.core.session.DeviceDescription
import com.slouchcat.core.session.EncryptedFileTokenStore
import com.slouchcat.core.session.HealthServerProbe
import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.KeystoreSecretBox
import com.slouchcat.core.session.LibrariesRepository
import com.slouchcat.core.session.NetworkMonitor
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.TokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl
import okhttp3.OkHttpClient
import java.io.File
import java.time.Duration

/**
 * The dependencies of the TV app, built by hand once per process (build log, Android architecture:
 * no Hilt, no Koin). A page gets what it needs from here through its ViewModel factory
 * (`viewModel { XViewModel(app.graph.…) }`).
 */
class TvGraph(
    context: Context,
    /** The features of the app (`shell/Features.kt`); tests bring their own. */
    features: List<TvFeature> = com.slouchcat.app.tv.shell.features,
    /** The stores of the session; tests bring stores in memory (no Keystore, no DataStore). */
    val serverStore: ServerStore = DataStoreServerStore(context.applicationContext),
    tokenStore: TokenStore = EncryptedFileTokenStore(File(context.applicationContext.noBackupFilesDir, "session.bin"), KeystoreSecretBox()),
    accountStore: AccountStore = DataStoreAccountStore(context.applicationContext),
    /** The address of the relay (F-21); tests bring one in memory. */
    val relaySettings: RelaySettings = DataStoreRelaySettings(context.applicationContext),
    /** Seams for tests: `null` takes the real one. */
    serverProbe: ServerProbe? = null,
    auth: AuthRepository? = null,
    redeemer: Redeemer? = null,
    librariesRepository: LibrariesRepository? = null,
) {
    private val appContext = context.applicationContext

    /** Lives as long as the process; for work that outlasts a page. */
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    val identity = ClientIdentity(ClientIdentity.Kind.TV, BuildConfig.VERSION_NAME)

    /** Token and device id in the Keystore, account and profile; the credentials of [http]. */
    val session = SessionManager(serverStore, tokenStore, accountStore)

    /** «Sitzung weg» and «App zu alt» from any call; the NavHost reacts. */
    val sessionEvents = SessionEvents()

    /** Every call of the generated client goes through it: `graph.api.call { … }`. */
    val api = ApiCaller(sessionEvents) { session.epoch.value }

    private val network = NetworkMonitor { http.connectionPool.evictAll() }

    /** For what is public and comes before any trust: `getHealth`, `proveServer`, the relay. */
    val publicClient: OkHttpClient = SlouchcatHttp.client(identity, builder = OkHttpClient.Builder().callTimeout(PROOF_TIMEOUT))

    /** «Server weist sich aus» in front of every request to a server over HTTP (F-38); the TV's key comes with the packet. */
    val identityGate = ServerIdentityGate(session, network, ApiServerProver(publicClient), timeout = PROOF_TIMEOUT)

    /** The one HTTP client for API, images and (stage 3) player. */
    val http: OkHttpClient = SlouchcatHttp.client(identity, session, gate = identityGate)

    val serverProbe: ServerProbe = serverProbe ?: HealthServerProbe(publicClient, AndroidTrustInspector())
    val auth: AuthRepository = auth ?: ApiAuthRepository(api, http)
    val libraries: LibrariesRepository = librariesRepository ?: ApiLibrariesRepository(api, http)
    val hostResolver: HostResolver = HostResolver.System

    /**
     * The redeem of a pairing: a client of its own for the server of the packet, without the
     * session's credentials, with «Server weist sich aus» against the packet's key.
     */
    val redeemer: Redeemer = redeemer ?: ApiRedeemer(api) { server, key ->
        val trusted = object : TrustedServer {
            override val server: HttpUrl = server
            override val serverKey: String = key
        }
        SlouchcatHttp.client(identity, gate = ServerIdentityGate(trusted, network, ApiServerProver(publicClient), timeout = PROOF_TIMEOUT))
    }

    /** A pairing against the relay in use (read when the QR code page opens). */
    fun pairing(relay: HttpUrl): TvPairing = TvPairing(RelayClient(publicClient, relay), redeemer)

    /** How this TV names itself to the server («Meine Geräte», the message `device_linked`). */
    val device: DeviceDescription = describeTv()

    /** The system version the redeem sends (`Android TV 14`). */
    val systemVersion: String = "Android TV ${Build.VERSION.RELEASE}"

    val hooks = TvSessionHooks()
    val sessionActions = TvSessionActions(session, serverStore, this.auth, hooks)

    /** Short messages of the app (`toast()`), also across a new session epoch. */
    val toasts = TvToasts()

    /** Images over [http]: the same headers, the same gate. */
    val imageLoader: ImageLoader = ImageLoader.Builder(appContext)
        .components { add(OkHttpNetworkFetcherFactory(callFactory = { http })) }
        .build()

    /** The pages, rail entries and settings rows of every stage. */
    val registry = TvRegistry(features)

    /** The libraries of the active profile for the rail (loaded only when a feature brings media entries). */
    val librariesSource = TvLibrariesSource(session, libraries, registry, scope)

    init {
        network.start(appContext)
        session.onServerChanged = { identityGate.reset() }
        session.start(scope)
        hooks.onProfileOrSessionEnd {
            imageLoader.memoryCache?.clear()
            withContext(Dispatchers.IO) { imageLoader.diskCache?.clear() }
        }
    }

    private companion object {
        val PROOF_TIMEOUT: Duration = Duration.ofSeconds(15)
    }
}

/** «Modell · Android TV 14», at most 80 characters, kind `tv` (concept 26, 4.2). */
fun describeTv(): DeviceDescription {
    val model = Build.MODEL?.takeIf { it.isNotBlank() } ?: Build.DEVICE.orEmpty()
    return DeviceDescription("$model · Android TV ${Build.VERSION.RELEASE}".take(DEVICE_NAME_MAX), AppLoginRequest.Kind.TV)
}

private const val DEVICE_NAME_MAX = 80

/**
 * The libraries of the active profile (`listVisibleLibraries`) for the rail's media entries, the
 * same rules as on the phone: only when a feature brings media entries, again with every epoch
 * that has a profile, empty without one.
 */
class TvLibrariesSource(
    private val session: SessionManager,
    private val repository: LibrariesRepository,
    private val registry: TvRegistry,
    private val scope: CoroutineScope,
) {
    private val _libraries = MutableStateFlow<Loadable<List<LibraryNav>>>(Loadable.Empty)
    val libraries: StateFlow<Loadable<List<LibraryNav>>> = _libraries.asStateFlow()
    private var loading: Job? = null

    init {
        if (registry.media.isNotEmpty()) {
            scope.launch {
                combine(session.epoch, session.state) { epoch, state -> epoch to (state as? SessionState.Ready)?.profile?.id }
                    .distinctUntilChanged()
                    .collect { (_, profile) -> if (profile == null) clear() else reload() }
            }
        }
    }

    fun reload() {
        val server = session.server ?: return
        loading?.cancel()
        if (_libraries.value !is Loadable.Ready) _libraries.value = Loadable.Loading
        loading = scope.launch {
            val result = repository.libraries(server).toLoadable { it.isEmpty() }
            if (result is Loadable.Failed && _libraries.value is Loadable.Ready) return@launch
            _libraries.value = result
        }
    }

    fun clear() {
        loading?.cancel()
        _libraries.value = Loadable.Empty
    }
}

class TvApplication : Application(), SingletonImageLoader.Factory {
    lateinit var graph: TvGraph
        private set

    override fun onCreate() {
        super.onCreate()
        Log.sink = AndroidLogSink(BuildConfig.DEBUG)
        graph = TvGraph(this)
    }

    override fun newImageLoader(context: PlatformContext): ImageLoader = graph.imageLoader
}

/** Logcat; the only place of the app that calls android.util.Log (lint rule SlouchcatDirectLog). */
class AndroidLogSink(private val verbose: Boolean) : LogSink {
    override fun write(priority: Int, tag: String, message: String) {
        if (!verbose && priority < Log.WARN) return
        android.util.Log.println(priority, "SlouchcatTv/$tag", message)
    }
}
