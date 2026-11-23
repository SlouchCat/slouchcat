package com.slouchcat.app.tv.audio

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.media.AudioManager
import android.os.SystemClock
import coil3.request.allowHardware
import coil3.toBitmap
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import androidx.navigation.NavDestination.Companion.hasRoute
import com.slouchcat.app.tv.ui.OnRemoteKey
import com.slouchcat.app.tv.ui.RemoteKey
import androidx.annotation.OptIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.media3.common.util.UnstableApi
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.AudioSeriesRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.ListenRoute
import com.slouchcat.app.tv.browse.PlayerRoute
import com.slouchcat.app.tv.browse.RadioListenRoute
import com.slouchcat.app.tv.browse.RadioRoute
import com.slouchcat.app.tv.playback.tvDeviceSettings
import com.slouchcat.app.tv.player.TvFactsReader
import com.slouchcat.app.tv.shell.Label
import com.slouchcat.app.tv.shell.RailEntry
import com.slouchcat.app.tv.shell.RailMedia
import com.slouchcat.app.tv.shell.RailSection
import com.slouchcat.app.tv.shell.TvFeature
import com.slouchcat.app.tv.shell.TvScope
import com.slouchcat.app.tv.shell.isRoute
import com.slouchcat.core.browse.audio.AudioSeriesViewModel
import com.slouchcat.core.browse.audio.PodcastEpisodesViewModel
import com.slouchcat.core.browse.radio.RadioMessage
import com.slouchcat.core.browse.radio.RadioViewModel
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.ApiPlaybackServer
import com.slouchcat.core.player.ApiRadioServer
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.MeteredNetwork
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.util.Collections
import java.util.WeakHashMap

/**
 * Stage 4 «Ton» (concept 26 5.6, 3.3; mockup `media.js`): the audio player of a title and of a
 * station (pages without the rail), «Webradio» in the rail after the libraries, the series of an
 * audiobook, and above every page «Läuft gerade» (top right and in the rail) while something plays.
 * Audiobooks and podcasts are detail pages of `ItemFeature` ([AudioItemPage]); their buttons open
 * [ListenRoute]. The sound plays on across the pages and under the screen saver; Home and standby
 * end the session at the server ([TvAudio]). [playback], [source]: tests bring fakes.
 */
