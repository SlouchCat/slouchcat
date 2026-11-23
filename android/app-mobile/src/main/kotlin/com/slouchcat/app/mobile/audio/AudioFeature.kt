package com.slouchcat.app.mobile.audio

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LifecycleStartEffect
import com.slouchcat.app.mobile.downloads.DownloadAction
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.design.ButtonKind
import com.slouchcat.core.design.SlouchcatButton
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.browse.AudiobooksRoute
import com.slouchcat.app.mobile.browse.PodcastsRoute
import com.slouchcat.app.mobile.browse.AudioSeriesRoute
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.ListenRoute
import com.slouchcat.app.mobile.browse.MORE_TARGET
import com.slouchcat.app.mobile.browse.RadioRoute
import com.slouchcat.app.mobile.home.LibraryPage
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.Label
import com.slouchcat.app.mobile.shell.MediaEntries
import com.slouchcat.app.mobile.shell.MediaEntry
import com.slouchcat.app.mobile.shell.MediaSection
import com.slouchcat.app.mobile.shell.MoreEntry
import com.slouchcat.app.mobile.shell.MoreSection
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.browse.audio.AudioSeriesViewModel
import com.slouchcat.core.browse.radio.RadioMessage
import com.slouchcat.core.browse.radio.RadioViewModel
import com.slouchcat.core.design.EmptyState
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.media.ImageSet
import com.slouchcat.core.media.ImageSource
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioRules
import com.slouchcat.core.player.ServerPaths
import com.slouchcat.core.player.SleepChoice
import com.slouchcat.core.strings.R
import okhttp3.HttpUrl

/**
 * Stage 7 «Ton» (concept 27 8.3, 3.1): the audio player (a page of its own), the small player above
 * the bar or at the foot of the side column, «Webradio» (media choice and «Mehr»), «Hörbücher» and
 * «Podcasts» in «Mehr», the series of an audiobook. Audiobooks and podcasts themselves are detail
 * pages of `ItemFeature`; their buttons open [ListenRoute] (`playRoute`). Everything plays through
 * the graph's `AudioPlayback`, so it plays on across pages and in the background ([AudioService]).
 */
object AudioFeature : Feature {
    override val fullScreen = setOf(ListenRoute::class)

    override val media = listOf(
        MediaEntries {
            listOf(
                MediaEntry(
                    key = "radio",
                    section = MediaSection.Media,
                    order = 90,
                    label = Label.Res(R.string.app_radio_title),
                    icon = SlouchcatIcon.Radio,
                    route = RadioRoute,
                    target = MORE_TARGET,
                    isCurrent = { it.destination.hasRoute(RadioRoute::class) },
                ),
            )
        },
    )

    override val more = listOf(
        MoreEntry("audiobooks", MoreSection.Library, 40, R.string.app_audio_audiobooks, SlouchcatIcon.Headphones, AudiobooksRoute, subtitle = { libraryCount(it, LibraryKind.AUDIOBOOKS) }),
        MoreEntry("podcasts", MoreSection.Library, 50, R.string.app_audio_podcasts, SlouchcatIcon.Mic, PodcastsRoute, subtitle = { libraryCount(it, LibraryKind.PODCASTS) }),
        MoreEntry("radio", MoreSection.Library, 60, R.string.app_radio_title, SlouchcatIcon.Radio, RadioRoute, subtitle = { androidx.compose.ui.res.stringResource(R.string.app_radio_moreSubtitle) }),
    )

    override val accessory: @Composable (AppScope) -> Unit = { app -> Mini(app) }

