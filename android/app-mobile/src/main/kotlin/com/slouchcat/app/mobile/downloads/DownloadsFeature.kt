package com.slouchcat.app.mobile.downloads

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalResources
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.item.ActionButton
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.player.PlayerFeature
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.app.mobile.shell.TopDestination
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.design.SlouchcatIcon
import com.slouchcat.core.design.isWideLayout
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.offline.OfflinePlaybackServer
import com.slouchcat.core.strings.R
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable

/** «Downloads»: the target of the bottom bar (order 30). */
@Serializable
data object DownloadsRoute

/**
 * The offline player of a title on the device (9.1, `/offline/play/$downloadId` of the PWA). No
 * profile in the route: it plays for the profile opened now (the one chosen offline with its PIN, or
 * the session's), also after the process was restored (security review L-6).
 */
@Serializable
data class OfflinePlayerRoute(val itemId: String)

/**
 * Stage 6 (concept 27 section 9): the target «Downloads» with the offline library, the offline
 * player over `:core:player` (the local HLS playlist), and the round action «Herunterladen» of the
 * detail pages with its dialog ([downloadAction]).
 */
object DownloadsFeature : Feature {
    const val TARGET = "downloads"

    override val destinations = listOf(TopDestination(TARGET, 30, DownloadsRoute, R.string.nav_downloads, SlouchcatIcon.Download))

    override val fullScreen = setOf(OfflinePlayerRoute::class)

    override fun link(address: String) = address.removePrefix("/").takeIf { it == "downloads" }?.let { com.slouchcat.app.mobile.shell.AppLink(TARGET, listOf(DownloadsRoute)) }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<DownloadsRoute> {
            val data = graph.offline
            val vm = viewModel { DownloadsViewModel(data.engine, graph.session, data.settings, data.viewer, data.network) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            ShowMessages(vm.events, snackbar) { message ->
                when (message) {
                    DownloadsMessage.Paused -> getString(R.string.app_downloads_paused)
                    DownloadsMessage.Resumed -> getString(R.string.app_downloads_resumed)
                    is DownloadsMessage.Deleted -> getQuantityString(R.plurals.app_downloads_deleted, message.count, message.count)
                    DownloadsMessage.RuleSaved -> getString(R.string.offline_rules_saved)
                }
            }
            DownloadsScreen(
                state = state,
                events = vm,
                links = DownloadsLinks(
                    // Stage 7: audiobooks and episodes play in the audio player, from the download.
                    play = { row -> navigator.navigate(if (row.audio) com.slouchcat.app.mobile.browse.ListenRoute(row.itemId, offline = true) else OfflinePlayerRoute(row.itemId)) },
                    item = if (navigator.canOpen(ItemRoute::class)) ({ id: String -> navigator.navigate(ItemRoute(id)) }) else null,
                    start = { navigator.openTop(graph.registry.home) },
                ),
                snackbar = snackbar,
            )
        }
        composable<OfflinePlayerRoute> { entry ->
            val route = entry.toRoute<OfflinePlayerRoute>()
            val data = graph.offline
            val profile = data.viewer.value ?: (graph.session.state.value as? com.slouchcat.core.session.SessionState.Ready)?.profile?.id ?: return@composable
            val player = graph.registry.features.filterIsInstance<PlayerFeature>().firstOrNull() ?: return@composable
            player.PlayerPage(app, itemId = route.itemId, startAtMs = null, fromStart = false, key = "offline-${route.itemId}") {
                // Plays only while this profile is still the one opened (a changed viewer plays nothing).
                OfflinePlaybackServer(data.engine, { data.engine.account()?.key?.takeIf { (data.viewer.value ?: data.engine.lastAccount.value?.profileId) == profile } }, profile)
            }
        }
    }
}

/**
 * The round action of a detail page (slot `download` of ItemScreen): its state on the device and
 * the dialog. Only video titles (audiobooks and podcast episodes come with stage 7).
 */
@Composable
fun DownloadAction(
    graph: AppGraph,
    item: ItemDetail,
    snackbar: SnackbarHostState,
    modifier: Modifier = Modifier,
    /** Another button than the round action (stage 7: «Laden» of the audio player). */
    look: (@Composable (icon: com.slouchcat.core.design.SlouchcatIcon, label: String, onClick: () -> Unit) -> Unit)? = null,
) {
    if (!downloadable(item.kind)) return
    val data = graph.offline
    val vm = viewModel(key = "download-${item.id}") {
        DownloadDialogViewModel(data.engine, graph.media.items, data.settings, { graph.session.server }, item.id)
    }
    val onDevice by vm.onDevice.collectAsStateWithLifecycle()
    val state by vm.state.collectAsStateWithLifecycle()
    val (icon, label) = DownloadActionLabel(onDevice)
    val scope = rememberCoroutineScope()
    val resources = LocalResources.current
    DialogMessages(vm) { message ->
        if (message is DialogMessage.Started) {
            val text = if (message.count > 1) resources.getQuantityString(R.plurals.app_downloads_startedMany, message.count, message.count) else resources.getString(R.string.app_downloads_started, message.title)
            scope.launch { snackbar.showSnackbar(text) }
        }
    }
    if (look != null) look(icon, label) { vm.open(item) } else ActionButton(icon, label, null, { vm.open(item) }, isWideLayout(), modifier)
    // Mockup photos 45 and 46: «2026 · Film», or the series with «S1 · F1 · Titel» of the episode.
    val episode = state.episode
    val subtitle = when {
        episode != null && item.kind != com.slouchcat.core.api.generated.models.ItemKind.MOVIE ->
            androidx.compose.ui.res.stringResource(R.string.app_downloads_episodeLine, (episode.seasonNumber ?: 1).toString(), (episode.episodeNumber ?: 1).toString(), episode.title)
        else -> listOfNotNull(item.year?.toString(), com.slouchcat.app.mobile.media.kindName(item.kind)).joinToString(" · ").ifEmpty { null }
    }
    val audio = com.slouchcat.core.media.isAudio(item.kind)
    val title = when {
        // Stage 7 (mockup photos 47, 48): the audiobook or episode with its author or podcast.
        item.kind == com.slouchcat.core.api.generated.models.ItemKind.PODCAST -> state.episode?.title ?: item.title
        audio || item.kind == com.slouchcat.core.api.generated.models.ItemKind.MOVIE -> item.title
        else -> item.seriesTitle ?: item.seriesRef?.title ?: item.series?.title ?: item.title
    }
    val audioLine = when (item.kind) {
        com.slouchcat.core.api.generated.models.ItemKind.PODCAST -> item.title
        com.slouchcat.core.api.generated.models.ItemKind.PODCAST_EPISODE -> item.podcast?.title ?: item.author
        else -> item.author
    }
    val server = graph.session.server
    val poster = remember(server, item.id) { server?.let { ServerImages(it).posters(item.images) } ?: com.slouchcat.core.media.ImageSet(emptyList()) }
    DownloadDialog(
        state,
        title,
        if (audio) audioLine else subtitle,
        vm,
        poster,
        onOpenDownloads = { graph.links.request("downloads") },
        note = if (item.kind == com.slouchcat.core.api.generated.models.ItemKind.AUDIOBOOK && item.mediaFiles.size > 1) androidx.compose.ui.res.stringResource(R.string.app_audio_allParts) else null,
        square = audio,
    )
}
