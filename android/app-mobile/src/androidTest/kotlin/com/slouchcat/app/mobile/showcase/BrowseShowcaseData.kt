package com.slouchcat.app.mobile.showcase

import com.slouchcat.core.api.ApiResult
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
import com.slouchcat.core.api.generated.models.LibraryTile
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.RootContent
import com.slouchcat.core.api.generated.models.SeriesRef
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.HomeRepository
import com.slouchcat.core.media.data.ItemRepository
import com.slouchcat.core.media.data.LibraryPage
import com.slouchcat.core.media.data.LibraryQuery
import com.slouchcat.core.media.data.LibraryRepository
import okhttp3.HttpUrl
import java.math.BigDecimal
import java.time.OffsetDateTime

/**
 * Made-up titles of stage 4 for the showcase (invented names, no real films): films, series with
 * episodes, audiobooks and podcasts, the start page with stage, «Weiterschauen» and rows, three
 * libraries with genres and age ratings. Other areas take titles from [all] or add their own file.
 */
object BrowseShowcaseData {
    private var n = 0
    fun id(): String = "01JSHOWCASE00000000000" + (++n).toString().padStart(5, '0')

    fun image(id: String, kind: String, size: String, width: Int, height: Int) =
        ImageRef("/api/v1/items/$id/images/$kind/$size?v=showcase&p=${SHOWCASE_PROFILE.id}", width, height)

    private fun images(id: String, logo: Boolean, square: Boolean) = ItemImages(
        posterSmall = image(id, "poster", "w200", 200, if (square) 200 else 300),
        poster = image(id, "poster", "w400", 400, if (square) 400 else 600),
        posterLarge = image(id, "poster", "w800", 800, if (square) 800 else 1200),
        backdropSmall = if (square) null else image(id, "backdrop", "w780", 780, 439),
        backdrop = if (square) null else image(id, "backdrop", "w1280", 1280, 720),
        logo = if (logo) image(id, "logo", "w400", 400, 150) else null,
    )

    fun progress(percent: Int, minutes: Int) = Progress(
        positionMs = minutes * 60_000L * percent / 100,
        durationMs = minutes * 60_000L,
        percent = BigDecimal(percent),
        completed = false,
        updatedAt = OffsetDateTime.parse("2026-11-20T20:00:00Z"),
    )

    private fun title(
        name: String,
        kind: ItemKind,
        year: Int,
        rating: Int? = null,
        minutes: Int? = null,
        author: String? = null,
        logo: Boolean = false,
    ): ItemRef {
        val id = id()
        val square = kind == ItemKind.AUDIOBOOK || kind == ItemKind.PODCAST
        return ItemRef(id, kind, name, inWatchlist = false, images = images(id, logo, square), year = year, rating = rating, runtimeMinutes = minutes, author = author)
    }

    val films = listOf(
        title("Nachtzug nach Lumen", ItemKind.MOVIE, 2026, 12, 145, logo = true),
        title("Das gläserne Tal", ItemKind.MOVIE, 2025, 6, 112, logo = true),
        title("Kalter Mond", ItemKind.MOVIE, 2024, 16, 128),
        title("Fünf Brücken", ItemKind.MOVIE, 2023, 12, 97),
        title("Die Uhrmacherin", ItemKind.MOVIE, 2021, 0, 104),
        title("Sturm über Arvik", ItemKind.MOVIE, 2019, 16, 139),
        title("Papierflieger", ItemKind.MOVIE, 2016, 0, 88),
        title("Der stille Hafen", ItemKind.MOVIE, 2012, 12, 121),
        title("Echo im Nebel", ItemKind.MOVIE, 1998, 16, 109),
    )
    val series = listOf(
        title("Nordlicht-Akten", ItemKind.SERIES, 2026, 12, logo = true),
        title("Station Vega", ItemKind.SERIES, 2024, 12),
        title("Die Erben von Halde", ItemKind.SERIES, 2022, 16),
    )
    val books = listOf(
        title("Der Kartograf", ItemKind.AUDIOBOOK, 2023, author = "Lena Aurich"),
        title("Stimmen im Schnee", ItemKind.AUDIOBOOK, 2021, author = "Tomas Ried"),
    )
    val podcasts = listOf(
        title("Morgenrunde", ItemKind.PODCAST, 2026, author = "Radio Beispiel"),
        title("Wissen kompakt", ItemKind.PODCAST, 2025, author = "Studio Muster"),
    )

    /** An episode of [series] that «Weiterschauen» shows as its series. */
    val episode: ItemRef = series[1].let { s ->
        val id = id()
        ItemRef(
            id, ItemKind.EPISODE, "Signal aus der Tiefe", inWatchlist = false,
            images = ItemImages(backdrop = image(id, "backdrop", "w1280", 1280, 720)),
            runtimeMinutes = 48, seasonNumber = 2, episodeNumber = 4,
            seriesRef = SeriesRef(s.id, s.title, s.images), progress = progress(55, 48),
        )
    }

    val all: List<ItemRef> = films + series + books + podcasts

    /** What the pictures write on each id. */
    val names: Map<String, String> = (all + episode).associate { it.id to it.title }

    val squareIds: Set<String> = (books + podcasts).map { it.id }.toSet()