    override fun link(address: String): AppLink? = when (address.removePrefix("/")) {
        "radio" -> AppLink(MORE_TARGET, listOf(RadioRoute))
        else -> null
    }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ListenRoute> { entry ->
            val route = entry.toRoute<ListenRoute>()
            val audio = graph.audio
            val playback = audio.playback
            // Plays once per page and only from a button that plays (route.play): opening the player
            // only shows (code review M-1); a page restored after the death of the process plays nothing.
            var started by rememberSaveable { mutableStateOf(false) }
            val askNotifications = rememberNotificationAsk(audio)
            LaunchedEffect(route) {
                if (!started && route.play) {
                    started = true
                    askNotifications()
                    val source = if (route.offline) audio.offlineSource() else audio.online
                    if (source != null) playback.playTitle(route.itemId, source, route.startAtMs.takeIf { it >= 0 }, route.fromStart)
                }
            }
            val now by playback.now.collectAsStateWithLifecycle()
            ListenPage(app, audio, now) { navigator.back() }
        }
        composable<AudioSeriesRoute> { entry ->
            val route = entry.toRoute<AudioSeriesRoute>()
            val vm = viewModel { AudioSeriesViewModel(graph.session, graph.media.audio, route.libraryId, route.seriesId) }
            val state by vm.state.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onShown() }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            AudioSeriesScreen(
                state,
                images,
                SeriesLinks(
                    back = navigator::back,
                    retry = vm::retry,
                    item = if (navigator.canOpen(ItemRoute::class)) ({ navigator.navigate(ItemRoute(it)) }) else null,
                    play = { navigator.navigate(ListenRoute(it)) },
                ),
            )
        }
        composable<AudiobooksRoute> { KindLibrary(app, LibraryKind.AUDIOBOOKS) }
        composable<PodcastsRoute> { KindLibrary(app, LibraryKind.PODCASTS) }
        composable<RadioRoute> {
            val vm = viewModel { RadioViewModel(graph.session, graph.media.radio) }
            val ui by vm.ui.collectAsStateWithLifecycle()
            val stations by vm.stations.collectAsStateWithLifecycle()
            val now by graph.audio.playback.now.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    is RadioMessage.Added -> getString(R.string.app_radio_added, message.name)
                    is RadioMessage.Removed -> getString(R.string.app_radio_removed, message.name)
                    RadioMessage.Failed -> getString(R.string.app_radio_failed)
                }
            }
            val server = graph.session.server
            val playback = graph.audio.playback
            val askNotifications = rememberNotificationAsk(graph.audio)
            RadioScreen(
                ui = ui,
                stations = stations,
                playing = (now as? AudioNow.Radio)?.state,
                logo = { station -> stationLogo(server, station) },
                events = object : RadioEvents {
                    override fun onBack() = navigator.back()
                    override fun onRetry() = vm.retry()
                    override fun onNearEnd() = vm.onNearEnd()
                    override fun onSearch(text: String) = vm.onSearch(text)
                    override fun onCountry(code: String?) = vm.onCountry(code)
                    override fun onCountrySheet(open: Boolean) = vm.onCountrySheet(open)
                    override fun onFavoritesOnly() = vm.onFavoritesOnly()
                    override fun onFavorite(station: RadioStation) = vm.onFavorite(station)
                    override fun onStation(station: RadioStation) {
                        val playing = (playback.now.value as? AudioNow.Radio)?.state
                        if (playing?.station?.id == station.id && playing.error == null) {
                            playback.togglePlay()
                        } else {
                            askNotifications()
                            playback.playRadio(station)
                        }
                    }
                },
                snackbar = snackbar,
            )
        }
    }

    /** The small player over every page of the shell: what plays, or nothing. */
    @Composable
    private fun Mini(app: AppScope) {
        val graph = app.graph
        val playback = graph.audio.playback
        val now by playback.now.collectAsStateWithLifecycle()
        val current = now ?: return
        val server = graph.session.server
        val cover = remember(current, server) { coverOf(current, server) }
        val audio = graph.audio
        LifecycleStartEffect(audio) {
            audio.show()
            onStopOrDispose { audio.hide() }
        }
        MiniPlayer(
            ui = miniUi(current, cover),
            onOpen = {
                when (current) {
                    is AudioNow.Title -> current.state.item?.id?.let { app.navigator.navigate(ListenRoute(it, offline = current.offline, play = false)) }
                    is AudioNow.Restored -> app.navigator.navigate(ListenRoute(current.last.itemId, offline = current.last.offline, play = false))
                    is AudioNow.Radio -> app.navigator.navigate(RadioRoute)
                }
            },
            onToggle = playback::togglePlay,
            onStop = playback::stop,
        )
    }
}

/** The first library of [kind] the profile sees, as its own page (`LibraryPage` of the start page). */
@Composable
private fun KindLibrary(app: AppScope, kind: LibraryKind) {
    val libraries by app.graph.librariesSource.libraries.collectAsStateWithLifecycle()
    val id = (libraries as? Loadable.Ready)?.value?.firstOrNull { it.kind == kind }?.id
    if (id != null) {
        LibraryPage(app, id, null)
    } else if (libraries !is Loadable.Loading) {
        EmptyState(androidx.compose.ui.res.stringResource(R.string.app_audio_noLibrary), icon = SlouchcatIcon.Info)
    }
}

/**
 * The audio player page over [now]; [leave]: «Tonplayer schliessen» (the sound plays on). The info
 * button opens the title's page, «Laden» the download dialog of stage 6 (online only; the dialog
 * says when the title is already on the device).
 */
