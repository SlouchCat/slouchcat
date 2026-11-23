package com.slouchcat.app.tv.audio

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.slouchcat.app.tv.TvGraph
import com.slouchcat.app.tv.browse.AudioSeriesRoute
import com.slouchcat.app.tv.browse.ItemRoute
import com.slouchcat.app.tv.browse.ListenRoute
import com.slouchcat.app.tv.shell.TvNavigator
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.browse.audio.EpisodesMessage
import com.slouchcat.core.browse.audio.PodcastEpisodesViewModel
import com.slouchcat.core.browse.item.ItemEvents
import com.slouchcat.core.browse.item.ItemUiState
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.strings.R

/**
 * The detail page of an audiobook, a podcast or an episode inside `ItemFeature` (stage 4): its
 * links to the audio player and the series, and for a podcast the episodes
 * ([PodcastEpisodesViewModel], read again quietly when the page is in front again).
 */
@Composable
fun AudioItemPage(graph: TvGraph, navigator: TvNavigator, state: ItemUiState, events: ItemEvents, images: ServerImages) {
    val detail = state.detail
    val episodes = if (detail?.kind == ItemKind.PODCAST) {
        val vm = viewModel(key = "episodes-${detail.id}") { PodcastEpisodesViewModel(graph.session, graph.media.audio, graph.media.detail, detail.id) }
        val paged by vm.state.collectAsStateWithLifecycle()
        val busy by vm.busy.collectAsStateWithLifecycle()
        LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.onShown() }
        LaunchedEffect(vm) {
            vm.messages.collect { message -> if (message == EpisodesMessage.Failed) graph.toasts.show(R.string.app_audio_failed) }
        }
        EpisodesUi(paged, busy, vm::onNearEnd, vm::retry, vm::onHeard)
    } else {
        null
    }
    val links = remember(navigator) {
        val player = navigator.canOpen(ListenRoute::class)
        AudioItemLinks(
            play = if (player) ({ id -> navigator.navigate(ListenRoute(id)) }) else null,
            playAt = if (player) ({ id, startMs -> navigator.navigate(ListenRoute(id, startAtMs = startMs)) }) else null,
            item = if (navigator.canOpen(ItemRoute::class)) ({ id -> navigator.navigate(ItemRoute(id)) }) else null,
            series = if (navigator.canOpen(AudioSeriesRoute::class)) ({ library, series -> navigator.navigate(AudioSeriesRoute(library, series)) }) else null,
        )
    }
    AudioItemScreen(state, images, links, events, episodes)
}
