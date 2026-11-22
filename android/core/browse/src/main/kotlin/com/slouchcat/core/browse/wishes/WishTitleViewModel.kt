package com.slouchcat.core.browse.wishes

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitle
import com.slouchcat.core.api.generated.models.Wish
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.data.WishesRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** The sheets of the wish page. */
enum class WishSheet { Request, Withdraw }

/**
 * The page of one title outside the library (mockup `/wish/:i`, web `/titles/:media/:tmdbId`).
 * [seasons]: Seerr's seasons of a series while the sheet «Anfordern» is open; [chosen]: the seasons
 * ticked there (at first every one that can be chosen, «Alle fehlenden Staffeln»).
 */
data class WishTitleUiState(
    val title: Loadable<ExternalTitle> = Loadable.Loading,
    val sheet: WishSheet? = null,
    val seasons: Loadable<List<WishSeasonOffer>> = Loadable.Loading,
    val chosen: Set<Int> = emptySet(),
    /** The request or the withdrawal is on its way. */
    val sending: Boolean = false,
    /** Why the last request failed (a problem code), shown in the sheet. */
    val error: String? = null,
) {
    /** The seasons the sheet offers to tick: missing or requested by another profile, never available ones. */
    val choosable: List<WishSeasonOffer>
        get() = (seasons as? Loadable.Ready)?.value.orEmpty().filter { it.state != WishSeasonOffer.State.AVAILABLE }

    /** Every choosable season is ticked: the request is «Alle fehlenden Staffeln». */
    val all: Boolean get() = choosable.isNotEmpty() && chosen.size == choosable.size

    /** Ticked seasons nobody asked for yet (the others only join another profile's wish). */
    val fresh: Int get() = choosable.count { it.state == WishSeasonOffer.State.MISSING && it.number in chosen }
}

/** What the wish page tells once. */
sealed interface WishTitleMessage {
    /** The wish is made ([joined]: another profile's wish was joined, «Benachrichtige mich»). */
    data class Requested(val joined: Boolean) : WishTitleMessage

    data object Withdrawn : WishTitleMessage

    data class Failed(val code: String?) : WishTitleMessage
}

/**
 * One title outside the library (concept 25 «Vorschau», concept 27 stage 8): `getExternalTitle`
 * with the wish card, «Anfordern» (a film at once, a series with the seasons of `getWishSeasons`),
 * «Zurückziehen» of the own open wish. The server says whether the profile may request and why not
 * (rule 12); the page shows it. [openCount]: the number at «Meine Wünsche».
 */
