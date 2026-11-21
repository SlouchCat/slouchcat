package com.slouchcat.app.mobile.item

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import okhttp3.HttpUrl
import java.time.OffsetDateTime

/** A change of the page that waits for the server; a second press waits too. */
enum class ItemAction { Watchlist, Watched, Recommend, SeasonWatched }

/** A question in a sheet before a change. */
sealed interface ItemAsk {
    /**
     * Clearing the check mark of a series, season or podcast, or a half one (web `WatchedChoice`):
     * [half] offers «alle markieren» beside «alle Haken entfernen»; [count] episodes carry it.
     */
    data class Watched(val title: String, val audio: Boolean, val count: Int, val half: Boolean, val season: Boolean) : ItemAsk

    /** «Empfehlen» or taking the own recommendation back (`recommendItem`, `unrecommendItem`). */
    data object Recommend : ItemAsk
}

/** A short message after a change. */
sealed interface ItemMessage {
    data object Failed : ItemMessage

    data class Recommended(val days: Int) : ItemMessage

    data object Unrecommended : ItemMessage
}

data class ItemUiState(
    val item: Loadable<ItemDetail> = Loadable.Loading,
    /** The YouTube id of the first trailer the server names; `null`: no trailer button. */
    val trailer: String? = null,
    /** The season whose episodes show (series). */
    val seasonId: String? = null,
    /** The seasons asked for so far (`getItem` of the season with its episodes). */
    val seasons: Map<String, Loadable<ItemDetail>> = emptyMap(),
    val busy: Set<ItemAction> = emptySet(),
    val ask: ItemAsk? = null,
    /** «Mehr anzeigen» of the summary; closed from the start (Marco's rule: foldables are closed). */
    val overviewOpen: Boolean = false,
    /** «Datei und Technik», closed from the start. */
    val techOpen: Boolean = false,
    /** The version of the file the tech section shows; `null`: the server's first (the primary). */
    val versionId: String? = null,
) {
    val detail: ItemDetail? get() = (item as? Loadable.Ready)?.value

    /** The episodes the page lists: those of the chosen season, or a season's own. */
    val episodes: Loadable<ItemDetail>?
        get() {
            val d = detail ?: return null
            if (d.kind == ItemKind.SEASON) return item
            return seasonId?.let { seasons[it] ?: Loadable.Loading }
        }
}

/** What the detail page can be asked to do; the ViewModel does it, tests record it. */
interface ItemEvents {
    fun retry()
    fun onWatchlist()
    fun onWatched()
    fun onSeasonWatched()
    fun onSeason(id: String)
    fun onAskChoice(watched: Boolean)
    fun onRecommendAsk()
    fun onRecommendConfirm()
    fun onDismissAsk()
    fun onToggleOverview()
    fun onToggleTech()
    fun onVersion(id: String)
}

/**
 * The detail page of a title (mockup `/item/:i`, web `ItemPage`): `getItem`, the trailers, the
 * episodes of the chosen season; Merken, Gesehen and Empfehlen change at once and go back when
 * the server refuses. After a change of a check mark the page reads the title again in place
 * (no spinner), once no other change waits.
 */
