package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.Trailer
import com.slouchcat.core.api.generated.models.UserPickResult
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * What a detail page reads and changes beyond [ItemRepository] (`getItem`, the watchlist): the
 * trailers, the check mark «gesehen» and the user pick «Empfehlen». The server decides every
 * state (rule 12); not visible is 404 (`ApiResult.NotFound`).
 */
interface DetailRepository : ItemRepository {
    /** `getItemTrailers`: the server's order, the first one plays. */
    suspend fun trailers(server: HttpUrl, id: String): ApiResult<List<Trailer>>

    /** `setItemWatched`: a season, series or podcast sets or clears every visible episode. */
    suspend fun setWatched(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit>

    /** `recommendItem`. */
    suspend fun recommend(server: HttpUrl, id: String): ApiResult<UserPickResult>

    /** `unrecommendItem`: takes the own pick back. */
    suspend fun unrecommend(server: HttpUrl, id: String): ApiResult<Unit>
}

class ApiDetailRepository(private val api: ApiCaller, private val http: Call.Factory) :
    DetailRepository,
    ItemRepository by ApiItemRepository(api, http) {
    private fun items(server: HttpUrl) = SlouchcatApi.create(::ItemsApi, server, http)

    private fun discover(server: HttpUrl) = SlouchcatApi.create(::DiscoverApi, server, http)

    override suspend fun trailers(server: HttpUrl, id: String) = api.call("trailers") { items(server).getItemTrailers(id) }

    override suspend fun setWatched(server: HttpUrl, id: String, on: Boolean) = api.call("watched") {
        items(server).setItemWatched(id, ItemWatchedRequest(on))
    }

    override suspend fun recommend(server: HttpUrl, id: String) = api.call("pick") { discover(server).recommendItem(id) }

    override suspend fun unrecommend(server: HttpUrl, id: String) = api.call("pick") { discover(server).unrecommendItem(id) }
}