class AudioFeature(
    private val playback: (Context, TvGraph, CoroutineScope) -> AudioPlayback = ::tvPlayback,
    private val source: (TvGraph) -> AudioSource = { graph -> AudioSource("online") { ApiPlaybackServer(graph.api, graph.http) { graph.session.server } } },
) : TvFeature {
    private val audios = Collections.synchronizedMap(WeakHashMap<TvGraph, TvAudio>())

    /** The audio of the graph shown last: the rail entry «Läuft gerade» reads its state. */
    @Volatile
    private var shown: TvAudio? = null

    /** The sound of [graph], built once. */
    fun audioOf(graph: TvGraph, context: Context): TvAudio =
        audios.getOrPut(graph) {
            TvAudio.create(graph) { scope -> playback(context.applicationContext, graph, scope) }.also { listen(context.applicationContext, it.lifecycle) }
        }.also { shown = it }

    override val fullScreen = setOf(ListenRoute::class, RadioListenRoute::class)

    override val media = listOf(
        RailMedia {
            listOfNotNull(
                RailEntry(
                    key = RADIO,
                    section = RailSection.Media,
                    order = RADIO_ORDER,
                    label = Label.Res(R.string.app_radio_title),
                    icon = SlouchcatIcon.Radio.vector,
                    route = RadioRoute,
                    isCurrent = { it.isRoute(RadioRoute::class) || it.isRoute(RadioListenRoute::class) },
                ),
                nowEntry(),
            )
        },
    )

    /**
     * «Läuft gerade» in the rail (mockup `mdRailNow`): to the audio player of the title, or to the
     * radio. Reads the Compose state of [TvAudio.now], so the rail follows what plays.
     */
    private fun nowEntry(): RailEntry? = when (val now = shown?.now?.value) {
        null -> null
        is Playing.Title -> RailEntry(
            key = NOW,
            section = RailSection.Media,
            order = NOW_ORDER,
            label = Label.Res(if (now.paused) R.string.radio_page_paused else R.string.tv_audio_nowPlaying),
            icon = AudioIcons.Bars,
            route = ListenRoute(now.itemId, show = true),
            isCurrent = { false },
        )
        is Playing.Radio -> RailEntry(
            key = NOW,
            section = RailSection.Media,
            order = NOW_ORDER,
            label = Label.Res(R.string.tv_audio_radioPlaying),
            icon = AudioIcons.Bars,
            route = RadioListenRoute,
            isCurrent = { false },
        )
    }

    override val overlay: @Composable (TvScope, NavBackStackEntry?, Boolean) -> Unit = { app, page, framed ->
        val audio = audioOf(app.graph, LocalContext.current)
        AudioLifecycle(audio)
        // A video: the title's session ends before the video's starts, a station ends (mockup
        // `hashchange`, review M-2); back from it the title waits paused.
        DisposableEffect(app.navigator, audio) {
            val off = app.navigator.onPage { destination -> if (destination.hasRoute(PlayerRoute::class)) audio.videoStarts() else audio.videoEnds() }
            onDispose { off() }
        }
        val now by audio.playback.now.collectAsStateWithLifecycle()
        val audioPage = page != null && (page.isRoute(ListenRoute::class) || page.isRoute(RadioListenRoute::class) || page.isRoute(PlayerRoute::class))
        // The media keys on the other pages (review m-5): to the sound, not to Media3's own session.
        if (now != null && !audioPage) OnRemoteKey(app.graph.remoteKeys) { key -> mediaKey(audio, key) }
        ArtworkOf(audio, app.graph)
        val onRadio = page?.isRoute(RadioRoute::class) == true
        if (framed && !(onRadio && now is AudioNow.Radio)) NowPill(now, app.graph.session.server)
    }

    /** A media key of the remote on a page without the audio player. */
    private fun mediaKey(audio: TvAudio, key: RemoteKey) {
        val playback = audio.playback
        val title = playback.title
        val radio = playback.radio
        when (key) {
            RemoteKey.PlayPause -> playback.togglePlay()
            RemoteKey.Play -> { title?.play(); radio?.play() }
            RemoteKey.Pause -> { title?.pause(); radio?.pause() }
            RemoteKey.Stop -> playback.stop(forget = false)
            RemoteKey.FastForward -> title?.seekBy(com.slouchcat.core.player.AudioRules.FORWARD_MS)
            RemoteKey.Rewind -> title?.seekBy(-com.slouchcat.core.player.AudioRules.BACK_MS)
            RemoteKey.Next -> if (title?.state?.value?.chapters.orEmpty().isNotEmpty()) playback.nextChapter() else title?.playNext()
            RemoteKey.Previous -> if (title?.state?.value?.chapters.orEmpty().isNotEmpty()) playback.previousChapter() else title?.playPrevious()
        }
    }

    override fun NavGraphBuilder.register(app: TvScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ListenRoute> { entry ->
            val route = entry.toRoute<ListenRoute>()
            val audio = audioOf(graph, LocalContext.current)
            val playback = audio.playback
            // Starts once per page: Back to it or a new activity do not start it again.
            var started by rememberSaveable { mutableStateOf(false) }
            LaunchedEffect(route) {
                if (!started) {
                    started = true
                    if (!route.show) playback.playTitle(route.itemId, source(graph), route.startAtMs.takeIf { it >= 0 }, route.fromStart)
                }
            }
            val now by playback.now.collectAsStateWithLifecycle()
            val title = (now as? AudioNow.Title)?.state
            val podcast = title?.item?.takeIf { it.kind == com.slouchcat.core.api.generated.models.ItemKind.PODCAST_EPISODE }?.let { it.parentId ?: it.seriesRef?.id }
            val episodes = podcast?.let { id ->
                val vm = viewModel(key = "episodes-$id") { PodcastEpisodesViewModel(graph.session, graph.media.audio, graph.media.detail, id) }
                val paged by vm.state.collectAsStateWithLifecycle()
                EpisodesUi(paged, emptySet(), vm::onNearEnd, vm::retry, vm::onHeard)
            }
            val server = graph.session.server
            ListenScreen(
                state = title,
                images = remember(server) { server?.let(::ServerImages) },
                episodes = episodes,
                remoteKeys = graph.remoteKeys,
                events = remember(playback) { PlaybackListenEvents(playback, source(graph), navigator::back) },
            )
        }
        composable<RadioListenRoute> {
            val audio = audioOf(graph, LocalContext.current)
            val now by audio.playback.now.collectAsStateWithLifecycle()
            val radio = (now as? AudioNow.Radio)?.state
            // Nothing plays any more (ended elsewhere, Stop): back to where it came from.
            LaunchedEffect(radio == null) { if (radio == null) navigator.back() }
            // The heart through the radio's own ViewModel (review m-6): sooner, back on a refusal, the message.
            val vm = viewModel { RadioViewModel(graph.session, graph.media.radio) }
            val ui by vm.ui.collectAsStateWithLifecycle()
            RadioMessages(vm, graph)
            val station = radio?.station
            val favorites = (ui.favorites as? com.slouchcat.core.media.Loadable.Ready)?.value
            RadioListenScreen(
                state = radio,
                server = graph.session.server,
                favorite = station != null && (favorites?.any { it.id == station.id } ?: station.favorite),
                remoteKeys = graph.remoteKeys,
                events = object : RadioListenEvents {
                    override fun onToggle() = audio.playback.togglePlay()
                    override fun onPlay() {
                        audio.playback.radio?.play()
                    }
                    override fun onPause() {
                        audio.playback.radio?.pause()
                    }
                    override fun onStop() {
                        audio.playback.stop(forget = false)
                    }
                    override fun onRetry() {
                        radio?.station?.let(audio.playback::playRadio)
                    }
                    override fun onFavorite() {
                        val current = station ?: return
                        val on = favorites?.any { it.id == current.id } ?: current.favorite
                        vm.onFavorite(current.copy(favorite = on))
                    }
                    override fun onBack() {
                        navigator.back()
                    }
                },
            )
        }
        composable<AudioSeriesRoute> { entry ->
            val route = entry.toRoute<AudioSeriesRoute>()
            val vm = viewModel { AudioSeriesViewModel(graph.session, graph.media.audio, route.libraryId, route.seriesId) }
            val state by vm.state.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onShown() }
            val server = graph.session.server ?: return@composable
            AudioSeriesScreen(
                state = state,
                images = remember(server) { ServerImages(server) },
                links = SeriesLinks(
                    retry = vm::retry,
                    item = navigator.canOpen(ItemRoute::class).takeIf { it }?.let { { id: String -> navigator.navigate(ItemRoute(id)) } },
                    play = navigator.canOpen(ListenRoute::class).takeIf { it }?.let { { id: String -> navigator.navigate(ListenRoute(id)) } },
                ),
            )
        }
        composable<RadioRoute> {
            val audio = audioOf(graph, LocalContext.current)
            val vm = viewModel { RadioViewModel(graph.session, graph.media.radio) }
            val ui by vm.ui.collectAsStateWithLifecycle()
            val stations by vm.stations.collectAsStateWithLifecycle()
            val now by audio.playback.now.collectAsStateWithLifecycle()
            RadioMessages(vm, graph)
            // Back from the station's page (a heart there): the lists again, quietly (review m-6).
            var shown by remember { mutableStateOf(false) }
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { if (shown) vm.retry() else shown = true }
            val playback = audio.playback
            RadioScreen(
                ui = ui,
                stations = stations,
                playing = (now as? AudioNow.Radio)?.state,
                server = graph.session.server,
                events = object : RadioEvents {
                    override fun onRetry() = vm.retry()
                    override fun onNearEnd() = vm.onNearEnd()
                    override fun onSearch(text: String) = vm.onSearch(text)
                    override fun onCountry(code: String?) = vm.onCountry(code)
                    override fun onFavorite(station: com.slouchcat.core.api.generated.models.RadioStation) = vm.onFavorite(station)
                    override fun onStation(station: com.slouchcat.core.api.generated.models.RadioStation) {
                        // OK starts a station, OK again pauses it (mockup).
                        val playing = (playback.now.value as? AudioNow.Radio)?.state
                        if (playing?.station?.id == station.id && playing.error == null) playback.togglePlay() else playback.playRadio(station)
                    }
                    override fun onOpenPlaying() = navigator.navigate(RadioListenRoute)
                },
            )
        }
    }

    companion object {
        const val RADIO = "radio"
        const val NOW = "now"

        /** After the libraries (`LibraryFeature.ORDER` + index). */
        const val RADIO_ORDER = 1_000
        const val NOW_ORDER = 1_001
    }
}

