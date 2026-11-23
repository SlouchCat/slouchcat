package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.WishesApi
import com.slouchcat.core.api.generated.models.DiscoverPage
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitle
import com.slouchcat.core.api.generated.models.ExternalTitleRef
import com.slouchcat.core.api.generated.models.Wish
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishDiscover
import com.slouchcat.core.api.generated.models.WishPage
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishRights
import com.slouchcat.core.api.generated.models.WishSeasons
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Page
import okhttp3.Call
import okhttp3.HttpUrl

/** The filter of «Meine Wünsche» (`WishStateFilter`): all, the open ones, the available ones, the closed ones. */
enum class WishFilter { All, Open, Available, Closed }

/** The open states (`WishStateFilter` `open`): the wish is on its way; every other state is closed. */
val OPEN_WISH_STATES: Set<WishState> = setOf(WishState.QUEUED, WishState.PENDING, WishState.PROCESSING, WishState.PARTIAL)

/**
 * The wishes of the active profile through Seerr (concept 25, ADR 0037; concept 27, 3.1 stage 8):
 * what the profile may request (`getSession` → `wishes`), «Entdecken» (`getWishDiscover`,
 * `searchWishTitles`), the titles outside the library of the search (`searchExternalTitles`), the
 * page of one title (`getExternalTitle`, `getWishSeasons`), «Meine Wünsche» (`listWishes`), the
 * limit (`getWishQuota`), and the two changes a viewer may make: request (`createWish`) and
 * withdraw (`withdrawWish`). Rule 11: nothing of the administration (approve, decline, retry);
 * rule 12: the server decides what the profile sees and may request, the app shows it.
 */
interface WishesRepository {
    /** `null` without an active profile or from an older server (no wishes there). */
    suspend fun rights(server: HttpUrl): ApiResult<WishRights?>

    suspend fun discover(server: HttpUrl): ApiResult<WishDiscover>

    /**
     * The next page of one row of «Entdecken» (`listWishDiscoverRow`, the TV loads a row further at
     * its end). A repository without it ends the row with its first page.
     */
    suspend fun discoverRow(server: HttpUrl, row: DiscoverRowKey, page: Int): ApiResult<DiscoverPage> = ApiResult.NotFound

    /** The wishes area's search (TMDB through Seerr), page by page from 1. */
    suspend fun search(server: HttpUrl, query: String, page: Int): ApiResult<DiscoverPage>

    /** «Noch nicht in der Bibliothek» of the search: titles the profile may request ([media] `null` = both). */
    suspend fun missing(server: HttpUrl, query: String, media: ExternalMedia?, offset: Int, limit: Int): ApiResult<Page<ExternalTitleRef>>

    suspend fun title(server: HttpUrl, media: ExternalMedia, tmdbId: Long): ApiResult<ExternalTitle>

    suspend fun seasons(server: HttpUrl, tmdbId: Long): ApiResult<WishSeasons>

    suspend fun wishes(server: HttpUrl, filter: WishFilter, offset: Int, limit: Int): ApiResult<WishPage>

    suspend fun quota(server: HttpUrl): ApiResult<WishQuota>

    /** 201 a new wish, 200 an existing one joined; 409 `already_available`, 429 `quota_exceeded` are [ApiResult.Failed]. */
    suspend fun create(server: HttpUrl, request: WishCreate): ApiResult<Wish>

    /** 409 `wish_not_withdrawable`: Seerr approved it meanwhile. */
    suspend fun withdraw(server: HttpUrl, id: Long): ApiResult<Unit>
}

class ApiWishesRepository(private val api: ApiCaller, private val http: Call.Factory) : WishesRepository {
    private fun wishes(server: HttpUrl) = SlouchcatApi.create(::WishesApi, server, http)

    override suspend fun rights(server: HttpUrl): ApiResult<WishRights?> =
        api.call("wishes") { SlouchcatApi.create(::AuthApi, server, http).getSession() }.map { it.wishes }

    override suspend fun discover(server: HttpUrl) = api.call("wishes") { wishes(server).getWishDiscover() }

    override suspend fun discoverRow(server: HttpUrl, row: DiscoverRowKey, page: Int) =
        api.call("wishes") { wishes(server).listWishDiscoverRow(row, page) }

    override suspend fun search(server: HttpUrl, query: String, page: Int) =
        api.call("wishes") { wishes(server).searchWishTitles(query, page) }

    override suspend fun missing(server: HttpUrl, query: String, media: ExternalMedia?, offset: Int, limit: Int) =
        api.call("wishes") { wishes(server).searchExternalTitles(query, media, offset, limit) }.map { Page(it.items, it.total) }

    override suspend fun title(server: HttpUrl, media: ExternalMedia, tmdbId: Long) =
        api.call("wishes") { wishes(server).getExternalTitle(media, tmdbId) }

    override suspend fun seasons(server: HttpUrl, tmdbId: Long) = api.call("wishes") { wishes(server).getWishSeasons(tmdbId) }

    override suspend fun wishes(server: HttpUrl, filter: WishFilter, offset: Int, limit: Int) = api.call("wishes") {
        val state = when (filter) {
            WishFilter.All -> null
            WishFilter.Open -> WishesApi.StateListWishes.OPEN
            WishFilter.Available -> WishesApi.StateListWishes.AVAILABLE
            WishFilter.Closed -> WishesApi.StateListWishes.CLOSED
        }
        wishes(server).listWishes(state, offset, limit)
    }

    override suspend fun quota(server: HttpUrl) = api.call("wishes") { wishes(server).getWishQuota() }

    override suspend fun create(server: HttpUrl, request: WishCreate) = api.call("wishes") { wishes(server).createWish(request) }

    override suspend fun withdraw(server: HttpUrl, id: Long) = api.call("wishes") { wishes(server).withdrawWish(id) }
}
