package com.slouchcat.core.browse.wishes

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.WishRights
import com.slouchcat.core.media.ExternalCard
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.WishesRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/**
 * «Noch nicht in der Bibliothek?» under the search's hits. [rights]: `null` until known, then what
 * the server allows; [hits] only while the profile may request (`canRequest`) and the query has at
 * least two characters; [discover]: the card «„…“ wünschen» leads to the wishes' own search.
 */
data class MissingTitlesUiState(
    val rights: WishRights? = null,
    val query: String = "",
    val hits: Loadable<List<ExternalCard>>? = null,
) {
    val canRequest: Boolean get() = rights?.enabled == true && rights.canRequest
    val discover: Boolean get() = rights?.enabled == true && rights.discover == true
}

/**
 * The titles outside the library for the search (concept 25, 5.2; web `SearchMissing`):
 * `searchExternalTitles` for the search's query, at most [LIMIT], only when the server says the
 * profile may request (rule 12: the server filters every hit by right and age).
 */
class MissingTitlesViewModel(
    private val session: SessionManager,
    private val repository: WishesRepository,
) : ViewModel() {
    private val images = session.server?.let(::ServerImages)
    private val _state = MutableStateFlow(MissingTitlesUiState())
    val state: StateFlow<MissingTitlesUiState> = _state.asStateFlow()
    private var job: Job? = null

    init {
        val server = session.server
        if (server != null) {
            viewModelScope.launch {
                val rights = (repository.rights(server) as? ApiResult.Ok)?.value
                _state.update { it.copy(rights = rights) }
                if (_state.value.query.isNotEmpty()) search(_state.value.query, immediately = true)
            }
        }
    }

    /** The search's query changed; the hits follow after a short pause (as the search's own). */
    fun onQuery(query: String) {
        val trimmed = query.trim()
        if (trimmed == _state.value.query) return
        _state.update { it.copy(query = trimmed) }
        search(trimmed, immediately = false)
    }

    fun retry() = search(_state.value.query, immediately = true)

    private fun search(query: String, immediately: Boolean) {
        job?.cancel()
        val server = session.server
        if (!_state.value.canRequest || query.length < WishesViewModel.MIN_QUERY || server == null || images == null) {
            _state.update { it.copy(hits = null) }
            return
        }
        _state.update { it.copy(hits = Loadable.Loading) }
        job = viewModelScope.launch {
            if (!immediately) delay(PAUSE_MS)
            val result = repository.missing(server, query, null, 0, LIMIT)
            _state.update {
                it.copy(
                    hits = when (result) {
                        is ApiResult.Ok -> result.value.items.map { ref -> ExternalCard.from(ref, images) }.let { cards -> if (cards.isEmpty()) Loadable.Empty else Loadable.Ready(cards) }
                        else -> result.toLoadable().let { l -> l as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) }
                    },
                )
            }
        }
    }

    companion object {
        const val LIMIT = 8
        private const val PAUSE_MS = 300L
    }
}