/**
 * The sound of the TV: Media3 with the TV's own media session (Play/Pause of the remote also on
 * a page without the audio player), the server's radio, what the TV decodes (as the video player).
 */
@OptIn(UnstableApi::class)
fun tvPlayback(context: Context, graph: TvGraph, scope: CoroutineScope): AudioPlayback {
    val store = tvDeviceSettings(context)
    return AudioPlayback(
        scope = scope,
        engines = { kind -> Media3Engine(context, graph.http, withSession = true, kind = kind) },
        radioServer = { ApiRadioServer(graph.api, graph.http) { graph.session.server } },
        capabilities = { narrowing ->
            withContext(Dispatchers.Default) {
                CapabilityRules.build(TvFactsReader(context) { graph.device.name }.read(), store.settings.first().audio, narrowing)
            }
        },
        settings = store.settings,
        metered = MeteredNetwork.of(context),
        clock = SystemClock::elapsedRealtime,
    )
}

/**
 * The activity's lifecycle for the sound ([AudioForeground] in the graph's scope, review B-1): Stop
 * and Start, and the end of the activity for good (`finish()`, the second Back of the start page)
 * ends the sound at once. Whether a page is seen also tells the radio to ask for its current title
 * ([AudioPlayback.visible]).
 */
@Composable
private fun AudioLifecycle(audio: TvAudio) {
    val activity = androidx.activity.compose.LocalActivity.current
    val foreground = audio.lifecycle
    LifecycleEventEffect(Lifecycle.Event.ON_STOP) {
        audio.playback.visible = false
        foreground.onStop()
    }
    LifecycleEventEffect(Lifecycle.Event.ON_START) {
        audio.playback.visible = true
        foreground.onStart()
    }
    DisposableEffect(activity, foreground) {
        onDispose { if (activity?.isFinishing == true) foreground.onFinish() }
    }
}

