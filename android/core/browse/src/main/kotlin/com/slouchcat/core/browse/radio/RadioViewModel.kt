package com.slouchcat.core.browse.radio

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.RadioFacetCount
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.PageLoader
import com.slouchcat.core.media.Paged
import com.slouchcat.core.media.data.RadioQuery
import com.slouchcat.core.media.data.RadioRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.SessionManager
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

/** What the station list shows (web quick filters «Mein Land», «Meine Sender», the search). */
data class RadioFilter(
    /** A country code of the offer; `null`: every country. */
    val country: String? = null,
    /** Only the profile's favourites («Meine Sender»). */
    val favorites: Boolean = false,
    val text: String = "",
) {
    val unfiltered: Boolean get() = country == null && !favorites && text.isBlank()
}

data class RadioUiState(
    val filter: RadioFilter = RadioFilter(),
    /** The profile's country when the offer has stations of it (`effectiveCountry`); `null`: none. */
    val myCountry: String? = null,
    /** The countries of the offer with their counts, for the sheet «Land». */
    val countries: List<RadioFacetCount> = emptyList(),
    /** «Meine Sender» in the profile's order (shown above the list while nothing filters). */
    val favorites: Loadable<List<RadioStation>> = Loadable.Loading,
    /** The stations behind a heart that waits for the server. */
    val busy: Set<Long> = emptySet(),
    val countrySheet: Boolean = false,
)

enum class RadioMessage { Failed, Added, Removed }

/**
 * The page «Webradio» (concept 16, concept 19 part B; web `RadioPage`): the stations of the offer
 * the administration chose, in pages, filtered by country, favourites and a search; the profile's
 * favourites with a heart (`addRadioFavorite`, `removeRadioFavorite`), changed at once and put back
 * when the server refuses. It opens on the profile's country when the offer has it, else
 * unfiltered (B4.5). What plays and how comes from the player (`:core:player`, `RadioController`).
 * [favoritesCount]: «Meine Sender» of the shell, adjusted with every heart.
 */
class RadioViewModel(
    private val session: SessionManager,
    private val repository: RadioRepository,
    private val favoritesCount: LiveCount? = null,
) : ViewModel() {
    private val _ui = MutableStateFlow(RadioUiState())

    private val pager = PageLoader<RadioStation, RadioStation>(viewModelScope, pageSize = PAGE, map = { it }, key = { it.id }) { offset, limit ->
        val server = session.server ?: return@PageLoader ApiResult.Offline
        repository.stations(server, query(), offset, limit)
    }

    /** Hearts changed on this page, until the lists are read again. */
    private val hearts = MutableStateFlow<Map<Long, Boolean>>(emptyMap())

    val ui: StateFlow<RadioUiState> = combine(_ui, hearts) { ui, changed ->
        if (changed.isEmpty()) {
            ui
        } else {
            when (val f = ui.favorites) {
                is Loadable.Ready -> ui.copy(favorites = favoritesAfter(f.value, changed))
                Loadable.Empty -> ui.copy(favorites = favoritesAfter(emptyList(), changed))
                else -> ui
            }
        }
    }.stateIn(viewModelScope, SharingStarted.Eagerly, RadioUiState())

    val stations: StateFlow<Paged<RadioStation>> = combine(pager.state, hearts) { paged, changed ->
        if (changed.isEmpty()) paged else paged.copy(items = (paged.items as? Loadable.Ready)?.let { r -> Loadable.Ready(r.value.map { s -> changed[s.id]?.let { s.copy(favorite = it) } ?: s }) } ?: paged.items)
    }.stateIn(viewModelScope, SharingStarted.Eagerly, Paged())

    private val _messages = Channel<RadioMessage>(Channel.BUFFERED)
    val messages: Flow<RadioMessage> = _messages.receiveAsFlow()

    /** Stations shown in the list, so a heart in «Meine Sender» finds the station again after a removal. */
    private val known = mutableMapOf<Long, RadioStation>()

    init {
        start()
    }

    private fun query() = _ui.value.filter.let { RadioQuery(text = it.text, country = it.country, favorites = it.favorites) }

    private fun start() {
        val server = session.server ?: return
        viewModelScope.launch {
            val prefs = (repository.preferences(server) as? ApiResult.Ok)?.value
            val mine = prefs?.effectiveCountry?.takeIf { it.isNotBlank() }
            _ui.update { it.copy(myCountry = mine, filter = it.filter.copy(country = mine)) }
            pager.refresh()
            loadFacets()
        }
        loadFavorites()
    }

    fun retry() {
        pager.refresh()
        loadFavorites()
        loadFacets()
    }

    fun onNearEnd() = pager.loadMore()

    private fun loadFavorites() {
        val server = session.server ?: return
        viewModelScope.launch {
            val loaded = repository.favorites(server).toLoadable { it.isEmpty() }
            hearts.value = emptyMap()
            (loaded as? Loadable.Ready)?.value?.forEach { known[it.id] = it }
            _ui.update { it.copy(favorites = loaded) }
            favoritesCount?.set((loaded as? Loadable.Ready)?.value?.size ?: if (loaded == Loadable.Empty) 0 else null)
        }
    }

    private fun loadFacets() {
        val server = session.server ?: return
        viewModelScope.launch {
            val facets = (repository.facets(server, RadioQuery()) as? ApiResult.Ok)?.value ?: return@launch
            _ui.update { it.copy(countries = facets.country.sortedByDescending { c -> c.count }) }
        }
    }

    private fun filter(change: (RadioFilter) -> RadioFilter) {
        val next = change(_ui.value.filter)
        if (next == _ui.value.filter) return
        _ui.update { it.copy(filter = next, countrySheet = false) }
        pager.refresh()
    }

    /** A country of the sheet, or every country with `null`. */
    fun onCountry(code: String?) = filter { it.copy(country = code) }

    fun onFavoritesOnly() = filter { it.copy(favorites = !it.favorites) }

    fun onSearch(text: String) = filter { it.copy(text = text.trim()) }

    fun onCountrySheet(open: Boolean) = _ui.update { it.copy(countrySheet = open) }

    /** The heart of [station]. */
    fun onFavorite(station: RadioStation) {
        val server = session.server ?: return
        if (station.id in _ui.value.busy) return
        known[station.id] = station
        val on = !(hearts.value[station.id] ?: station.favorite)
        _ui.update { it.copy(busy = it.busy + station.id) }
        hearts.update { it + (station.id to on) }
        viewModelScope.launch {
            val result = repository.setFavorite(server, station.id, on)
            _ui.update { it.copy(busy = it.busy - station.id) }
            if (result is ApiResult.Ok) {
                favoritesCount?.adjust(if (on) 1 else -1)
                _messages.send(if (on) RadioMessage.Added else RadioMessage.Removed)
            } else {
                hearts.update { it + (station.id to !on) }
                _messages.send(RadioMessage.Failed)
            }
        }
    }

    /** «Meine Sender» after the changed hearts: removed ones go, added ones come at the end. */
    private fun favoritesAfter(list: List<RadioStation>, changed: Map<Long, Boolean>): Loadable<List<RadioStation>> {
        val kept = list.filter { changed[it.id] != false }.map { it.copy(favorite = true) }
        val added = changed.filter { (id, on) -> on && kept.none { it.id == id } }.keys.mapNotNull { known[it]?.copy(favorite = true) }
        val all = kept + added
        return if (all.isEmpty()) Loadable.Empty else Loadable.Ready(all)
    }

    private companion object {
        const val PAGE = 60
    }
}
