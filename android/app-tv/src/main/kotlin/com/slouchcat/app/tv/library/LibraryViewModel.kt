package com.slouchcat.app.tv.library

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.LibraryItemStatus
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.PageLoader
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.LibraryQuery
import com.slouchcat.core.media.data.LibraryRepository
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.time.Year

/*
 * A library of the TV app: the ViewModel of the phone app (`app-mobile/.../library`) without its
 * list view, the filter «Anzeigen» as a side menu of its own; kept alike until a shared module.
 */

/** A decade of the year filter (mockup «2010er», «Vor 1960»); [before] the decade is the bound below which every year counts. */
data class Decade(val start: Int, val before: Boolean = false) {
    val from: Int? get() = if (before) null else start
    val to: Int get() = if (before) start - 1 else start + 9
}

/** The filters and the order of the page (mockup chips «Alle», «Ungesehen», «Genre», «Jahr», sort sheet). */
data class LibraryFilters(
    val sort: LibrarySort = LibrarySort.ADDED,
    val unseen: Boolean = false,
    val genre: String? = null,
    val decade: Decade? = null,
    val content: RootContent? = null,
    /** The age rating filter: titles up to this minimum age (`maxRating`). */
    val maxRating: Int? = null,
) {
    val active: Boolean get() = unseen || genre != null || decade != null || content != null || maxRating != null

    fun query(): LibraryQuery = LibraryQuery(
        sort = sort,
        genre = genre,
        yearFrom = decade?.from,
        yearTo = decade?.to,
        status = if (unseen) listOf(LibraryItemStatus.UNWATCHED) else emptyList(),
        content = content,
        maxRating = maxRating,
    )
}

/** Which side menu of the page is open (mockup `brLibMenu`: «Anzeigen», «Sortierung», «Genre», «Jahr»). */
enum class LibrarySheet { Show, Sort, Genre, Year, Rating }

data class LibraryUiState(
    /** The library from `listVisibleLibraries` (name, kind, count); `null` while unknown. */
    val library: LibraryNav? = null,
    val filters: LibraryFilters = LibraryFilters(),
    val items: Paged<MediaItem> = Paged(),
    /** The genres of the library with their counts (`facets.genres`, rule 12: only what the server names). */
    val genres: List<GenreRef> = emptyList(),
    /** The age ratings present in the library below the profile's ceiling (`facets.ratings`). */
    val ratings: List<Int> = emptyList(),
    val sheet: LibrarySheet? = null,
) {
    val audio: Boolean get() = library?.kind == LibraryKind.AUDIOBOOKS || library?.kind == LibraryKind.PODCASTS
    val podcasts: Boolean get() = library?.kind == LibraryKind.PODCASTS

    /** The sort orders this library offers (mockup: the four; the web app's «Zuletzt gesehen»; «Autor» for audiobooks). */
    val sorts: List<LibrarySort>
        get() = buildList {
            add(LibrarySort.ADDED)
            add(LibrarySort.TITLE)
            if (!podcasts) add(LibrarySort.RELEASED)
            if (!audio) add(LibrarySort.VOTE)
            add(LibrarySort.PLAYED)
            if (library?.kind == LibraryKind.AUDIOBOOKS) add(LibrarySort.AUTHOR)
        }

    /** The decades of the year filter, newest first, down to [Decade.before] 1960. */
    val decades: List<Decade>
        get() {
            val newest = Year.now().value / 10 * 10
            return (newest downTo OLDEST step 10).map { Decade(it) } + Decade(OLDEST, before = true)
        }

    private companion object {
        const val OLDEST = 1960
    }
}

/**
 * A library (mockup `/library/:key`; web `LibraryPage`): the titles of `listLibraryItems` in
 * pages, filtered and sorted on the server. [initialSort]: the order the page opens with (the row
 * «Neu in …» opens it by «Zuletzt hinzugefügt»).
 */
class LibraryViewModel(
    private val session: SessionManager,
    private val repository: LibraryRepository,
    private val libraryId: Long,
    libraries: StateFlow<Loadable<List<LibraryNav>>>,
    initialSort: LibrarySort? = null,
) : ViewModel() {
    private val _state = MutableStateFlow(LibraryUiState(filters = LibraryFilters(sort = initialSort ?: LibrarySort.ADDED)))
    val state: StateFlow<LibraryUiState> = _state.asStateFlow()

    private val pager = PageLoader<ItemRef, MediaItem>(viewModelScope, map = { MediaItem.from(it, images()) }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        repository.items(server, libraryId, _state.value.filters.query(), offset, limit).map { page ->
            if (offset == 0) _state.update { it.copy(genres = page.facets.genres, ratings = page.facets.ratings) }
            Page(page.items, page.total)
        }
    }

    init {
        viewModelScope.launch {
            combine(libraries, pager.state) { list, items -> list to items }.collect { (list, items) ->
                val nav = (list as? Loadable.Ready)?.value?.firstOrNull { it.id == libraryId }
                _state.update { it.copy(library = nav ?: it.library, items = items) }
            }
        }
        pager.refresh()
    }

    private fun images() = ServerImages(requireNotNull(session.server))

    fun retry() = pager.refresh()

    fun onNearEnd() = pager.loadMore()

    fun onSheet(sheet: LibrarySheet?) = _state.update { it.copy(sheet = sheet) }

    fun onSort(sort: LibrarySort) = change { it.copy(sort = sort) }

    fun onAll() = change { it.copy(unseen = false, genre = null, decade = null, content = null, maxRating = null) }

    fun onUnseen() = change { it.copy(unseen = !it.unseen) }

    fun onGenre(genre: String?) = change { it.copy(genre = genre) }

    fun onDecade(decade: Decade?) = change { it.copy(decade = decade) }

    fun onContent(content: RootContent?) = change { it.copy(content = content) }

    fun onRating(rating: Int?) = change { it.copy(maxRating = rating) }

    private fun change(transform: (LibraryFilters) -> LibraryFilters) {
        val before = _state.value.filters
        val after = transform(before)
        _state.update { it.copy(filters = after, sheet = null) }
        if (after != before) pager.refresh()
    }
}
