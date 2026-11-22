package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.LibraryFacets
import com.slouchcat.core.api.generated.models.LibraryItemStatus
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.api.map
import com.slouchcat.core.media.Page
import okhttp3.Call
import okhttp3.HttpUrl

/*
 * The data of stage 4 over the generated client, each behind an interface (the seam of the
 * ViewModel tests, fakes in testing/BrowseFakes.kt). Stateless: a feature builds them where it
 * builds its ViewModel, `ApiHomeRepository(app.graph.api, app.graph.http)`. Every answer passed the
 * visibility layer of the active profile; not visible is 404 (`ApiResult.NotFound`).
 */

/** The start page and the lists behind its rows. */
interface HomeRepository {
    /** `getHome`: the rows in the server's order. */
    suspend fun home(server: HttpUrl): ApiResult<HomePage>

    /** `listHomeRow`: the whole list behind a row (`continue`, `recent`, `released`, `spotlight`). */
    suspend fun homeRow(server: HttpUrl, key: HomeListKey, offset: Int, limit: Int): ApiResult<Page<ItemRef>>

    /** `listPicks`: «User-Empfehlungen». */
    suspend fun picks(server: HttpUrl, offset: Int, limit: Int): ApiResult<Page<ItemRef>>
}

/** What the library page asks for (`listLibraryItems` without paging). */
data class LibraryQuery(
    val sort: LibrarySort? = null,
    val descending: Boolean? = null,
    val genre: String? = null,
    val yearFrom: Int? = null,
    val yearTo: Int? = null,
    val status: List<LibraryItemStatus> = emptyList(),
    /** One content kind of a library with several (the chips «Filme», «Serien»). */
    val content: RootContent? = null,
    /** Lowers the profile's ceiling for this page (`maxRating`; the server takes the minimum). */
    val maxRating: Int? = null,
)

/** One page of a library with what it may filter by. */
data class LibraryPage(val items: List<ItemRef>, val total: Int, val facets: LibraryFacets)

/** The libraries and their titles. The list of libraries itself: `graph.librariesSource` (`listVisibleLibraries`). */
interface LibraryRepository {
    /** `listLibraryItems`. */
    suspend fun items(server: HttpUrl, library: Long, query: LibraryQuery, offset: Int, limit: Int): ApiResult<LibraryPage>

    /** `listGenres`: the genres of everything the profile sees, at most 20. */
    suspend fun genres(server: HttpUrl): ApiResult<List<GenreRef>>
}

/** One title and the profile's watchlist. */
interface ItemRepository {
    /** `getItem`. */
    suspend fun item(server: HttpUrl, id: String): ApiResult<ItemDetail>

    /** `addToWatchlist` ([on]) or `removeFromWatchlist`. */
    suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit>
}

class ApiHomeRepository(private val api: ApiCaller, private val http: Call.Factory) : HomeRepository {
    private fun items(server: HttpUrl) = SlouchcatApi.create(::ItemsApi, server, http)

    override suspend fun home(server: HttpUrl) = api.call("home") { items(server).getHome() }

    override suspend fun homeRow(server: HttpUrl, key: HomeListKey, offset: Int, limit: Int) =
        api.call("home-row") { items(server).listHomeRow(key, offset = offset, limit = limit) }.map { Page(it.items, it.total) }

    override suspend fun picks(server: HttpUrl, offset: Int, limit: Int) =
        api.call("picks") { items(server).listPicks(offset, limit) }.map { Page(it.items, it.total) }
}

class ApiLibraryRepository(private val api: ApiCaller, private val http: Call.Factory) : LibraryRepository {
    override suspend fun items(server: HttpUrl, library: Long, query: LibraryQuery, offset: Int, limit: Int) = api.call("library") {
        SlouchcatApi.create(::ItemsApi, server, http).listLibraryItems(
            id = library,
            content = query.content,
            sort = query.sort,
            order = query.descending?.let { if (it) ItemsApi.OrderListLibraryItems.DESC else ItemsApi.OrderListLibraryItems.ASC },
            genre = query.genre,
            yearFrom = query.yearFrom,
            yearTo = query.yearTo,
            maxRating = query.maxRating,
            status = query.status.takeIf { it.isNotEmpty() },
            offset = offset,
            limit = limit,
        )
    }.map { LibraryPage(it.items, it.total, it.facets) }

    override suspend fun genres(server: HttpUrl) = api.call("genres") { SlouchcatApi.create(::DiscoverApi, server, http).listGenres() }
}

class ApiItemRepository(private val api: ApiCaller, private val http: Call.Factory) : ItemRepository {
    override suspend fun item(server: HttpUrl, id: String) = api.call("item") { SlouchcatApi.create(::ItemsApi, server, http).getItem(id) }

    override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = api.call("watchlist") {
        val discover = SlouchcatApi.create(::DiscoverApi, server, http)
        if (on) discover.addToWatchlist(id) else discover.removeFromWatchlist(id)
    }
}
