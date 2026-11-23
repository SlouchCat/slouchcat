package com.slouchcat.app.mobile.item

import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalUriHandler
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.slouchcat.app.mobile.AppGraph
import androidx.compose.runtime.Composable
import com.slouchcat.app.mobile.audio.AudioDetailLinks
import com.slouchcat.app.mobile.audio.AudioDetailSections
import com.slouchcat.app.mobile.audio.EpisodesUi
import com.slouchcat.app.mobile.browse.AudioSeriesRoute
import com.slouchcat.app.mobile.browse.CollectionRoute
import com.slouchcat.app.mobile.browse.ListenRoute
import com.slouchcat.app.mobile.browse.playRoute
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.audio.PodcastEpisodesViewModel
import com.slouchcat.core.media.isAudio
import com.slouchcat.app.mobile.browse.ItemRoute
import com.slouchcat.app.mobile.browse.PersonRoute
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.app.mobile.browse.SEARCH_TARGET
import com.slouchcat.app.mobile.downloads.DownloadAction
import com.slouchcat.app.mobile.downloads.DownloadsRoute
import com.slouchcat.app.mobile.media.ShowMessages
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.AppScope
import com.slouchcat.app.mobile.shell.Feature
import com.slouchcat.core.browse.item.ItemMessage
import com.slouchcat.core.browse.item.ItemViewModel
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.strings.R

/**
 * The detail pages of stage 4 (concept 27, 3.1; mockup `/item/:i`): film, series with seasons and
 * episodes, season, episode, and a plain page for every other kind until its own page comes
 * (audiobooks and podcasts in stage 7). In-app address `item/<id>` (a notification of stage 8).
 * [repository]: the data of the page; tests bring a fake.
 */
class ItemFeature(
    private val repository: (AppGraph) -> DetailRepository = { it.media.detail },
) : Feature {
    override fun link(address: String): AppLink? =
        address.removePrefix("/").takeIf { it.startsWith("item/") }?.removePrefix("item/")?.takeIf { it.isNotBlank() && '/' !in it }
            ?.let { AppLink(TARGET, listOf(ItemRoute(it))) }

    override fun NavGraphBuilder.register(app: AppScope) {
        val graph = app.graph
        val navigator = app.navigator
        composable<ItemRoute> { entry ->
            val id = entry.toRoute<ItemRoute>().id
            val vm = viewModel { ItemViewModel(graph.session, repository(graph), id, watchlistCount = graph.media.watchlistCount) }
            val state by vm.state.collectAsStateWithLifecycle()
            val snackbar = remember { SnackbarHostState() }
            val uri = LocalUriHandler.current
            ShowMessages(vm.messages, snackbar) { message ->
                when (message) {
                    ItemMessage.Failed -> getString(R.string.app_item_failed)
                    is ItemMessage.Recommended -> getQuantityString(R.plurals.item_recommend_added, message.days, message.days)
                    ItemMessage.Unrecommended -> getString(R.string.item_recommend_removed)
                }
            }
            val server = graph.session.server ?: return@composable
            val images = remember(server) { ServerImages(server) }
            // Audiobooks and podcast episodes play in the audio player of stage 7, everything else in the video player.
            val kind = state.detail?.kind
            val play = navigator.opener(PlayerRoute(id))?.let { { target: String -> navigator.navigate(playRoute(target, kind)) } }
            val playFromStart = navigator.opener(PlayerRoute(id))?.let { { target: String -> navigator.navigate(playRoute(target, kind, fromStart = true)) } }
            val audioLinks = AudioDetailLinks(
                playAt = if (navigator.canOpen(ListenRoute::class)) ({ target, at -> navigator.navigate(ListenRoute(target, startAtMs = at)) }) else null,
                play = if (navigator.canOpen(ListenRoute::class)) ({ target -> navigator.navigate(ListenRoute(target)) }) else null,
                item = { navigator.opener(ItemRoute(it)) },
                series = if (navigator.canOpen(AudioSeriesRoute::class)) ({ library, series -> navigator.navigate(AudioSeriesRoute(library, series)) }) else null,
            )
            // Back from the player: the page reads the title again quietly (the new progress, stage 5).
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onShown() }
            ItemScreen(
                state = state,
                images = images,
                links = ItemLinks(
                    back = navigator::back,
                    item = { navigator.opener(ItemRoute(it)) },
                    person = { navigator.opener(PersonRoute(it)) },
                    collection = { navigator.opener(CollectionRoute(it)) },
                    search = navigator.openerOf(SEARCH_TARGET),
                    play = play,
                    playFromStart = playFromStart,
                    // No app for the address (no browser, a kiosk profile): a notice, not a crash (code review m-4).
                    trailer = { youtube -> runCatching { uri.openUri(YOUTUBE + youtube) }.onFailure { graph.notices.show(R.string.app_item_trailerFailed) } },
                    audioSections = if (kind != null && isAudio(kind)) ({ detail -> AudioSections(graph, detail, audioLinks, snackbar) }) else null,
                ),
                events = vm,
                snackbar = snackbar,
                // Stage 6: the round action «Herunterladen» with its dialog, when the app has the downloads.
                download = if (navigator.canOpen(DownloadsRoute::class)) ({ item -> DownloadAction(graph, item, snackbar) }) else null,
            )
        }
    }

    /** The chapters of an audiobook, the episodes of a podcast with their own list (stage 7). */
    @Composable
    private fun AudioSections(graph: AppGraph, detail: ItemDetail, links: AudioDetailLinks, snackbar: SnackbarHostState) {
        val episodes = if (detail.kind == ItemKind.PODCAST) {
            val vm = viewModel(key = "episodes-${detail.id}") { PodcastEpisodesViewModel(graph.session, graph.media.audio, repository(graph), detail.id) }
            val paged by vm.state.collectAsStateWithLifecycle()
            val busy by vm.busy.collectAsStateWithLifecycle()
            LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onShown() }
            ShowMessages(vm.messages, snackbar) { getString(R.string.app_audio_failed) }
            EpisodesUi(paged, busy, vm::onNearEnd, vm::retry, vm::onHeard)
        } else {
            null
        }
        AudioDetailSections(detail, links, episodes)
    }

    private companion object {
        /** The target a jump to a title opens under: «Start» (the first target when there is none). */
        const val TARGET = "start"
        const val YOUTUBE = "https://www.youtube.com/watch?v="
    }
}
