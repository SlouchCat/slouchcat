package com.slouchcat.app.tv.player

import android.content.Context
import android.media.AudioFormat
import android.os.SystemClock
import android.view.accessibility.AccessibilityManager
import androidx.annotation.OptIn
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.analytics.AnalyticsListener
import androidx.media3.exoplayer.audio.AudioSink
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.CaptionStyleCompat
import androidx.media3.ui.PlayerView
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import coil3.ImageLoader
import coil3.request.ImageRequest
import coil3.request.SuccessResult
import coil3.request.allowHardware
import coil3.toBitmap
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.PlayerRoute
import com.slouchcat.app.tv.playback.PlaybackSettingsRoute
import com.slouchcat.app.tv.playback.tvDeviceSettings
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.ApiPlaybackServer
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.MeteredNetwork
import com.slouchcat.core.player.PlaybackServer
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.TrickplayTile
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import java.util.Collections
import java.util.WeakHashMap
import kotlin.coroutines.CoroutineContext

/**
 * The video player of stage 3 (concept 26 5.1; mockup `/play/:i`): a page without rail and clock
 * (`fullScreen`). «Abspielen» on the stage of Start, «Weiterschauen», the detail pages and the
 * collections open it through `navigator.opener(PlayerRoute(…))`. The screen stays on while it
 * plays (TV-BU); Home, the screen saver and standby end the session and pause (TV-NP, ON_STOP), it
 * comes back paused. A profile change or a sign-out stops a playback first ([TvGraph.hooks]).
 * [engine], [server], [settings], [facts], [outputs]: tests bring fakes.
 */
@OptIn(UnstableApi::class)
class PlayerFeature(
    private val engine: (Context, TvGraph) -> PlayerEngine = { context, graph -> Media3Engine(context, graph.http) },
    private val server: (TvGraph) -> PlaybackServer = { graph -> ApiPlaybackServer(graph.api, graph.http) { graph.session.server } },
    private val deviceSettings: (Context) -> DeviceSettingsStore = ::tvDeviceSettings,
    /** The TV's decoders, outputs and display; tests bring fixed facts. */
    private val facts: (Context, TvGraph) -> DeviceFacts = { context, graph -> TvFactsReader(context) { graph.device.name }.read() },
    /** A signal when an audio output comes or goes. */
    private val outputs: (Context) -> Flow<Unit> = ::outputChanges,
    /** Where the facts are read (`MediaCodecList` takes a moment); tests stay on the main thread. */
    private val background: CoroutineContext = Dispatchers.Default,
) : TvFeature {
    override val fullScreen = setOf(PlayerRoute::class)

    /** The players of each graph that play now, and the last progress and stops still on their way. */
    private val players = Collections.synchronizedMap(WeakHashMap<TvGraph, Players>())

    private class Players(val finals: Job) {
        val playing: MutableSet<PlayerViewModel> = Collections.synchronizedSet(mutableSetOf())
    }

    private fun players(graph: TvGraph): Players = players.getOrPut(graph) {
        Players(SupervisorJob(graph.scope.coroutineContext[Job])).also { p ->
            // Concept 26 5.7: before the profile changes or the TV signs out, the playback ends at the
            // server (with the token and the profile it began with), then the caches go.
            graph.hooks.onProfileOrSessionEnd {
                p.playing.toList().forEach { it.end() }
                withTimeoutOrNull(STOP_WAIT_MS) { p.finals.children.toList().joinAll() }
            }
        }
    }

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        val own = players(graph)
        composable<PlayerRoute> { entry ->
            val route = entry.toRoute<PlayerRoute>()
            val context = LocalContext.current.applicationContext
            val vm = viewModel {
                val store = deviceSettings(context)
                PlayerViewModel(
                    itemId = route.itemId,
                    startAtMs = route.startAtMs.takeIf { it >= 0 },
                    fromStart = route.fromStart,
                    engine = engine(context, graph),
                    server = server(graph),
                    items = graph.media.items,
                    serverUrl = { graph.session.server },
                    facts = { withContext(background) { facts(context, graph) } },
                    settings = store.settings,
                    outputChanges = outputs(context),
                    metered = MeteredNetwork.of(context),
                    // The last progress and the stop are children of `finals`: a profile change waits for them.
                    finalScope = CoroutineScope(graph.scope.coroutineContext + own.finals),
                    clock = SystemClock::elapsedRealtime,
                    saved = createSavedStateHandle(),
                ).also { own.playing += it }
            }
            DisposableEffect(vm) {
                onDispose { own.playing -= vm }
            }
            vm.leave = navigator::back
            vm.openSoundSettings = { navigator.navigate(PlaybackSettingsRoute) }
            val state by vm.state.collectAsStateWithLifecycle()
            val sound by vm.sound.collectAsStateWithLifecycle()
            val seasons by vm.seasons.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_STOP) { vm.onBackground() }
            LifecycleEventEffect(Lifecycle.Event.ON_START) { vm.onForeground() }
            KeepScreenOn(state.playing)
            val address = graph.session.server
            val images = remember(address) { address?.let(::ServerImages) }
            val media3 = vm.engine as? Media3Engine
            if (media3 != null) AudioOutputWatch(media3, vm)
            PlayerScreen(
                state = state,
                sound = sound,
                seasons = seasons,
                events = vm,
                images = images,
                remoteKeys = graph.remoteKeys,
                autoHide = !talkBack(context),
                surface = { if (media3 != null) VideoSurface(media3) },
                thumbnail = { tile, modifier -> TrickplayThumbnail(tile, graph.imageLoader, modifier) },
            )
        }
    }

    private companion object {
        /** How long a profile change waits for the last progress and the stop of a playback. */
        const val STOP_WAIT_MS = 3_000L
    }
}

