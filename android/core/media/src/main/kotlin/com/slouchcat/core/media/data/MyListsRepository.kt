package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.WatchedEntry
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Page
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
