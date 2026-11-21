package com.slouchcat.app.tv.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRow
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.HomeRepository
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.SessionManager
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/*
 * The start page of the TV app: the ViewModel of the phone app (`app-mobile/.../home`) with the
 * summary of the stage added; the two are kept alike until they move to a shared module.
 */

/** One library of the row «Meine Medien» (`LibraryTile`). */
data class LibraryChip(val id: Long, val name: String, val kind: LibraryKind, val seriesOnly: Boolean, val count: Int)

/** A row of the start page as the screen draws it, in the order of the server (`getHome`). */
sealed interface HomeSection {
    /** The row's identity (`HomeRow.id`). */
    val id: String

    /** The stage (`heroRow`): backdrop, logo or title, the facts, «Abspielen», «Merken», info; [start] the server's draw. */
    data class Spotlight(
        override val id: String,
        val items: List<MediaItem>,
        val start: Int,
        val intervalSeconds: Int,
        /** The summary of each title (`ItemRef.overview`), the TV stage shows two lines of it. */
        val overviews: List<String?> = emptyList(),
    ) : HomeSection

    /**
     * A row of title cards. [list]: the list behind its title (`listHomeRow`) or [picks]; [libraryId]:
     * the library of a row «Neu in …». [more]: the state of the next titles at its end.
     */
    data class Cards(
        override val id: String,
        val key: HomeRowKey,
        val items: List<MediaItem>,
        val libraryId: Long? = null,
        val libraryName: String? = null,
        val list: HomeListKey? = null,
        val picks: Boolean = false,
        val more: MoreState = MoreState.Idle,
        val complete: Boolean = true,
    ) : HomeSection

    /** «Sammlungen»: a card per collection, its count as caption. */
    data class Collections(override val id: String, val collections: List<Pair<CollectionRef, MediaItem>>) : HomeSection

    /** «Genres»: each opens the search filtered by it. */
    data class Genres(override val id: String, val genres: List<GenreRef>) : HomeSection

    /** «Meine Medien»: the libraries of the profile. */
    data class Libraries(override val id: String, val libraries: List<LibraryChip>) : HomeSection
}

data class HomeUiState(
    val rows: Loadable<List<HomeSection>> = Loadable.Loading,
    val refreshing: Boolean = false,
    val profile: ActiveProfile? = null,
)

/** What the start page says once (a snackbar). */
sealed interface HomeMessage {
    data class Watchlist(val title: String, val added: Boolean) : HomeMessage

    data object WatchlistFailed : HomeMessage
}

/**
 * The start page (mockup `/`, `home.js`; web `HomePage`): the rows of `getHome` in the server's
 * order, the stage the server names, more titles at the end of a row (`listHomeRow`, `listPicks`),
 * pull to refresh, «Merken» on the stage. A new profile (or the same chosen again) loads afresh;
 * the shell starts a new ViewModel with every session epoch anyway.
 */
