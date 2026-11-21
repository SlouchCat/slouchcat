package com.slouchcat.app.mobile.collections

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.models.Collection
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * The collections the active profile sees (`listVisibleCollections`, `getCollection`) and what a
 * collection page changes (`setCollectionWatched`, the watchlist of its parts). A collection
 * without a visible member is 404 (rules 2 and 12).
 */
interface CollectionsRepository {
    suspend fun collections(server: HttpUrl): ApiResult<List<CollectionRef>>

    suspend fun collection(server: HttpUrl, id: String, offset: Int, limit: Int): ApiResult<Collection>

    suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean): ApiResult<Unit>

    suspend fun setWatchlist(server: HttpUrl, itemId: String, on: Boolean): ApiResult<Unit>
}

class ApiCollectionsRepository(private val api: ApiCaller, private val http: Call.Factory) : CollectionsRepository {
    private fun discover(server: HttpUrl) = SlouchcatApi.create(::DiscoverApi, server, http)

    override suspend fun collections(server: HttpUrl) = api.call("collections") { discover(server).listVisibleCollections() }

    override suspend fun collection(server: HttpUrl, id: String, offset: Int, limit: Int) =
        api.call("collection") { discover(server).getCollection(id, offset, limit) }

    override suspend fun setWatched(server: HttpUrl, id: String, watched: Boolean) =
        api.call("collection") { discover(server).setCollectionWatched(id, ItemWatchedRequest(watched)) }

    override suspend fun setWatchlist(server: HttpUrl, itemId: String, on: Boolean) = api.call("watchlist") {
        if (on) discover(server).addToWatchlist(itemId) else discover(server).removeFromWatchlist(itemId)
    }
}
