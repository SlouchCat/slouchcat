package com.slouchcat.app.mobile.testing

import com.slouchcat.app.mobile.browse.HomeRepository
import com.slouchcat.app.mobile.browse.ItemRepository
import com.slouchcat.app.mobile.browse.LibraryPage
import com.slouchcat.app.mobile.browse.LibraryQuery
import com.slouchcat.app.mobile.browse.LibraryRepository
import com.slouchcat.app.mobile.media.Page
import com.slouchcat.app.mobile.media.ServerImages
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.map
import com.slouchcat.core.api.generated.models.GenreRef
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.HomePage
import com.slouchcat.core.api.generated.models.HomeRow
import com.slouchcat.core.api.generated.models.HomeRowKey
import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.LibraryFacets
import com.slouchcat.core.api.generated.models.LibraryKind
import com.slouchcat.core.api.generated.models.LibraryNav
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.RootContent
import kotlinx.coroutines.CompletableDeferred
import okhttp3.HttpUrl
import java.math.BigDecimal
import java.time.OffsetDateTime

/*
 * Fakes and sample data of stage 4 «Stöbern» (testing/Fakes.kt stays untouched). Every fake
 * records what it was asked and answers from a field a test may change between calls.
 */

/** A ULID-shaped id from a short number (the server's ids are ULIDs). */
fun ulid(n: Int): String = "01J0000000000000000000" + n.toString().padStart(4, '0')

/** An image of the server at [path] (below the API) in [width]. */
fun image(path: String, width: Int, profile: Long = 1): ImageRef =
    ImageRef("/api/v1/$path?v=hash&p=$profile", width, width * 3 / 2)

/** The three poster sizes of [id] as the server lists them. */
fun posters(id: String, profile: Long = 1) = ItemImages(
    posterSmall = image("items/$id/images/poster/w200", 200, profile),
    poster = image("items/$id/images/poster/w400", 400, profile),
    posterLarge = image("items/$id/images/poster/w800", 800, profile),
    backdrop = image("items/$id/images/backdrop/w1280", 1280, profile),
)

fun itemRef(
    n: Int,
    title: String = "Title $n",
    kind: ItemKind = ItemKind.MOVIE,
    year: Int? = 2000 + n % 25,
    images: ItemImages = ItemImages(),
    rating: Int? = null,
    progress: Progress? = null,
    author: String? = null,
    inWatchlist: Boolean = false,
    runtimeMinutes: Int? = null,
) = ItemRef(
    id = ulid(n),
    kind = kind,
    title = title,
    inWatchlist = inWatchlist,
    images = images,
    year = year,
    rating = rating,
    progress = progress,
    author = author,
    runtimeMinutes = runtimeMinutes,
)

/** A progress at [percent] of a title of [minutes]. */
fun progress(percent: Int, minutes: Int = 100, watched: Boolean = false) = Progress(
    positionMs = minutes * 60_000L * percent / 100,
    durationMs = minutes * 60_000L,
    percent = BigDecimal(percent),
    completed = percent >= 90,
    updatedAt = OffsetDateTime.parse("2026-11-20T20:00:00Z"),
    watchedAt = if (watched) OffsetDateTime.parse("2026-11-20T21:00:00Z") else null,
)

fun homeRow(key: HomeRowKey, items: List<ItemRef>, id: String = key.value, libraryId: Long? = null, libraryName: String? = null, genres: List<GenreRef>? = null) =
    HomeRow(id = id, key = key, items = items, libraryId = libraryId, libraryName = libraryName, genres = genres)

fun library(id: Long, name: String, kind: LibraryKind = LibraryKind.VIDEO, count: Int = 10, contents: List<RootContent> = listOf(RootContent.MOVIE)) =
    LibraryNav(id, name, kind, contents, count)

fun facets(genres: List<GenreRef> = listOf(GenreRef("Drama", 3), GenreRef("Comedy", 2))) =
    LibraryFacets(letters = emptyList(), genres = genres, ratings = listOf(0, 12, 16))

/** The image addresses of [HTTP_SERVER]. */
val SERVER_IMAGES = ServerImages(HTTP_SERVER.address)

class FakeHome(
    var page: ApiResult<HomePage> = ApiResult.Ok(HomePage(emptyList())),
) : HomeRepository {
    /** The whole list behind each row; [homeRow] cuts its pages. */
    var lists: MutableMap<HomeListKey, ApiResult<List<ItemRef>>> = mutableMapOf()
    var picks: ApiResult<List<ItemRef>> = ApiResult.Ok(emptyList())
    val asked = mutableListOf<String>()

    /** When set, [home] waits for it (a load still under way). */
    var gate: CompletableDeferred<Unit>? = null

    override suspend fun home(server: HttpUrl): ApiResult<HomePage> {
        asked += "home"
        gate?.await()
        return page
    }

    override suspend fun homeRow(server: HttpUrl, key: HomeListKey, offset: Int, limit: Int): ApiResult<Page<ItemRef>> {
        asked += "row:${key.value}:$offset"
        return cut(lists[key] ?: ApiResult.NotFound, offset, limit)
    }

    override suspend fun picks(server: HttpUrl, offset: Int, limit: Int): ApiResult<Page<ItemRef>> {
        asked += "picks:$offset"
        return cut(picks, offset, limit)
    }
}

class FakeLibrary(var all: ApiResult<List<ItemRef>> = ApiResult.Ok(emptyList())) : LibraryRepository {
    var facets: LibraryFacets = facets()
    var genres: ApiResult<List<GenreRef>> = ApiResult.Ok(listOf(GenreRef("Drama", 3)))
    val queries = mutableListOf<Pair<LibraryQuery, Int>>()

    /** What the server filters: by genre only (enough to see that the query arrives). */
    var genreOf: (ItemRef) -> String? = { null }

    override suspend fun items(server: HttpUrl, library: Long, query: LibraryQuery, offset: Int, limit: Int): ApiResult<LibraryPage> {
        queries += query to offset
        val filtered = when (val list = all) {
            is ApiResult.Ok -> ApiResult.Ok(list.value.filter { query.genre == null || genreOf(it) == query.genre })
            else -> list
        }
        return cut(filtered, offset, limit).map { LibraryPage(it.items, it.total, facets) }
    }

    override suspend fun genres(server: HttpUrl) = genres
}

class FakeItems(var detail: ApiResult<ItemDetail> = ApiResult.NotFound) : ItemRepository {
    var watchlist: ApiResult<Unit> = ApiResult.Ok(Unit)
    val watchlistCalls = mutableListOf<Pair<String, Boolean>>()

    override suspend fun item(server: HttpUrl, id: String) = detail

    override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit> {
        watchlistCalls += id to on
        return watchlist
    }
}

private fun cut(list: ApiResult<List<ItemRef>>, offset: Int, limit: Int): ApiResult<Page<ItemRef>> =
    list.map { Page(it.drop(offset).take(limit), it.size) }