class HomeViewModel(
    private val session: SessionManager,
    private val home: HomeRepository,
    private val items: ItemRepository,
    /** «Merkliste» of the shell: «Merken» on the stage changes it (code review m-1). */
    private val watchlistCount: LiveCount? = null,
) : ViewModel() {
    private val _state = MutableStateFlow(HomeUiState())
    val state: StateFlow<HomeUiState> = _state.asStateFlow()
    private val _messages = Channel<HomeMessage>(Channel.BUFFERED)
    val messages: Flow<HomeMessage> = _messages.receiveAsFlow()
    private var loading: Job? = null
    private val growing = mutableMapOf<String, Job>()

    init {
        viewModelScope.launch {
            var shown: Long? = null
            session.state.map { (it as? SessionState.Ready)?.profile }.distinctUntilChanged().collect { profile ->
                // Another profile loads afresh; a change of the same profile (its colour) only redraws.
                if (profile?.id == shown) {
                    _state.update { it.copy(profile = profile) }
                    return@collect
                }
                shown = profile?.id
                _state.update { HomeUiState(profile = profile) }
                if (profile != null) load()
            }
        }
    }

    fun retry() = load()

    /** Pull to refresh: the rows stay while the new ones load, and when that fails. */
    fun refresh() = load(refresh = true)

    private fun load(refresh: Boolean = false) {
        val server = session.server ?: return
        loading?.cancel()
        growing.values.forEach { it.cancel() }
        growing.clear()
        _state.update { if (refresh && it.rows is Loadable.Ready) it.copy(refreshing = true) else it.copy(rows = Loadable.Loading, refreshing = refresh) }
        loading = viewModelScope.launch {
            val result = home.home(server)
            val images = ServerImages(server)
            _state.update { now ->
                val rows: Loadable<List<HomeSection>> = when (result) {
                    is ApiResult.Ok -> sections(result.value, images).let { if (it.isEmpty()) Loadable.Empty else Loadable.Ready(it) }
                    else -> result.toLoadable().let { it as? Loadable.Failed ?: Loadable.Empty }
                }
                if (refresh && rows is Loadable.Failed && now.rows is Loadable.Ready) now.copy(refreshing = false) else now.copy(rows = rows, refreshing = false)
            }
        }
    }

    /** The end of a row came into view: its next titles, when the row has a list behind it and more there. */
    fun onRowEnd(id: String) {
        val server = session.server ?: return
        val row = cards(id) ?: return
        if (row.complete || row.more == MoreState.Loading || growing[id]?.isActive == true) return
        val key = row.list
        if (key == null && !row.picks) return
        setRow(id) { it.copy(more = MoreState.Loading) }
        growing[id] = viewModelScope.launch {
            val offset = row.items.size
            val result: ApiResult<Page<ItemRef>> =
                if (key != null) home.homeRow(server, key, offset, GROW) else home.picks(server, offset, GROW)
            val images = ServerImages(server)
            setRow(id) { now ->
                when (result) {
                    is ApiResult.Ok -> {
                        val known = now.items.mapTo(mutableSetOf()) { it.id }
                        val added = result.value.items.filter { it.id !in known }.map { MediaItem.from(it, images) }
                        val all = now.items + added
                        now.copy(items = all, more = MoreState.Idle, complete = added.isEmpty() || all.size >= result.value.total)
                    }
                    else -> now.copy(more = MoreState.Failed)
                }
            }
        }
    }

    /** «Merken» of a title of the stage; the stage shows the new state at once and returns on a failure. */
    fun onWatchlist(item: MediaItem) {
        val server = session.server ?: return
        val on = !item.inWatchlist
        mark(item.id, on)
        viewModelScope.launch {
            when (items.setWatchlist(server, item.id, on)) {
                is ApiResult.Ok -> {
                    watchlistCount?.adjust(if (on) 1 else -1)
                    _messages.send(HomeMessage.Watchlist(item.title, on))
                }
                else -> {
                    mark(item.id, !on)
                    _messages.send(HomeMessage.WatchlistFailed)
                }
            }
        }
    }

    private fun mark(id: String, on: Boolean) = _state.update { now ->
        val rows = (now.rows as? Loadable.Ready)?.value ?: return@update now
        now.copy(
            rows = Loadable.Ready(
                rows.map { row ->
                    when (row) {
                        is HomeSection.Spotlight -> row.copy(items = row.items.map { if (it.id == id) it.copy(inWatchlist = on) else it })
                        is HomeSection.Cards -> row.copy(items = row.items.map { if (it.id == id) it.copy(inWatchlist = on) else it })
                        else -> row
                    }
                },
            ),
        )
    }

    private fun cards(id: String): HomeSection.Cards? =
        ((_state.value.rows as? Loadable.Ready)?.value)?.firstOrNull { it.id == id } as? HomeSection.Cards

    private fun setRow(id: String, change: (HomeSection.Cards) -> HomeSection.Cards) = _state.update { now ->
        val rows = (now.rows as? Loadable.Ready)?.value ?: return@update now
        now.copy(rows = Loadable.Ready(rows.map { if (it.id == id && it is HomeSection.Cards) change(it) else it }))
    }

    private companion object {
        /** Titles a row of the start page holds at most (`getHome`: «at most 20»). */
        const val ROW_CUT = 20
        const val GROW = 50

        fun sections(page: HomePage, images: ServerImages): List<HomeSection> = page.rows.mapNotNull { row -> section(row, page, images) }

        fun section(row: HomeRow, page: HomePage, images: ServerImages): HomeSection? {
            val cards = row.items.map { MediaItem.from(it, images) }
            if (row.key == page.heroRow && cards.isNotEmpty()) {
                return HomeSection.Spotlight(row.id, cards, (page.heroStart ?: 0).coerceIn(0, cards.lastIndex), page.heroIntervalSeconds ?: DEFAULT_INTERVAL, row.items.map { it.overview })
            }
            return when (row.key) {
                HomeRowKey.COLLECTIONS -> row.collections.orEmpty().takeIf { it.isNotEmpty() }?.let { list ->
                    HomeSection.Collections(row.id, list.map { it to collectionCard(it, images) })
                }
                HomeRowKey.GENRES -> row.genres.orEmpty().takeIf { it.isNotEmpty() }?.let { HomeSection.Genres(row.id, it) }
                HomeRowKey.LIBRARIES -> row.libraries.orEmpty().takeIf { it.isNotEmpty() }?.let { tiles ->
                    HomeSection.Libraries(row.id, tiles.map { t -> LibraryChip(t.id, t.name, t.kind, seriesOnly = t.items.isNotEmpty() && t.items.all { it.kind == ItemKind.SERIES }, count = t.itemCount) })
                }
                // «Meine Sender» plays through the radio of stage 7; until then the row is left out.
                HomeRowKey.RADIO, HomeRowKey.UNKNOWN_DEFAULT_OPEN_API -> null
                else -> cards.takeIf { it.isNotEmpty() }?.let {
                    val list = HomeListKey.entries.firstOrNull { k -> k != HomeListKey.UNKNOWN_DEFAULT_OPEN_API && k.value == row.key.value }
                    val picks = row.key == HomeRowKey.PICKS
                    HomeSection.Cards(
                        id = row.id,
                        key = row.key,
                        items = it,
                        libraryId = row.libraryId,
                        libraryName = row.libraryName,
                        list = list,
                        picks = picks,
                        // Only a row cut by the server has more, and only with a list behind it.
                        complete = it.size < ROW_CUT || (list == null && !picks),
                    )
                }
            }
        }

        private fun collectionCard(collection: CollectionRef, images: ServerImages): MediaItem {
            val image = images.single(collection.image)
            val cover = collection.cover?.let { MediaItem.from(it, images) }
            return MediaItem(
                id = collection.id,
                kind = ItemKind.MOVIE,
                title = collection.name,
                poster = image.or(cover?.poster ?: image),
            )
        }

        private const val DEFAULT_INTERVAL = 7
    }
}
