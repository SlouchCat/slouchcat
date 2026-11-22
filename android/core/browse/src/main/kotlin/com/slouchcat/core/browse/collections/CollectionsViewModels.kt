package com.slouchcat.core.browse.collections

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Collection
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.media.LiveCount
import com.slouchcat.core.media.LoadError
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.data.CollectionsRepository
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.session.SessionManager
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class CollectionsUiState(val collections: Loadable<List<CollectionRef>> = Loadable.Loading)

/** «Sammlungen» (mockup `/collections`, web `CollectionsPage`): every collection with a visible member, by name. */
class CollectionsViewModel(
    private val session: SessionManager,
    private val repository: CollectionsRepository,
    private val count: LiveCount,
) : ViewModel() {
    private val _state = MutableStateFlow(CollectionsUiState())
    val state: StateFlow<CollectionsUiState> = _state.asStateFlow()
    private var loading: Job? = null

    init {
        load()
    }

    fun retry() = load()

    private fun load() {
        val server = session.server ?: return
        loading?.cancel()
        _state.update { it.copy(collections = Loadable.Loading) }
        loading = viewModelScope.launch {
            val result = repository.collections(server)
            if (result is ApiResult.Ok) count.set(result.value.size)
            _state.update { it.copy(collections = result.toLoadable { list -> list.isEmpty() }) }
        }
    }
}

/** The check mark «Gesehen» of a whole collection (web `useCollectionWatchedToggle`). */
enum class CollectionWatched { None, Half, Full }

/** A collection with every visible part. */
data class CollectionPage(val collection: Collection, val parts: List<ItemRef>, val items: List<MediaItem>) {
    val watched: CollectionWatched
        get() {
            val seen = collection.seenCount ?: 0
            return when {
                seen <= 0 -> CollectionWatched.None
                seen >= collection.total -> CollectionWatched.Full
                else -> CollectionWatched.Half
            }
        }

    /** «Alle merken» is pressed: every part is on the watchlist. */
    val allListed: Boolean get() = parts.isNotEmpty() && parts.all { it.inWatchlist }
}

/** What a collection page tells once. */
sealed interface CollectionMessage {
    data class Listed(val count: Int) : CollectionMessage

    data object Unlisted : CollectionMessage

    data object Watched : CollectionMessage

    data object Unwatched : CollectionMessage

    data object Failed : CollectionMessage
}

data class CollectionUiState(
    val page: Loadable<CollectionPage> = Loadable.Loading,
    /** «Alle merken» on its way. */
    val listing: Boolean = false,
    /** «Gesehen» on its way. */
    val marking: Boolean = false,
    /** The question before check marks go: a full or a half marked collection. */
    val asking: CollectionWatched? = null,
)

/**
 * One collection (mockup `/collection/:id`, web `CollectionPage`): `getCollection` with every
 * visible part, «Alle merken» (each part on or off the watchlist) and «Gesehen» for all parts
 * (`setCollectionWatched`); taking the marks off asks first (it cannot be undone).
 */