@Composable
private fun ListenPage(app: AppScope, audio: AudioData, now: AudioNow?, leave: () -> Unit) {
    val graph = app.graph
    val playback = audio.playback
    val server = graph.session.server
    val title = now as? AudioNow.Title
    val restored = now as? AudioNow.Restored
    val offline = title?.offline ?: restored?.last?.offline ?: false
    val itemId = title?.state?.item?.id ?: restored?.last?.itemId
    // The cover of a download comes from the device (code review m-11), else from the server.
    val stored by produceState<java.io.File?>(null, itemId, offline) { value = if (offline && itemId != null) offlinePoster(graph, itemId) else null }
    val cover = remember(now?.let { coverKey(it) }, server, stored) {
        stored?.let { ImageSet(listOf(ImageSource("file://" + it.absolutePath, LOGO_WIDTH))) } ?: now?.let { coverOf(it, server) } ?: ImageSet()
    }
    LifecycleStartEffect(audio) {
        audio.show()
        onStopOrDispose { audio.hide() }
    }
    val detail by produceState<ItemDetail?>(null, itemId, server, offline) {
        value = if (!offline && itemId != null && server != null) (graph.media.detail.item(server, itemId) as? ApiResult.Ok)?.value else null
    }
    val snackbar = remember { SnackbarHostState() }
    val navigator = app.navigator
    ListenScreen(
        ui = ListenUi(state = title?.state, sleep = title?.sleep, offline = title?.offline == true, restored = restored?.last, cover = cover),
        onDetails = itemId?.takeIf { !offline && navigator.canOpen(ItemRoute::class) }?.let { id -> { navigator.navigate(ItemRoute(id)) } },
        download = detail?.let { item ->
            {
                DownloadAction(graph, item, snackbar, look = { icon, label, onClick ->
                    SlouchcatButton(label, onClick = onClick, kind = ButtonKind.Ghost, leadingIcon = icon)
                })
            }
        },
        events = object : ListenEvents {
            override fun onClose() = leave()
            override fun onTogglePlay() = playback.togglePlay()
            override fun onSeekTo(positionMs: Long) {
                playback.title?.seekTo(positionMs)
            }
            override fun onBack() {
                playback.title?.seekBy(-AudioRules.BACK_MS)
            }
            override fun onForward() {
                playback.title?.seekBy(AudioRules.FORWARD_MS)
            }
            override fun onPreviousChapter() = playback.previousChapter()
            override fun onNextChapter() = playback.nextChapter()
            override fun onPreviousEpisode() {
                playback.title?.playPrevious()
            }
            override fun onNextEpisode() {
                playback.title?.playNext()
            }
            override fun onSpeed(speed: Float) {
                playback.title?.setSpeed(AudioRules.clampSpeed(speed))
            }
            override fun onSleep(choice: SleepChoice) = playback.setSleep(choice)
            override fun onChapter(chapter: Chapter) {
                playback.title?.seekTo(chapter.startMs)
            }
            override fun onRetry() {
                playback.title?.retry()
            }
        },
    )
}

/**
 * Asks for the notifications once per run of the app, at the first start of a sound (security review
 * L-7): without them Android 13 and later show no controls in the notification. A refusal changes
 * nothing; the sound plays all the same.
 */
@Composable
private fun rememberNotificationAsk(audio: AudioData): () -> Unit {
    val context = LocalContext.current
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { }
    return remember(launcher, audio) {
        {
            val missing = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
            if (missing && !audio.notificationsAsked) {
                audio.notificationsAsked = true
                launcher.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
        }
    }
}

private fun coverKey(now: AudioNow): Any? = when (now) {
    is AudioNow.Title -> now.state.item?.id
    is AudioNow.Radio -> now.state.station?.id to now.state.now?.cover?.url
    is AudioNow.Restored -> now.last.itemId
}

/** The cover of what plays: the title's poster, the station's current track cover or its logo. */
private fun coverOf(now: AudioNow, server: HttpUrl?): ImageSet {
    server ?: return ImageSet()
    return when (now) {
        is AudioNow.Title -> ServerImages(server).posters(now.state.item?.images)
        is AudioNow.Restored -> ImageSet()
        is AudioNow.Radio -> {
            val path = now.state.now?.cover?.url ?: now.state.station?.logo?.url
            path?.let { ServerPaths.absolute(server, it) }?.let { ImageSet(listOf(ImageSource(it, LOGO_WIDTH))) } ?: ImageSet()
        }
    }
}

private fun stationLogo(server: HttpUrl?, station: RadioStation): ImageSet {
    val path = station.logo?.url ?: return ImageSet()
    val url = server?.let { ServerPaths.absolute(it, path) } ?: return ImageSet()
    return ImageSet(listOf(ImageSource(url, LOGO_WIDTH)))
}

@Composable
private fun libraryCount(graph: com.slouchcat.app.mobile.AppGraph, kind: LibraryKind): String? {
    val libraries by graph.librariesSource.libraries.collectAsStateWithLifecycle()
    val library = (libraries as? Loadable.Ready)?.value?.firstOrNull { it.kind == kind } ?: return null
    return com.slouchcat.app.mobile.media.titleCount(library.itemCount)
}

private const val LOGO_WIDTH = 256
