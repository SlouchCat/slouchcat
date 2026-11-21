package com.slouchcat.app.mobile.mylists

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.MoreState
import com.slouchcat.app.mobile.media.PageLoader
import com.slouchcat.app.mobile.media.Paged
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.WatchedEntry
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.OffsetDateTime

/** What a list page tells once (a toast): a title left the list, or a change failed. */
sealed interface ListMessage {
    /** Off the watchlist; «Rückgängig» puts it back. */
    data class Removed(val id: String, val title: String) : ListMessage

    /** The check mark went (no way back: dates and episodes are gone, as in the web app). */
    data class Unwatched(val title: String) : ListMessage

    data object Failed : ListMessage
}

/** A page of «Merkliste» or «Gesehen»: the entries, those taken off meanwhile, the mode «Bearbeiten». */
data class MyListUiState<T>(
    val entries: Paged<T> = Paged(),
    /** Entries taken off on this page; they stay hidden until the list is read again. */
    val removed: Set<String> = emptySet(),
    /** «Bearbeiten»: every card shows its ✕. */
    val editing: Boolean = false,
    /** Entries whose change is on its way (their ✕ waits). */
    val busy: Set<String> = emptySet(),
    /** «Gesehen»: the entry whose check mark the page asks about. */
    val asking: T? = null,
) {
    /** The entries shown, as [Paged.items] without those taken off ([Loadable.Empty] when none is left). */
    fun shown(id: (T) -> String): Loadable<List<T>> = when (val items = entries.items) {
        is Loadable.Ready -> items.value.filterNot { id(it) in removed }.let { if (it.isEmpty()) Loadable.Empty else Loadable.Ready(it) }
        else -> items
    }

    val total: Int get() = (entries.total - removed.size).coerceAtLeast(0)
}

/**
 * «Merkliste» (mockup `/watchlist`, web `WatchlistPage`): `listWatchlist` page by page, newest
 * first; «Bearbeiten» shows a ✕ on each card, which takes the title off (`removeFromWatchlist`) with
 * «Rückgängig» in the toast (`addToWatchlist`).
 */
class WatchlistViewModel(
    private val session: SessionManager,
    private val repository: MyListsRepository,
    private val count: WatchlistCount,
) : ViewModel() {
    private val local = MutableStateFlow(MyListUiState<MediaItem>())
    private val pager = PageLoader(viewModelScope, map = { it: com.slouchcat.core.api.generated.models.ItemRef -> MediaItem.from(it, ServerImages(session.server!!)) }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        repository.watchlist(server, offset, limit).also { result -> if (offset == 0 && result is ApiResult.Ok) count.set(result.value.total) }
    }
    val state: StateFlow<MyListUiState<MediaItem>> =
        combine(pager.state, local) { entries, own -> own.copy(entries = entries) }.stateIn(viewModelScope, SharingStarted.Eagerly, MyListUiState())

    private val _messages = Channel<ListMessage>(Channel.BUFFERED)
    val messages: Flow<ListMessage> = _messages.receiveAsFlow()

    init {
        pager.refresh()
    }

    fun retry() = pager.refresh()

    fun onMore() = pager.loadMore()

    fun onToggleEdit() = local.update { it.copy(editing = !it.editing) }

    fun onRemove(item: MediaItem) = change(item.id) { server ->
        when (repository.setWatchlist(server, item.id, on = false)) {
            is ApiResult.Ok -> {
                local.update { it.copy(removed = it.removed + item.id) }
                count.set(shownTotal())
                _messages.send(ListMessage.Removed(item.id, item.title))
            }
            else -> _messages.send(ListMessage.Failed)
        }
    }

    /** «Rückgängig»: back on the list (on the server it is the newest entry now). */
    fun onUndo(id: String) = change(id) { server ->
        when (repository.setWatchlist(server, id, on = true)) {
            is ApiResult.Ok -> {
                local.update { it.copy(removed = it.removed - id) }
                count.set(shownTotal())
            }
            else -> _messages.send(ListMessage.Failed)
        }
    }

    /** The number the list has now: the server's total without the titles taken off here. */
    private fun shownTotal(): Int = (pager.state.value.total - local.value.removed.size).coerceAtLeast(0)

    private fun change(id: String, block: suspend (okhttp3.HttpUrl) -> Unit) {
        val server = session.server ?: return
        if (id in local.value.busy) return
        local.update { it.copy(busy = it.busy + id) }
        viewModelScope.launch {
            try {
                block(server)
            } finally {
                local.update { it.copy(busy = it.busy - id) }
            }
        }
    }
}

/** A card of «Gesehen»: the title, when it was watched last and, for a series, how many episodes. */
data class WatchedCard(
    val item: MediaItem,
    val lastWatchedAt: OffsetDateTime,
    /** Episodes of a series or podcast; `null` for a film or an audiobook. */
    val episodeCount: Int? = null,
    val watchedCount: Int? = null,
) {
    val id: String get() = item.id

    companion object {
        fun from(entry: WatchedEntry, images: ServerImages) =
            WatchedCard(MediaItem.from(entry.item, images), entry.lastWatchedAt, entry.episodeCount, entry.item.watchedCount)
    }
}

/**
 * «Gesehen» (mockup `/watched`, web `WatchedPage`): `listWatched`, the last watched first;
 * «Bearbeiten» shows a ✕ on each card. Removing the check mark asks first, as the web app does:
 * dates and episode marks cannot come back (`setItemWatched` with `false`).
 */
class WatchedViewModel(
    private val session: SessionManager,
    private val repository: MyListsRepository,
) : ViewModel() {
    private val local = MutableStateFlow(MyListUiState<WatchedCard>())
    private val pager = PageLoader(viewModelScope, map = { it: WatchedEntry -> WatchedCard.from(it, ServerImages(session.server!!)) }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        repository.watched(server, offset, limit)
    }
    val state: StateFlow<MyListUiState<WatchedCard>> =
        combine(pager.state, local) { entries, own -> own.copy(entries = entries) }.stateIn(viewModelScope, SharingStarted.Eagerly, MyListUiState())

    private val _messages = Channel<ListMessage>(Channel.BUFFERED)
    val messages: Flow<ListMessage> = _messages.receiveAsFlow()

    init {
        pager.refresh()
    }

    fun retry() = pager.refresh()

    fun onMore() = pager.loadMore()

    fun onToggleEdit() = local.update { it.copy(editing = !it.editing) }

    /** The ✕ of a card: a series or podcast asks first (every episode loses its mark), a film goes at once. */
    fun onAsk(card: WatchedCard) {
        local.update { it.copy(asking = card) }
        if (card.episodeCount == null) onConfirm()
    }

    fun onDismiss() = local.update { it.copy(asking = null) }

    fun onConfirm() {
        val card = local.value.asking ?: return
        val server = session.server ?: return
        local.update { it.copy(asking = null, busy = it.busy + card.id) }
        viewModelScope.launch {
            when (repository.setWatched(server, card.id, watched = false)) {
                is ApiResult.Ok -> {
                    local.update { it.copy(removed = it.removed + card.id, busy = it.busy - card.id) }
                    _messages.send(ListMessage.Unwatched(card.item.title))
                }
                else -> {
                    local.update { it.copy(busy = it.busy - card.id) }
                    _messages.send(ListMessage.Failed)
                }
            }
        }
    }
}

/** Whether the next page of a list is on its way or failed (for the foot of the grid). */
val <T> MyListUiState<T>.more: MoreState get() = entries.more
