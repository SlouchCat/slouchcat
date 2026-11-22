package com.slouchcat.core.browse.audio

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.PageLoader
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.data.AudioRepository
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.media.watchedMark
import com.slouchcat.core.media.withWatched
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.OffsetDateTime

/**
 * The series of an audiobook (web `SeriesPage`, route `/libraries/$libraryId/series/$seriesId`): the
 * volumes in their order, the length, the volume to listen to next. 404 when the profile sees fewer
 * than two volumes (the page says «Reihe nicht gefunden»). Back from the player it reads the series
 * again quietly ([onShown]).
 */
class AudioSeriesViewModel(
    private val session: SessionManager,
    private val repository: AudioRepository,
    private val libraryId: Long,
    private val seriesId: String,
) : ViewModel() {
    private val _state = MutableStateFlow<Loadable<AudioSeries>>(Loadable.Loading)
    val state: StateFlow<Loadable<AudioSeries>> = _state.asStateFlow()

    private var job: Job? = null
    private var shown = false

    init {
        load(quiet = false)
    }

    fun retry() = load(quiet = false)

    /** The page is in front again: the progress of the volumes changed while the player played. */
    fun onShown() {
        if (shown) load(quiet = true) else shown = true
    }

    private fun load(quiet: Boolean) {
        val server = session.server ?: return
        job?.cancel()
        if (!quiet) _state.value = Loadable.Loading
        job = viewModelScope.launch {
            val loaded = repository.series(server, libraryId, seriesId).toLoadable { it.volumes.isEmpty() }
            if (quiet && loaded !is Loadable.Ready) return@launch
            _state.value = loaded
        }
    }
}

/** A short message of the episode list. */
enum class EpisodesMessage { Failed }

/**
 * The episodes of a podcast (web `PodcastEpisodes`): newest first, in pages of
 * `listPodcastEpisodes`, each with its own check mark «gehört» that changes at once and goes back
 * when the server refuses. [onShown] reads the first page again quietly (the progress after the
 * player).
 */
class PodcastEpisodesViewModel(
    private val session: SessionManager,
    private val audio: AudioRepository,
    private val detail: DetailRepository,
    private val podcastId: String,
    private val now: () -> OffsetDateTime = OffsetDateTime::now,
) : ViewModel() {
    private val pager = PageLoader<ItemRef, ItemRef>(viewModelScope, map = { it }, key = { it.id }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        audio.episodes(server, podcastId, offset, limit)
    }

    /** Check marks changed on this page, until the list is read again. */
    private val changed = MutableStateFlow<Map<String, ItemRef>>(emptyMap())
    private val _busy = MutableStateFlow<Set<String>>(emptySet())

    /** The episodes waiting for the server's answer to a change of their check mark. */
    val busy: StateFlow<Set<String>> = _busy.asStateFlow()

    val state: StateFlow<Paged<ItemRef>> = combine(pager.state, changed) { paged, patches ->
        if (patches.isEmpty()) paged else paged.copy(items = (paged.items as? Loadable.Ready)?.let { Loadable.Ready(it.value.map { e -> patches[e.id] ?: e }) } ?: paged.items)
    }.stateIn(viewModelScope, SharingStarted.Eagerly, Paged())

    private val _messages = Channel<EpisodesMessage>(Channel.BUFFERED)
    val messages: Flow<EpisodesMessage> = _messages.receiveAsFlow()

    private var shown = false

    init {
        pager.refresh()
    }

    fun retry() = pager.refresh()

    fun onNearEnd() = pager.loadMore()

    fun onShown() {
        if (!shown) {
            shown = true
            return
        }
        changed.value = emptyMap()
        pager.refresh(keep = true)
    }

    /** The check mark «gehört» of [episode]. */
    fun onHeard(episode: ItemRef) {
        val server = session.server ?: return
        if (episode.id in _busy.value) return
        val current = changed.value[episode.id] ?: episode
        val on = current.watchedMark != WatchedMark.Full
        _busy.update { it + episode.id }
        changed.update { it + (episode.id to current.withWatched(on, now())) }
        viewModelScope.launch {
            val result = detail.setWatched(server, episode.id, on)
            _busy.update { it - episode.id }
            if (result !is ApiResult.Ok) {
                changed.update { it + (episode.id to current) }
                _messages.send(EpisodesMessage.Failed)
            }
        }
    }
}
