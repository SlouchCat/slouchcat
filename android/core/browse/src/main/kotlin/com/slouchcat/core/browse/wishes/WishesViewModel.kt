package com.slouchcat.core.browse.wishes

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.Wish
import com.slouchcat.core.api.generated.models.WishCounts
import com.slouchcat.core.api.generated.models.WishRights
import com.slouchcat.core.api.map
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MoreState
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.PageLoader
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WishCard
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.media.data.WishesRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
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
import okhttp3.HttpUrl

/** The two parts of the area «Wünsche» (mockup `/wishes`): «Entdecken» and «Meine Wünsche». */
enum class WishesTab { Discover, Mine }

/** One row of «Entdecken» with its tiles (the genre rows stay in the web app). */
data class DiscoverRowCards(val key: DiscoverRowKey, val cards: List<ExternalCard>)

/**
 * The area «Wünsche». [rights]: what the server says the profile may do; [Loadable.Empty] when the
 * wishes are off or Seerr is not set up (the page says so, as the web app hides the entry).
 */
data class WishesUiState(
    val rights: Loadable<WishRights> = Loadable.Loading,
    val tab: WishesTab = WishesTab.Discover,
    /** The rows of «Entdecken»; [Loadable.Failed] with [LoadError.NotFound]: the pages are not for this profile. */
    val rows: Loadable<List<DiscoverRowCards>> = Loadable.Loading,
    /** Seerr did not answer: the rows are the last ones it sent. */
    val stale: Boolean = false,
    /** The text of the search field of «Entdecken». */
    val text: String = "",
    /** The hits of the submitted search; `null` without one. */
    val results: Paged<ExternalCard>? = null,
    val filter: WishFilter = WishFilter.All,
    val mine: Paged<WishCard> = Paged(),
    val counts: WishCounts? = null,
    /** The wish whose «Zurückziehen» the page asks about. */
    val withdrawing: WishCard? = null,
    /** Wishes whose change is on its way. */
    val busy: Set<Long> = emptySet(),
) {
    /** «Entdecken» is there only when the server shows its pages to the profile (`WishRights.discover`). */
    val discover: Boolean get() = (rights as? Loadable.Ready)?.value?.discover == true

    /** The tab the page shows: «Meine Wünsche» alone when «Entdecken» is not for this profile. */
    val shownTab: WishesTab get() = if (discover) tab else WishesTab.Mine

    /** Every wish of the profile, open or closed. */
    val total: Int get() = counts?.let { it.open + it.available + it.closed } ?: 0
}

/** What the area tells once. */
sealed interface WishesMessage {
    data class Withdrawn(val title: String, val joined: Boolean) : WishesMessage

    /** A change failed; [code] the problem's code (`wish_not_withdrawable` …). */
    data class Failed(val code: String?) : WishesMessage
}

/**
 * The area «Wünsche» (mockup `/wishes`, web `/wishes` and `/discover`; concept 25, concept 27
 * stage 8): what the profile may request (`getSession`), «Entdecken» with its rows and its search,
 * «Meine Wünsche» page by page with the filter and «Zurückziehen». [openCount]: the number at
 * «Meine Wünsche» the shell shows too. Both apps use it (TV: concept 26 stage 5).
 */