class WishTitleViewModel(
    private val session: SessionManager,
    private val repository: WishesRepository,
    private val openCount: LiveCount,
    private val media: ExternalMedia,
    private val tmdbId: Long,
    /** Where the request comes from (`WishCreate.source`): the page itself, or the search. */
    private val source: WishCreate.Source = WishCreate.Source.DETAIL,
) : ViewModel() {
    private val _state = MutableStateFlow(WishTitleUiState())
    val state: StateFlow<WishTitleUiState> = _state.asStateFlow()

    private val _messages = Channel<WishTitleMessage>(Channel.BUFFERED)
    val messages: Flow<WishTitleMessage> = _messages.receiveAsFlow()

    init {
        load()
    }

    private fun load(keep: Boolean = false) {
        val server = session.server ?: run {
            _state.update { it.copy(title = Loadable.Failed(LoadError.Offline)) }
            return
        }
        if (!keep || _state.value.title !is Loadable.Ready) _state.update { it.copy(title = Loadable.Loading) }
        viewModelScope.launch {
            val result = repository.title(server, media, tmdbId)
            if (keep && result !is ApiResult.Ok && _state.value.title is Loadable.Ready) return@launch
            _state.update { it.copy(title = result.toLoadable()) }
        }
    }

    fun retry() = load()

    /** Back on the page: the wish may have changed elsewhere («Meine Wünsche», another profile). */
    fun onResume() {
        if (_state.value.title is Loadable.Ready && !_state.value.sending) load(keep = true)
    }

    /** «Anfordern»: a series opens the sheet with its seasons, a film the sheet to confirm. */
    fun onRequest() {
        val title = (_state.value.title as? Loadable.Ready)?.value ?: return
        _state.update { it.copy(sheet = WishSheet.Request, error = null, seasons = Loadable.Loading, chosen = emptySet()) }
        if (title.media != ExternalMedia.SHOW) {
            _state.update { it.copy(seasons = Loadable.Empty) }
            return
        }
        val server = session.server ?: return
        viewModelScope.launch {
            val result = repository.seasons(server, tmdbId)
            _state.update { now ->
                val seasons = when (result) {
                    is ApiResult.Ok -> result.value.seasons.sortedBy { it.number }.let { if (it.isEmpty()) Loadable.Empty else Loadable.Ready(it) }
                    else -> result.toLoadable().let { it as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) }
                }
                val choosable = (seasons as? Loadable.Ready)?.value.orEmpty().filter { it.state != WishSeasonOffer.State.AVAILABLE }
                now.copy(seasons = seasons, chosen = choosable.map { it.number }.toSet())
            }
        }
    }

    fun onRetrySeasons() = onRequest()

    fun onSeason(number: Int) = _state.update {
        it.copy(chosen = if (number in it.chosen) it.chosen - number else it.chosen + number, error = null)
    }

    /** «Alle Staffeln»: ticks every choosable season, or none when all are ticked. */
    fun onAllSeasons() = _state.update {
        it.copy(chosen = if (it.all) emptySet() else it.choosable.map { s -> s.number }.toSet(), error = null)
    }

    fun onWithdraw() = _state.update { it.copy(sheet = WishSheet.Withdraw, error = null) }

    fun onDismiss() = _state.update { if (it.sending) it else it.copy(sheet = null, error = null) }

    fun onSubmit() {
        val now = _state.value
        val title = (now.title as? Loadable.Ready)?.value ?: return
        val server = session.server ?: return
        if (now.sending) return
        val series = title.media == ExternalMedia.SHOW
        if (series && now.chosen.isEmpty()) return
        val everySeason = series && now.all && now.choosable.size == (now.seasons as? Loadable.Ready)?.value?.size
        val request = WishCreate(
            media = title.media,
            tmdbId = title.tmdbId,
            seasons = if (series && !everySeason) now.chosen.sorted() else null,
            allSeasons = if (everySeason) true else null,
            source = source,
        )
        _state.update { it.copy(sending = true, error = null) }
        viewModelScope.launch {
            when (val result = repository.create(server, request)) {
                is ApiResult.Ok -> {
                    _state.update { it.copy(sending = false, sheet = null) }
                    if (result.value.role == Wish.Role.OWNER) openCount.adjust(+1)
                    _messages.send(WishTitleMessage.Requested(joined = result.value.role == Wish.Role.JOINED))
                    load(keep = true)
                }
                else -> _state.update { it.copy(sending = false, error = codeOf(result)) }
            }
        }
    }

    fun onConfirmWithdraw() {
        val title = (_state.value.title as? Loadable.Ready)?.value ?: return
        val wish = title.wish ?: return
        val server = session.server ?: return
        if (_state.value.sending) return
        _state.update { it.copy(sending = true) }
        viewModelScope.launch {
            val result = repository.withdraw(server, wish.id)
            _state.update { it.copy(sending = false, sheet = null) }
            when (result) {
                is ApiResult.Ok -> {
                    openCount.adjust(-1)
                    _messages.send(WishTitleMessage.Withdrawn)
                }
                ApiResult.NotFound -> Unit
                else -> _messages.send(WishTitleMessage.Failed(codeOf(result)))
            }
            load(keep = true)
        }
    }

    private fun codeOf(result: ApiResult<*>): String? = when (result) {
        is ApiResult.Failed -> result.code ?: FAILED
        is ApiResult.NotAllowed -> result.code ?: FAILED
        ApiResult.NotFound -> NOT_FOUND
        ApiResult.Offline -> OFFLINE
        else -> FAILED
    }

    companion object {
        /** Codes of the page itself next to the server's: no answer, gone, anything else. */
        const val OFFLINE = "offline"
        const val NOT_FOUND = "not_found"
        const val FAILED = "failed"
    }
}
