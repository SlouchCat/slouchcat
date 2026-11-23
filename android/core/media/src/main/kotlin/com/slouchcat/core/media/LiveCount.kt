package com.slouchcat.core.media

import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/**
 * A number the shell shows next to an entry («Merkliste 12», «Sammlungen 4»), one per app graph
 * ([MediaData]): read once when the entry is drawn for a session ([refresh]), then kept current by
 * the pages that change it ([adjust], [set]) instead of a request per drawing. `null`: not known
 * (not read yet, another profile, signed out).
 */
class LiveCount {
    private val _count = MutableStateFlow<Int?>(null)
    val count: StateFlow<Int?> = _count.asStateFlow()

    /** Counts the changes of the pages: a read that started before one is older than it. */
    @Volatile
    private var generation = 0

    fun set(value: Int?) {
        generation++
        _count.value = value
    }

    /** [delta] entries more or fewer, never below 0; nothing while the number is not known. */
    fun adjust(delta: Int) {
        generation++
        _count.update { it?.let { n -> maxOf(0, n + delta) } }
    }

    /**
     * Reads the number from the server ([read] gives it); a failed read keeps the shown one, and so
     * does a read a page overtook with [set] or [adjust] meanwhile («Alle gelesen» while the rail
     * read; K26 stage 5 review L-8).
     */
    suspend fun refresh(read: suspend () -> ApiResult<Int>) {
        val started = generation
        (read() as? ApiResult.Ok)?.let { if (generation == started) _count.value = it.value }
    }
}