class WishesViewModel(
    private val session: SessionManager,
    private val repository: WishesRepository,
    private val openCount: LiveCount,
    initialTab: WishesTab = WishesTab.Discover,
    initialQuery: String? = null,
) : ViewModel() {
    private val images = session.server?.let(::ServerImages)
    private val local = MutableStateFlow(WishesUiState(tab = initialTab, text = initialQuery.orEmpty()))
    private val search = MutableStateFlow<Paged<ExternalCard>?>(null)
    private var searchPage = 0
    private var searchPages = 0
    private var searchQuery = ""
    private var searchJob: Job? = null

    private val mine = PageLoader(viewModelScope, pageSize = PAGE, map = { it: Wish -> WishCard.from(it, images!!) }, key = { it.id }) { offset, limit ->
        val server = session.server
        if (images == null || server == null) return@PageLoader ApiResult.Offline
        repository.wishes(server, local.value.filter, offset, limit).also { result ->
            if (result is ApiResult.Ok) {
                local.update { it.copy(counts = result.value.counts) }
                openCount.set(result.value.counts.open)
            }
        }.map { Page(it.items, it.total) }
    }

    val state: StateFlow<WishesUiState> = combine(local, mine.state, search) { own, wishes, hits -> own.copy(mine = wishes, results = hits) }
        .stateIn(viewModelScope, SharingStarted.Eagerly, WishesUiState(tab = initialTab, text = initialQuery.orEmpty()))

    private val _messages = Channel<WishesMessage>(Channel.BUFFERED)
    val messages: Flow<WishesMessage> = _messages.receiveAsFlow()

    init {
        load()
    }

    private fun load() {
        val server = session.server
        if (server == null || images == null) {
            local.update { it.copy(rights = Loadable.Failed(LoadError.Offline)) }
            return
        }
        local.update { it.copy(rights = Loadable.Loading) }
        viewModelScope.launch {
            val rights = when (val result = repository.rights(server)) {
                is ApiResult.Ok -> result.value?.takeIf { it.enabled }?.let { Loadable.Ready(it) } ?: Loadable.Empty
                else -> result.toLoadable().let { it as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) }
            }
            local.update { it.copy(rights = rights) }
            val ready = (rights as? Loadable.Ready)?.value ?: return@launch
            mine.refresh()
            if (ready.discover == true) {
                loadRows(server)
                if (local.value.text.trim().length >= MIN_QUERY) onSubmit()
            }
        }
    }

    private suspend fun loadRows(server: HttpUrl) {
        local.update { it.copy(rows = Loadable.Loading) }
        val result = repository.discover(server)
        val rows = result.map { discover ->
            discover.rows.filter { it.propertyEntries.isNotEmpty() }.map { row -> DiscoverRowCards(row.key, row.propertyEntries.map { ExternalCard.from(it, images!!) }) }
        }
        local.update { state ->
            state.copy(
                rows = rows.toLoadable { it.isEmpty() },
                stale = (result as? ApiResult.Ok)?.value?.rows?.any { it.stale } == true,
            )
        }
    }

    fun retry() = load()

    fun onTab(tab: WishesTab) {
        local.update { it.copy(tab = tab) }
        if (tab == WishesTab.Mine) mine.refresh(keep = true)
    }

    fun onFilter(filter: WishFilter) {
        if (filter == local.value.filter) return
        local.update { it.copy(filter = filter) }
        mine.refresh()
    }

    fun onMoreWishes() = mine.loadMore()

    fun onText(text: String) {
        local.update { it.copy(text = text) }
        if (text.isBlank()) clearSearch()
    }

    fun onClear() {
        local.update { it.copy(text = "") }
        clearSearch()
    }

    private fun clearSearch() {
        searchJob?.cancel()
        search.value = null
        searchQuery = ""
    }

    /** Searches TMDB through Seerr for the text (at least two characters, as the web app). */
    fun onSubmit() {
        val query = local.value.text.trim()
        val server = session.server ?: return
        if (query.length < MIN_QUERY || images == null) return
        searchJob?.cancel()
        searchQuery = query
        search.value = Paged()
        searchJob = viewModelScope.launch {
            val result = repository.search(server, query, 1)
            search.value = when (result) {
                is ApiResult.Ok -> {
                    searchPage = result.value.page
                    searchPages = result.value.totalPages ?: result.value.page
                    val cards = result.value.propertyEntries.map { ExternalCard.from(it, images) }.distinctBy { it.key }
                    Paged(if (cards.isEmpty()) Loadable.Empty else Loadable.Ready(cards), totalOf(cards.size))
                }
                else -> Paged(result.toLoadable().let { it as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) })
            }
        }
    }

    /** The next page of the search's hits. */
    fun onMoreResults() {
        val now = search.value ?: return
        val server = session.server ?: return
        if (!now.hasMore || now.more == MoreState.Loading || searchJob?.isActive == true || images == null) return
        search.value = now.copy(more = MoreState.Loading)
        searchJob = viewModelScope.launch {
            when (val result = repository.search(server, searchQuery, searchPage + 1)) {
                is ApiResult.Ok -> {
                    searchPage = result.value.page
                    searchPages = result.value.totalPages ?: searchPage
                    val known = now.loaded.mapTo(HashSet()) { it.key }
                    val cards = now.loaded + result.value.propertyEntries.map { ExternalCard.from(it, images) }.filter { known.add(it.key) }
                    search.value = Paged(Loadable.Ready(cards), if (cards.size == now.loaded.size) cards.size else totalOf(cards.size))
                }
                else -> search.value = now.copy(more = MoreState.Failed)
            }
        }
    }

    /** One more than shown while Seerr has another page, so the list asks for it at its end. */
    private fun totalOf(shown: Int) = if (searchPage < searchPages) shown + 1 else shown

    fun onAskWithdraw(card: WishCard) = local.update { it.copy(withdrawing = card) }

    fun onDismissWithdraw() = local.update { it.copy(withdrawing = null) }

    fun onConfirmWithdraw() {
        val card = local.value.withdrawing ?: return
        val server = session.server ?: return
        local.update { it.copy(withdrawing = null, busy = it.busy + card.id) }
        viewModelScope.launch {
            when (val result = repository.withdraw(server, card.id)) {
                is ApiResult.Ok -> {
                    _messages.send(WishesMessage.Withdrawn(card.title, card.joined))
                    mine.refresh(keep = true)
                }
                is ApiResult.Failed -> _messages.send(WishesMessage.Failed(result.code))
                is ApiResult.NotAllowed -> _messages.send(WishesMessage.Failed(result.code))
                ApiResult.NotFound -> mine.refresh(keep = true)
                else -> _messages.send(WishesMessage.Failed(null))
            }
            local.update { it.copy(busy = it.busy - card.id) }
        }
    }

    /** Back from a title's page: its wish may have changed. */
    fun onResume() {
        if (local.value.rights is Loadable.Ready) mine.refresh(keep = true)
    }

    companion object {
        const val MIN_QUERY = 2
        private const val PAGE = 50
    }
}