/**
 * The screen saver, standby and the HDMI output as the system tells them (review m-7), on the
 * application's context for the life of the process: also while the activity is gone.
 */
private fun listen(context: Context, foreground: AudioForeground) {
    val receiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            when (intent.action) {
                Intent.ACTION_DREAMING_STARTED -> foreground.onDreamingStarted()
                Intent.ACTION_DREAMING_STOPPED -> foreground.onDreamingStopped()
                Intent.ACTION_SCREEN_OFF -> foreground.onScreenOff()
                // The sticky state at the registration is no change.
                AudioManager.ACTION_HDMI_AUDIO_PLUG -> if (!isInitialStickyBroadcast) {
                    foreground.onHdmi(intent.getIntExtra(AudioManager.EXTRA_AUDIO_PLUG_STATE, 1) != 0)
                }
            }
        }
    }
    val filter = IntentFilter().apply {
        addAction(Intent.ACTION_DREAMING_STARTED)
        addAction(Intent.ACTION_DREAMING_STOPPED)
        addAction(Intent.ACTION_SCREEN_OFF)
        addAction(AudioManager.ACTION_HDMI_AUDIO_PLUG)
    }
    ContextCompat.registerReceiver(context, receiver, filter, ContextCompat.RECEIVER_NOT_EXPORTED)
}

/** The short messages of the radio's ViewModel as toasts. */
@Composable
private fun RadioMessages(vm: RadioViewModel, graph: TvGraph) {
    LaunchedEffect(vm) {
        vm.messages.collect { message ->
            when (message) {
                is RadioMessage.Added -> graph.toasts.show(R.string.app_radio_added, message.name)
                is RadioMessage.Removed -> graph.toasts.show(R.string.app_radio_removed, message.name)
                RadioMessage.Failed -> graph.toasts.show(R.string.app_radio_failed)
            }
        }
    }
}

/** The cover of what plays as a picture for the system's media card (no address in the metadata; review L-6). */
@Composable
private fun ArtworkOf(audio: TvAudio, graph: TvGraph) {
    val context = LocalContext.current.applicationContext
    LaunchedEffect(audio) {
        audio.playback.now.map { coverUrl(graph, it) }.distinctUntilChanged().collectLatest { url ->
            val bytes = url?.let { load(context, graph, it) }
            (audio.playback.engine.value as? Media3Engine)?.updateArtwork(bytes)
        }
    }
}

private fun coverUrl(graph: TvGraph, now: AudioNow?): String? {
    val server = graph.session.server ?: return null
    return when (now) {
        is AudioNow.Title -> ServerImages(server).posters(now.state.item?.images).pick(ARTWORK_PX)
        is AudioNow.Radio -> stationImage(server, now.state.now?.cover?.url ?: now.state.station?.logo?.url) as? String
        else -> null
    }
}

private suspend fun load(context: Context, graph: TvGraph, url: String): ByteArray? {
    val request = coil3.request.ImageRequest.Builder(context).data(url).allowHardware(false).build()
    val bitmap = (graph.imageLoader.execute(request) as? coil3.request.SuccessResult)?.image?.toBitmap() ?: return null
    return withContext(Dispatchers.Default) {
        java.io.ByteArrayOutputStream().use { out ->
            bitmap.compress(android.graphics.Bitmap.CompressFormat.JPEG, JPEG_QUALITY, out)
            out.toByteArray()
        }
    }
}

private const val ARTWORK_PX = 512
private const val JPEG_QUALITY = 85
