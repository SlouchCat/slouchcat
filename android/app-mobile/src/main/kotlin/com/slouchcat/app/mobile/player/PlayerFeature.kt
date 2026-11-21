package com.slouchcat.app.mobile.player

import android.content.Context
import android.content.pm.ActivityInfo
import android.os.SystemClock
import androidx.activity.compose.LocalActivity
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
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.annotation.OptIn
import androidx.media3.common.util.UnstableApi
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
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.core.media.ServerImages
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.player.ApiPlaybackServer
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DataStoreDeviceSettingsStore
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceFactsReader
import com.slouchcat.core.player.DeviceSettingsStore
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.MeteredNetwork
import com.slouchcat.core.player.PlaybackServer
import com.slouchcat.core.player.PlayerEngine
import com.slouchcat.core.player.TrickplayTile
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.withContext
import kotlin.coroutines.CoroutineContext

/**
 * The video player of stage 5 (concept 27 8.1, mockup `/play/:i`): a page without the shell
 * (`fullScreen`), landscape on a phone, the screen stays on while it plays, the system bars hidden.
 * «Abspielen», «Weiterschauen», «Von vorn» and the episode rows of the detail pages, and
 * «Abspielen» of a collection, hook in through `navigator.opener(PlayerRoute(…))`. Leaving the page
 * ends the playback (last progress, `stopPlayback`); the detail page reads its progress again.
 * [engine], [server] and [settings]: tests bring fakes.
 */
@OptIn(UnstableApi::class)
class PlayerFeature(
    private val engine: (Context, AppGraph) -> PlayerEngine = { context, graph -> Media3Engine(context, graph.http) },
    private val server: (AppGraph) -> PlaybackServer = { graph -> ApiPlaybackServer(graph.api, graph.http) { graph.session.server } },
    private val settings: (Context) -> DeviceSettingsStore = { DataStoreDeviceSettingsStore(it) },
    /** The device's decoders, outputs and display; tests bring fixed facts. */
    private val facts: (Context, DeviceType) -> DeviceFacts = { context, type -> DeviceFactsReader(context).read(type) },
    /** Where the facts are read (`MediaCodecList` takes a moment); tests stay on the main thread. */
    private val background: CoroutineContext = Dispatchers.Default,
) : Feature {
    override val fullScreen = setOf(PlayerRoute::class)

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        composable<PlayerRoute> { entry ->
            val route = entry.toRoute<PlayerRoute>()
            val context = LocalContext.current.applicationContext
            val tablet = LocalConfiguration.current.smallestScreenWidthDp >= TABLET_DP
            val vm = viewModel {
                val store = settings(context)
                PlayerViewModel(
                    itemId = route.itemId,
                    startAtMs = route.startAtMs.takeIf { it >= 0 },
                    fromStart = route.fromStart,
                    engine = engine(context, graph),
                    server = server(graph),
                    capabilities = { narrowing ->
                        withContext(background) {
                            CapabilityRules.build(facts(context, if (tablet) DeviceType.Tablet else DeviceType.Phone), store.settings.first().audio, narrowing)
                        }
                    },
                    settings = store.settings,
                    metered = MeteredNetwork.of(context),
                    finalScope = graph.scope,
                    clock = SystemClock::elapsedRealtime,
                    saved = createSavedStateHandle(),
                )
            }
            vm.leave = app.navigator::back
            val state by vm.state.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_STOP) { vm.onBackground() }
            LifecycleEventEffect(Lifecycle.Event.ON_START) { vm.onForeground() }
            PlayerWindow(phone = !tablet, playing = state.playing)
            val address = graph.session.server
            val images = remember(address) { address?.let(::ServerImages) }
            val media3 = vm.engine as? Media3Engine
            PlayerScreen(
                state = state,
                events = vm,
                images = images,
                surface = { if (media3 != null) VideoSurface(media3) },
                thumbnail = { tile, modifier -> TrickplayThumbnail(tile, graph.imageLoader, modifier) },
            )
        }
    }

    private companion object {
        const val TABLET_DP = 600
    }
}

/**
 * The window of the player: landscape on a phone (concept 27 8.1), the system bars hidden (a swipe
 * shows them for a moment), the screen on while it plays. Everything goes back when the page goes.
 */
@Composable
private fun PlayerWindow(phone: Boolean, playing: Boolean) {
    val activity = LocalActivity.current ?: return
    val view = LocalView.current
    DisposableEffect(activity, phone) {
        val before = activity.requestedOrientation
        if (phone) activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
        val bars = WindowCompat.getInsetsController(activity.window, view)
        bars.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        bars.hide(WindowInsetsCompat.Type.systemBars())
        onDispose {
            bars.show(WindowInsetsCompat.Type.systemBars())
            activity.requestedOrientation = before
        }
    }
    DisposableEffect(view, playing) {
        view.keepScreenOn = playing
        onDispose { view.keepScreenOn = false }
    }
}

/** Media3's view without its own controls: the picture and the subtitles (outlined, K26 stage 0 point 3). */
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
                resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
                setShutterBackgroundColor(edge)
                subtitleView?.setStyle(CaptionStyleCompat(text, none, none, CaptionStyleCompat.EDGE_TYPE_OUTLINE, edge, null))
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
            srcSize = IntSize(tile.width, tile.height).let { IntSize(minOf(it.width, image.width - tile.x), minOf(it.height, image.height - tile.y)) },
            dstSize = IntSize(size.width.toInt(), size.height.toInt()),
        )
    }
}
