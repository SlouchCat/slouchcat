package com.slouchcat.app.mobile.lists

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.browse.ListRoute
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.PageLoader
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.HomeRepository
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.flow.StateFlow

/** Which list a [ListRoute] key names: a row of the start page or «User-Empfehlungen». */
sealed interface ListKind {
    data class Row(val key: HomeListKey) : ListKind

    data object Picks : ListKind

    companion object {
        /** `null` for a key the app does not know: the page says the list is not there. */
        fun of(key: String): ListKind? = when (key) {
            ListRoute.PICKS -> Picks
            else -> HomeListKey.entries.firstOrNull { it != HomeListKey.UNKNOWN_DEFAULT_OPEN_API && it.value == key }?.let(::Row)
        }
    }
}

/**
 * «Alle anzeigen» of a row of the start page and «User-Empfehlungen» (mockup `/list/:key`; web
 * `HomeListPage`, `PicksPage`): the whole list in pages.
 */
class ListViewModel(
    private val session: SessionManager,
    private val repository: HomeRepository,
    key: String,
) : ViewModel() {
    val kind: ListKind? = ListKind.of(key)

    private val pager = PageLoader<ItemRef, MediaItem>(viewModelScope, map = { MediaItem.from(it, ServerImages(requireNotNull(session.server))) }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        when (val k = kind) {
            is ListKind.Row -> repository.homeRow(server, k.key, offset, limit)
            ListKind.Picks -> repository.picks(server, offset, limit)
            null -> ApiResult.NotFound
        }
    }

    val state: StateFlow<Paged<MediaItem>> = pager.state

    init {
        pager.refresh()
    }

    fun retry() = pager.refresh()

    fun onNearEnd() = pager.loadMore()
}
