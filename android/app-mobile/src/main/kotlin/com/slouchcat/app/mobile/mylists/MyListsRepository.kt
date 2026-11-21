package com.slouchcat.app.mobile.mylists

import com.slouchcat.app.mobile.media.Page
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.WatchedEntry
import com.slouchcat.core.api.map
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * «Merkliste» and «Gesehen» of the active profile (`listWatchlist`, `addToWatchlist`,
 * `removeFromWatchlist`, `listWatched`, `setItemWatched`). Entries whose title is no longer visible
 * are left out by the server (rules 2 and 12).
 */
interface MyListsRepository {
    suspend fun watchlist(server: HttpUrl, offset: Int, limit: Int): ApiResult<Page<ItemRef>>

    suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit>

    /** Newest first (`sort=watched`), every kind (the mockup has no filter). */
    suspend fun watched(server: HttpUrl, offset: Int, limit: Int): ApiResult<Page<WatchedEntry>>

    suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean): ApiResult<Unit>
}

class ApiMyListsRepository(private val api: ApiCaller, private val http: Call.Factory) : MyListsRepository {
    private fun discover(server: HttpUrl) = SlouchcatApi.create(::DiscoverApi, server, http)

    override suspend fun watchlist(server: HttpUrl, offset: Int, limit: Int) =
        api.call("watchlist") { discover(server).listWatchlist(offset, limit) }.map { Page(it.items, it.total) }

    override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = api.call("watchlist") {
        if (on) discover(server).addToWatchlist(id) else discover(server).removeFromWatchlist(id)
    }

    override suspend fun watched(server: HttpUrl, offset: Int, limit: Int) =
        api.call("watched") { discover(server).listWatched(offset = offset, limit = limit) }.map { Page(it.items, it.total) }

    override suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean) = api.call("watched") {
        SlouchcatApi.create(::ItemsApi, server, http).setItemWatched(id, ItemWatchedRequest(watched))
    }
}

/**
 * The number of «Merkliste» for the media choice and «Mehr», one per app: read when the choice is
 * drawn for a session, set again by the pages that change the list (no request per drawing).
 */
class WatchlistCount {
    private val _count = MutableStateFlow<Int?>(null)
    val count: StateFlow<Int?> = _count.asStateFlow()

    fun set(value: Int?) {
        _count.value = value
    }

    /** Reads the number from the server (one entry is enough: `total` counts them all). */
    suspend fun refresh(repository: MyListsRepository, server: HttpUrl) {
        (repository.watchlist(server, 0, 1) as? ApiResult.Ok)?.let { _count.value = it.value.total }
    }
}
