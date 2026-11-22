package com.slouchcat.core.media

import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** One page of a list of the server (`ItemPage`, `PicksPage`, `LibraryItemsPage`): its entries and the size of the whole list. */
data class Page<T>(val items: List<T>, val total: Int)

/** The state of the next page of a [Paged] list. */
enum class MoreState {
    /** Nothing loading; [Paged.hasMore] says whether there is more. */
    Idle,
    Loading,

    /** The last page failed: the list shows «Erneut versuchen» at its end. */
    Failed,
}

/**
 * A list the server sends in pages: the first page as a [Loadable] (loading, empty, failed, there,
 * drawn by `LoadableContent`), the further pages appended, [total] from the server.
 */
data class Paged<T>(
    val items: Loadable<List<T>> = Loadable.Loading,
    val total: Int = 0,
    val more: MoreState = MoreState.Idle,
) {
    val loaded: List<T> get() = (items as? Loadable.Ready)?.value.orEmpty()

    /** The server has more than the list holds. */
    val hasMore: Boolean get() = items is Loadable.Ready && loaded.size < total
}

/**
 * The pager of the phone app (decided in stage 4 instead of Paging 3: no further library, the
 * same `Loadable` as every page, and a ViewModel test needs no paging test kit). A ViewModel holds
 * one per list, in its own scope:
 *
 * ```
 * private val pager = PageLoader(viewModelScope) { offset, limit -> repo.homeRow(server, key, offset, limit).map { Page(it.items, it.total) } }
 * val items: StateFlow<Paged<ItemRef>> = pager.state
 * init { pager.refresh() }
 * fun onNearEnd() = pager.loadMore()
 * ```
 * [refresh] starts again at offset 0 (a filter changed, pull to refresh) and cancels a page on its
 * way; [loadMore] asks for the next page once, only while there is more and nothing loads, and is
 * safe to call on every scroll near the end. [map] turns the entries of the server into what the
 * page shows (an `ItemRef` into a `MediaItem`).
 */
class PageLoader<S, T>(
    private val scope: CoroutineScope,
    private val pageSize: Int = PAGE_SIZE,
    private val map: (S) -> T,
    /**
     * What makes two entries the same title (the id): a list that shifts between two pages on the
     * server (a title added, sorted by «zuletzt gesehen») sends one twice; the second goes, else a
     * grid with keys would crash (code review M-1). The default compares the entries themselves.
     */
    private val key: (T) -> Any? = { it },
    private val load: suspend (offset: Int, limit: Int) -> ApiResult<Page<S>>,
) {
    private val _state = MutableStateFlow(Paged<T>())
    val state: StateFlow<Paged<T>> = _state.asStateFlow()
    private var job: Job? = null

    /** Entries of the server left out as doubles: the next page starts after them. */
    private var dropped = 0

    /** Loads the first page again; [keep]: leave the shown entries while it loads (pull to refresh). */
    fun refresh(keep: Boolean = false) {
        job?.cancel()
        if (!keep || _state.value.items !is Loadable.Ready) _state.value = Paged()
        job = scope.launch {
            val result = load(0, pageSize)
            val page = (result as? ApiResult.Ok)?.value
            if (page != null) {
                val mapped = page.items.map(map)
                val items = mapped.distinctBy(key)
                dropped = mapped.size - items.size
                _state.value = Paged(if (items.isEmpty()) Loadable.Empty else Loadable.Ready(items), page.total - dropped)
            } else if (!(keep && _state.value.items is Loadable.Ready)) {
                _state.value = Paged(result.toLoadable().let { it as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) })
            } else {
                // The shown list stays; a page that was loading when the refresh cancelled it is not (code review L-2).
                _state.update { it.copy(more = MoreState.Idle) }
            }
        }
    }

    /** The next page, when there is one and none loads. */
    fun loadMore() {
        val now = _state.value
        if (!now.hasMore || now.more == MoreState.Loading || job?.isActive == true) return
        _state.update { it.copy(more = MoreState.Loading) }
        val offset = now.loaded.size + dropped
        job = scope.launch {
            when (val result = load(offset, pageSize)) {
                is ApiResult.Ok -> _state.update {
                    val known = it.loaded.mapTo(HashSet(), key)
                    val mapped = result.value.items.map(map)
                    val fresh = mapped.filter { item -> known.add(key(item)) }
                    dropped += mapped.size - fresh.size
                    val items = it.loaded + fresh
                    // A page with nothing new ends the list, else it would ask again and again.
                    val total = if (fresh.isEmpty()) items.size else result.value.total - dropped
                    Paged(Loadable.Ready(items), total, MoreState.Idle)
                }
                else -> _state.update { it.copy(more = MoreState.Failed) }
            }
        }
    }

    companion object {
        /** Entries per request: the default of the server's lists. */
        const val PAGE_SIZE = 50
    }
}