class CollectionViewModel(
    private val session: SessionManager,
    private val repository: CollectionsRepository,
    private val id: String,
    /** «Merkliste» of the shell: «Alle merken» changes it (code review m-1). */
    private val watchlistCount: LiveCount? = null,
) : ViewModel() {
    private val _state = MutableStateFlow(CollectionUiState())
    val state: StateFlow<CollectionUiState> = _state.asStateFlow()

    private val _messages = Channel<CollectionMessage>(Channel.BUFFERED)
    val messages: Flow<CollectionMessage> = _messages.receiveAsFlow()

    private var loading: Job? = null

    init {
        load(quiet = false)
    }

    fun retry() = load(quiet = false)

    /** Every part at once, page after page (a collection is short; «Alle merken» needs them all). */
    private fun load(quiet: Boolean) {
        val server = session.server ?: return
        loading?.cancel()
        if (!quiet) _state.update { it.copy(page = Loadable.Loading) }
        loading = viewModelScope.launch {
            var first: Collection? = null
            val parts = mutableListOf<ItemRef>()
            while (true) {
                when (val result = repository.collection(server, id, parts.size, PAGE_SIZE)) {
                    is ApiResult.Ok -> {
                        if (first == null) first = result.value
                        parts += result.value.items
                        if (result.value.items.isEmpty() || parts.size >= result.value.total) break
                    }
                    else -> {
                        if (!quiet || _state.value.page !is Loadable.Ready) {
                            _state.update { it.copy(page = result.toLoadable().let { l -> l as? Loadable.Failed ?: Loadable.Failed(LoadError.Failed) }) }
                        }
                        return@launch
                    }
                }
            }
            val images = ServerImages(server)
            val collection = checkNotNull(first)
            _state.update { it.copy(page = Loadable.Ready(CollectionPage(collection, parts, parts.map { p -> MediaItem.from(p, images) }))) }
        }
    }

    /** «Alle merken»: the parts not yet on the watchlist go on it; when all are, all go off it. */
    fun onListAll() {
        val page = (_state.value.page as? Loadable.Ready)?.value ?: return
        val server = session.server ?: return
        if (_state.value.listing) return
        val on = !page.allListed
        val changing = page.parts.filter { it.inWatchlist != on }
        _state.update { it.copy(listing = true) }
        viewModelScope.launch {
            val done = mutableSetOf<String>()
            for (part in changing) {
                if (repository.setWatchlist(server, part.id, on) is ApiResult.Ok) done += part.id
            }
            watchlistCount?.adjust(if (on) done.size else -done.size)
            val images = ServerImages(server)
            // On the state as it is now: a quiet reload meanwhile («Gesehen») keeps its marks (code review m-6).
            _state.update { s ->
                val current = (s.page as? Loadable.Ready)?.value ?: return@update s.copy(listing = false)
                val parts = current.parts.map { if (it.id in done) it.copy(inWatchlist = on) else it }
                s.copy(listing = false, page = Loadable.Ready(current.copy(parts = parts, items = parts.map { p -> MediaItem.from(p, images) })))
            }
            _messages.send(
                when {
                    done.size < changing.size -> CollectionMessage.Failed
                    on -> CollectionMessage.Listed(done.size)
                    else -> CollectionMessage.Unlisted
                },
            )
        }
    }

    /** «Gesehen»: none marked sets every part; a full or half marked collection asks first. */
    fun onWatched() {
        val page = (_state.value.page as? Loadable.Ready)?.value ?: return
        when (page.watched) {
            CollectionWatched.None -> mark(true)
            else -> _state.update { it.copy(asking = page.watched) }
        }
    }

    fun onDismiss() = _state.update { it.copy(asking = null) }

    /** The answer to the question: mark every part ([watched]) or take every mark off. */
    fun onConfirm(watched: Boolean) {
        _state.update { it.copy(asking = null) }
        mark(watched)
    }

    private fun mark(watched: Boolean) {
        val server = session.server ?: return
        if (_state.value.marking) return
        _state.update { it.copy(marking = true) }
        viewModelScope.launch {
            val result = repository.setWatched(server, id, watched)
            _state.update { it.copy(marking = false) }
            if (result is ApiResult.Ok) {
                _messages.send(if (watched) CollectionMessage.Watched else CollectionMessage.Unwatched)
                load(quiet = true)
            } else {
                _messages.send(CollectionMessage.Failed)
            }
        }
    }

    companion object {
        /** The most the server sends per page (`getCollection` `limit`). */
        const val PAGE_SIZE = 200

        /**
         * A series or podcast plays from its own page (its next episode). Not `isGroup` of
         * `:core:media`, which also counts seasons (code review m-9).
         */
        fun playsFromOwnPage(item: ItemRef): Boolean = item.kind == ItemKind.SERIES || item.kind == ItemKind.PODCAST
    }
}