class ItemViewModel(
    private val session: SessionManager,
    private val repository: DetailRepository,
    private val id: String,
    private val now: () -> OffsetDateTime = OffsetDateTime::now,
) : ViewModel(), ItemEvents {
    private val _state = MutableStateFlow(ItemUiState())
    val state: StateFlow<ItemUiState> = _state.asStateFlow()

    private val _messages = Channel<ItemMessage>(Channel.BUFFERED)
    val messages: Flow<ItemMessage> = _messages.receiveAsFlow()

    private var loading: Job? = null

    init {
        load()
    }

    override fun retry() = load()

    private fun load(quiet: Boolean = false) {
        val server = session.server ?: return
        loading?.cancel()
        if (!quiet) _state.update { it.copy(item = Loadable.Loading) }
        loading = viewModelScope.launch {
            val loaded = repository.item(server, id).toLoadable()
            if (quiet && loaded !is Loadable.Ready) return@launch
            _state.update { it.copy(item = loaded) }
            val detail = (loaded as? Loadable.Ready)?.value ?: return@launch
            chooseSeason(detail, quiet)
            if (!quiet && (detail.trailerCount ?: 0) > 0) loadTrailer(detail.id)
        }
    }

    private fun loadTrailer(item: String) {
        val server = session.server ?: return
        viewModelScope.launch {
            val first = (repository.trailers(server, item) as? ApiResult.Ok)?.value?.firstOrNull()
            // Only a well-formed id goes into the address of YouTube.
            _state.update { it.copy(trailer = first?.youtubeId?.takeIf { y -> YOUTUBE_ID.matches(y) }) }
        }
    }

    /** The season of the next episode, else the first; a quiet reload reads the shown one again. */
    private fun chooseSeason(detail: ItemDetail, quiet: Boolean) {
        if (detail.kind != ItemKind.SERIES) return
        val seasons = detail.seasons.orEmpty()
        val current = _state.value.seasonId?.takeIf { s -> seasons.any { it.id == s } }
        val chosen = current
            ?: seasons.firstOrNull { it.id == detail.nextUp?.episode?.parentId }?.id
            ?: seasons.firstOrNull()?.id
            ?: return
        _state.update { it.copy(seasonId = chosen) }
        loadSeason(chosen, quiet)
    }

    private fun loadSeason(season: String, quiet: Boolean) {
        val server = session.server ?: return
        if (!quiet) _state.update { it.copy(seasons = it.seasons + (season to Loadable.Loading)) }
        viewModelScope.launch {
            val loaded = repository.item(server, season).toLoadable()
            _state.update {
                if (quiet && loaded !is Loadable.Ready && it.seasons[season] is Loadable.Ready) it else it.copy(seasons = it.seasons + (season to loaded))
            }
        }
    }

    override fun onSeason(id: String) {
        _state.update { it.copy(seasonId = id) }
        if (_state.value.seasons[id] !is Loadable.Ready) loadSeason(id, quiet = false)
    }

    override fun onToggleOverview() = _state.update { it.copy(overviewOpen = !it.overviewOpen) }

    override fun onToggleTech() = _state.update { it.copy(techOpen = !it.techOpen) }

    override fun onVersion(id: String) = _state.update { it.copy(versionId = id) }

    override fun onDismissAsk() = _state.update { it.copy(ask = null) }

    /** Runs [call] for [action] after [patch]; puts back what [patch] changed when it fails. */
    private fun <T> change(
        action: ItemAction,
        patch: (ItemUiState) -> ItemUiState,
        undo: (ItemUiState) -> ItemUiState,
        call: suspend (HttpUrl) -> ApiResult<T>,
        reload: Boolean,
        done: suspend (T) -> Unit = {},
    ) {
        val server = session.server ?: return
        if (action in _state.value.busy) return
        _state.update { patch(it).copy(busy = it.busy + action) }
        viewModelScope.launch {
            val result = call(server)
            _state.update { s -> (if (result is ApiResult.Ok) s else undo(s)).copy(busy = s.busy - action) }
            if (result is ApiResult.Ok) done(result.value) else _messages.send(ItemMessage.Failed)
            // The title and the shown season again, both stay on the page meanwhile.
            if (reload && _state.value.busy.isEmpty()) load(quiet = true)
        }
    }

    private fun mapDetail(state: ItemUiState, f: (ItemDetail) -> ItemDetail): ItemUiState =
        state.detail?.let { state.copy(item = Loadable.Ready(f(it))) } ?: state

    override fun onWatchlist() {
        val detail = _state.value.detail ?: return
        val on = !detail.inWatchlist
        change(
            ItemAction.Watchlist,
            patch = { s -> mapDetail(s) { it.copy(inWatchlist = on) } },
            undo = { s -> mapDetail(s) { it.copy(inWatchlist = !on) } },
            call = { repository.setWatchlist(it, detail.id, on) },
            reload = false,
        )
    }

    override fun onWatched() {
        val detail = _state.value.detail ?: return
        if (ItemAction.Watched in _state.value.busy) return
        when (detail.watchedMark) {
            WatchedMark.None -> setWatched(true)
            WatchedMark.Full -> if (isGroup(detail.kind)) ask(detail.title, detail.kind, detail.watchedCount ?: 0, half = false, season = false) else setWatched(false)
            WatchedMark.Half -> ask(detail.title, detail.kind, detail.watchedCount ?: 0, half = true, season = false)
        }
    }

    override fun onSeasonWatched() {
        val season = currentSeason() ?: return
        if (ItemAction.SeasonWatched in _state.value.busy) return
        when (season.watchedMark) {
            WatchedMark.None -> setSeasonWatched(true)
            WatchedMark.Full -> ask(season.title, season.kind, season.watchedCount ?: 0, half = false, season = true)
            WatchedMark.Half -> ask(season.title, season.kind, season.watchedCount ?: 0, half = true, season = true)
        }
    }

    private fun ask(title: String, kind: ItemKind, count: Int, half: Boolean, season: Boolean) =
        _state.update { it.copy(ask = ItemAsk.Watched(title, isAudio(kind), count, half, season)) }

    override fun onAskChoice(watched: Boolean) {
        val ask = _state.value.ask as? ItemAsk.Watched ?: return
        _state.update { it.copy(ask = null) }
        if (ask.season) setSeasonWatched(watched) else setWatched(watched)
    }

    private fun setWatched(on: Boolean) {
        val detail = _state.value.detail ?: return
        val before = detail
        val time = now()
        change(
            ItemAction.Watched,
            patch = { s -> mapDetail(s) { it.withWatched(on, time) } },
            undo = { s -> mapDetail(s) { before } },
            call = { repository.setWatched(it, detail.id, on) },
            reload = true,
        )
    }

    /** The season shown under a series (its reference in the series' list). */
    private fun currentSeason(): ItemRef? {
        val s = _state.value
        return s.detail?.seasons?.firstOrNull { it.id == s.seasonId }
    }

    private fun setSeasonWatched(on: Boolean) {
        val season = currentSeason() ?: return
        val time = now()
        val beforeSeason = _state.value.seasons[season.id]
        val beforeRefs = _state.value.detail?.seasons
        change(
            ItemAction.SeasonWatched,
            patch = { s ->
                val episodes = (s.seasons[season.id] as? Loadable.Ready)?.value?.withWatched(on, time)
                mapDetail(s) { d -> d.copy(seasons = d.seasons?.map { if (it.id == season.id) it.withWatched(on, time) else it }) }
                    .let { if (episodes != null) it.copy(seasons = it.seasons + (season.id to Loadable.Ready(episodes))) else it }
            },
            undo = { s ->
                mapDetail(s) { it.copy(seasons = beforeRefs) }
                    .let { if (beforeSeason != null) it.copy(seasons = it.seasons + (season.id to beforeSeason)) else it }
            },
            call = { repository.setWatched(it, season.id, on) },
            reload = true,
        )
    }

    override fun onRecommendAsk() {
        if (_state.value.detail?.canRecommend == true) _state.update { it.copy(ask = ItemAsk.Recommend) }
    }

    override fun onRecommendConfirm() {
        val detail = _state.value.detail ?: return
        val on = !detail.recommendedByMe
        _state.update { it.copy(ask = null) }
        if (on) {
            change(
                ItemAction.Recommend,
                patch = { s -> mapDetail(s) { it.copy(recommendedByMe = true) } },
                undo = { s -> mapDetail(s) { it.copy(recommendedByMe = false) } },
                call = { repository.recommend(it, detail.id) },
                reload = false,
                done = { pick -> _messages.send(ItemMessage.Recommended(pick.retentionDays)) },
            )
        } else {
            change(
                ItemAction.Recommend,
                patch = { s -> mapDetail(s) { it.copy(recommendedByMe = false) } },
                undo = { s -> mapDetail(s) { it.copy(recommendedByMe = true) } },
                call = { repository.unrecommend(it, detail.id) },
                reload = false,
                done = { _messages.send(ItemMessage.Unrecommended) },
            )
        }
    }

    private companion object {
        val YOUTUBE_ID = Regex("^[A-Za-z0-9_-]{6,20}$")
    }
}