    val libraries = listOf(
        LibraryNav(1, "Filme", LibraryKind.VIDEO, listOf(RootContent.MOVIE), films.size),
        LibraryNav(2, "Serien", LibraryKind.VIDEO, listOf(RootContent.SERIES), series.size),
        LibraryNav(3, "Hörbücher", LibraryKind.AUDIOBOOKS, listOf(RootContent.AUDIOBOOK), books.size),
        LibraryNav(4, "Podcasts", LibraryKind.PODCASTS, listOf(RootContent.PODCAST), podcasts.size),
    )

    private val genreOf: Map<String, List<String>> = all.withIndex().associate { (i, item) ->
        item.id to listOf(listOf("Drama", "Science Fiction", "Komödie", "Thriller")[i % 4], listOf("Abenteuer", "Krimi")[i % 2])
    }

    private fun itemsOf(library: Long) = when (library) {
        1L -> films
        2L -> series
        3L -> books
        else -> podcasts
    }

    private fun row(key: HomeRowKey, items: List<ItemRef>, id: String = key.value, libraryId: Long? = null, libraryName: String? = null) =
        HomeRow(id = id, key = key, items = items, libraryId = libraryId, libraryName = libraryName)

    val home = HomePage(
        rows = listOf(
            row(HomeRowKey.SPOTLIGHT, listOf(films[0], series[0], films[1], films[2], series[2], films[5])),
            row(HomeRowKey.CONTINUE, listOf(episode, films[3].copy(progress = progress(70, 97)), films[6].copy(progress = progress(20, 88)))),
            row(HomeRowKey.RELEASED, films.take(5) + series.take(2)),
            row(HomeRowKey.RECENT, listOf(films[7], series[1], films[8], books[0], films[4], podcasts[0])),
            row(HomeRowKey.PICKS, listOf(series[2], films[3], films[1], films[8])),
            HomeRow(
                id = "libraries", key = HomeRowKey.LIBRARIES, items = emptyList(),
                libraries = libraries.map { LibraryTile(it.id, it.name, it.kind, it.itemCount, itemsOf(it.id).take(4)) },
            ),
            row(HomeRowKey.LIBRARY, series, id = "library:2", libraryId = 2, libraryName = "Serien"),
            row(HomeRowKey.LIBRARY, films.drop(2), id = "library:1", libraryId = 1, libraryName = "Filme"),
            row(HomeRowKey.LIBRARY, podcasts, id = "library:4", libraryId = 4, libraryName = "Podcasts"),
            HomeRow(id = "genres", key = HomeRowKey.GENRES, items = emptyList(), genres = listOf(GenreRef("Drama", 5), GenreRef("Science Fiction", 4), GenreRef("Komödie", 4), GenreRef("Thriller", 3))),
        ),
        heroRow = HomeRowKey.SPOTLIGHT,
        heroStart = 0,
        heroIntervalSeconds = 7,
    )

    private fun <T> page(list: List<T>, offset: Int, limit: Int) = ApiResult.Ok(Page(list.drop(offset).take(limit), list.size))

    /** The repositories of stage 4 over this data, the rest from [base]. */
    fun data(base: MediaData): MediaData = base.copy(
        home = object : HomeRepository {
            override suspend fun home(server: HttpUrl) = ApiResult.Ok(home)
            override suspend fun homeRow(server: HttpUrl, key: HomeListKey, offset: Int, limit: Int) =
                page(home.rows.firstOrNull { it.key.value == key.value }?.items.orEmpty(), offset, limit)
            override suspend fun picks(server: HttpUrl, offset: Int, limit: Int) =
                page(home.rows.first { it.key == HomeRowKey.PICKS }.items, offset, limit)
        },
        library = object : LibraryRepository {
            override suspend fun items(server: HttpUrl, library: Long, query: LibraryQuery, offset: Int, limit: Int): ApiResult<LibraryPage> {
                val list = itemsOf(library).filter { item ->
                    (query.genre == null || query.genre in genreOf[item.id].orEmpty()) &&
                        (query.maxRating == null || (item.rating ?: 18) <= query.maxRating!!) &&
                        (query.yearFrom == null || (item.year ?: 0) >= query.yearFrom!!) &&
                        (query.yearTo == null || (item.year ?: 0) <= query.yearTo!!)
                }.let { if (query.sort?.value == "title") it.sortedBy { item -> item.title } else it }
                val genres = itemsOf(library).flatMap { genreOf[it.id].orEmpty() }.groupingBy { it }.eachCount()
                    .entries.sortedByDescending { it.value }.map { GenreRef(it.key, it.value) }
                val ratings = itemsOf(library).mapNotNull { it.rating }.distinct().sorted()
                return ApiResult.Ok(LibraryPage(list.drop(offset).take(limit), list.size, LibraryFacets(emptyList(), genres, ratings)))
            }
            override suspend fun genres(server: HttpUrl) = ApiResult.Ok(listOf(GenreRef("Drama", 5)))
        },
        items = object : ItemRepository {
            override suspend fun item(server: HttpUrl, id: String): ApiResult<ItemDetail> = ApiResult.NotFound
            override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean) = ApiResult.Ok(Unit)
        },
    )

    /** The pictures of these titles. */
    fun images() = ShowcaseImages(names).also { it.squareIds = squareIds }
}
