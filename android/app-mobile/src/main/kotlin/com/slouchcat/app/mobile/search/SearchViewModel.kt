package com.slouchcat.app.mobile.search

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.app.mobile.common.LoadError
import com.slouchcat.app.mobile.common.Loadable
import com.slouchcat.app.mobile.common.toLoadable
import com.slouchcat.app.mobile.media.MediaItem
import com.slouchcat.app.mobile.media.MoreState
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.PersonRef
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** What a search found: titles page by page, people and collections from the first page. */
data class SearchHits(
    val items: List<MediaItem>,
    val total: Int,
    val people: List<PersonRef>,
    val collections: List<CollectionRef>,
    val more: MoreState = MoreState.Idle,
) {
    /** Everything found, the line «3 Treffer». */
    val count: Int get() = total + people.size + collections.size

    val hasMore: Boolean get() = items.size < total
}

data class SearchUiState(
    /** The text in the field, as typed. */
    val text: String = "",
    /** A genre the search is filtered by (a tile, or the genre a library page opened it with). */
    val genre: String? = null,
    /** `null`: nothing asked (no text, no genre): the page shows the ways in. */
    val hits: Loadable<SearchHits>? = null,
    /** One character typed: too short to ask the server (it answers 422). */
    val tooShort: Boolean = false,
    /** The genres of the profile for the tiles «Genres». */
    val genres: Loadable<List<GenreRef>> = Loading,
) {
    /** The text the server got (trimmed), for the highlight of the matches. */
    val query: String get() = text.trim()

    private companion object {
        val Loading: Loadable<List<GenreRef>> = Loadable.Loading
    }
}

/**
 * The search (mockup `/search`, web `SearchPage`): matches while typing, [DEBOUNCE_MS] after the
 * last key; a new text cancels the request still under way (its answer would be stale). Titles
 * come page by page, people and collections with the first page. Without a text the page offers
 * the libraries and the genres; a genre filters the search (also without a text, as the server
 * allows: `query` absent with a filter).
 */
class SearchViewModel(
    private val session: SessionManager,
    private val repository: SearchRepository,
    genre: String? = null,
) : ViewModel() {
    private val _state = MutableStateFlow(SearchUiState(genre = genre?.takeIf { it.isNotBlank() }))
    val state: StateFlow<SearchUiState> = _state.asStateFlow()

    /** The search under way: the pause before the request, then the request itself. */
    private var job: Job? = null
    private var genresJob: Job? = null

    init {
        loadGenres()
        if (_state.value.genre != null) search(debounce = false)
    }

    fun onText(text: String) {
        if (text == _state.value.text) return
        _state.update { it.copy(text = text) }
        search(debounce = true)
    }

    fun onClear() = onText("")

    /** «Suchen» on the keyboard: no need to wait for the pause. */
    fun onSubmit() = search(debounce = false)

    fun onGenre(genre: String) {
        _state.update { it.copy(genre = genre) }
        search(debounce = false)
    }

    fun onClearGenre() {
        _state.update { it.copy(genre = null) }
        search(debounce = false)
    }

    fun retry() = search(debounce = false)

    fun retryGenres() = loadGenres()

    /** The next page of titles; safe to call on every scroll near the end. */
    fun onMore() {
        val hits = (_state.value.hits as? Loadable.Ready)?.value ?: return
        if (!hits.hasMore || hits.more == MoreState.Loading || job?.isActive == true) return
        val query = request() ?: return
        val server = session.server ?: return
        setHits(hits.copy(more = MoreState.Loading))
        job = viewModelScope.launch {
            when (val result = repository.search(server, query, hits.items.size, PAGE_SIZE)) {
                is ApiResult.Ok -> setHits(
                    hits.copy(items = hits.items + result.value.items.map { MediaItem.from(it, ServerImages(server)) }, total = result.value.total, more = MoreState.Idle),
                )
                else -> setHits(hits.copy(more = MoreState.Failed))
            }
        }
    }

    private fun setHits(hits: SearchHits) = _state.update { it.copy(hits = Loadable.Ready(hits)) }

    /** What to ask for the field and the genre; `null` when there is nothing to ask. */
    private fun request(): SearchQuery? {
        val now = _state.value
        val text = now.query
        if (text.length == 1 || (text.isEmpty() && now.genre == null)) return null
        return SearchQuery(text.takeIf { it.isNotEmpty() }, now.genre)
    }

    private fun search(debounce: Boolean) {
        // A new text: the request of the old one goes (the generated client is interrupted).
        job?.cancel()
        val query = request()
        _state.update { it.copy(tooShort = it.query.length == 1) }
        if (query == null) {
            _state.update { it.copy(hits = null) }
            return
        }
        val server = session.server ?: return
        job = viewModelScope.launch {
            if (debounce) delay(DEBOUNCE_MS)
            _state.update { it.copy(hits = Loadable.Loading) }
            val result = repository.search(server, query, 0, PAGE_SIZE)
            val hits = when (result) {
                is ApiResult.Ok -> result.value.let { page ->
                    val found = SearchHits(page.items.map { MediaItem.from(it, ServerImages(server)) }, page.total, page.people, page.collections)
                    if (found.count == 0) Loadable.Empty else Loadable.Ready(found)
                }
                else -> result.toLoadable().let { it as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) }
            }
            _state.update { it.copy(hits = hits) }
        }
    }

    private fun loadGenres() {
        val server = session.server ?: return
        genresJob?.cancel()
        _state.update { it.copy(genres = Loadable.Loading) }
        genresJob = viewModelScope.launch {
            val genres = repository.genres(server).toLoadable { it.isEmpty() }
            _state.update { it.copy(genres = genres) }
        }
    }

    companion object {
        /** Typing waits this long before the request goes (web `DEBOUNCE_MS`). */
        const val DEBOUNCE_MS = 200L

        /** Titles per page (the server's default). */
        const val PAGE_SIZE = 50
    }
}