/** No screen saver while it plays (TV-BU); while it stands, the TV may dim and sleep. */
@Composable
private fun KeepScreenOn(playing: Boolean) {
    val view = LocalView.current
    DisposableEffect(view, playing) {
        view.keepScreenOn = playing
        onDispose { view.keepScreenOn = false }
    }
}

/** With TalkBack the controls stay: hidden ones would leave nothing to reach (review M-3 of K27 stage 5). */
private fun talkBack(context: Context): Boolean =
    context.getSystemService(AccessibilityManager::class.java)?.let { it.isEnabled && it.isTouchExplorationEnabled } == true

/**
 * Tells the page what Media3 does with the sound (K26 stage 0 point 2): an encoded output (AC-3,
 * E-AC-3, DTS, TrueHD) goes to the sound system as it is, PCM is decoded on the TV.
 */
@OptIn(UnstableApi::class)
@Composable
private fun AudioOutputWatch(engine: Media3Engine, vm: PlayerViewModel) {
    DisposableEffect(engine, vm) {
        val listener = object : AnalyticsListener {
            override fun onAudioTrackInitialized(eventTime: AnalyticsListener.EventTime, audioTrackConfig: AudioSink.AudioTrackConfig) {
                vm.onAudioOutput(audioTrackConfig.encoding in ENCODED)
            }
        }
        engine.player.addAnalyticsListener(listener)
        onDispose { engine.player.removeAnalyticsListener(listener) }
    }
}

private val ENCODED = setOf(
    AudioFormat.ENCODING_AC3,
    AudioFormat.ENCODING_E_AC3,
    AudioFormat.ENCODING_E_AC3_JOC,
    AudioFormat.ENCODING_DTS,
    AudioFormat.ENCODING_DTS_HD,
    AudioFormat.ENCODING_DOLBY_TRUEHD,
)

/**
 * Media3's view without its own controls: the picture on a SurfaceView (the system switches the
 * TV into HDR for an HDR stream itself; concept 26 5.5, F-8: no switch of the frame rate) and
 * the subtitles: outlined text (K26 stage 0 point 3), the styles of ASS and the pictures of PGS as
 * the file has them.
 */
@OptIn(UnstableApi::class)
@Composable
private fun VideoSurface(engine: Media3Engine) {
    val c = SlouchcatTheme.colors
    val text = c.tx.toArgb()
    val edge = c.bg.toArgb()
    val none = Color.Transparent.toArgb()
    AndroidView(
        factory = { context ->
            PlayerView(context).apply {
                useController = false
                isFocusable = false
                resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
                setShutterBackgroundColor(edge)
                subtitleView?.apply {
                    setApplyEmbeddedStyles(true)
                    setApplyEmbeddedFontSizes(true)
                    setStyle(CaptionStyleCompat(text, none, none, CaptionStyleCompat.EDGE_TYPE_OUTLINE, edge, null))
                }
                player = engine.player
            }
        },
        onRelease = { it.player = null },
        modifier = Modifier.fillMaxSize(),
    )
}

/** One trickplay tile: the sheet through the app's image loader (the one HTTP client), the tile cut out. */
@Composable
private fun TrickplayThumbnail(tile: TrickplayTile, loader: ImageLoader, modifier: Modifier) {
    val context = LocalContext.current
    val sheet by produceState<ImageBitmap?>(null, tile.sheetUrl) {
        val request = ImageRequest.Builder(context).data(tile.sheetUrl).allowHardware(false).build()
        value = (loader.execute(request) as? SuccessResult)?.image?.toBitmap()?.asImageBitmap()
    }
    Canvas(modifier) {
        val image = sheet ?: return@Canvas
        drawImage(
            image,
            srcOffset = IntOffset(tile.x, tile.y),
            srcSize = IntSize(minOf(tile.width, image.width - tile.x), minOf(tile.height, image.height - tile.y)),
            dstSize = IntSize(size.width.toInt(), size.height.toInt()),
        )
    }
}
