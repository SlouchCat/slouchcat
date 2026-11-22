package com.slouchcat.core.browse.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.AudioSeries
import com.slouchcat.core.api.generated.models.AudioSeriesVolume
import com.slouchcat.core.api.generated.models.Chapter
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.RadioFacetCount
import com.slouchcat.core.api.generated.models.RadioFacets
import com.slouchcat.core.api.generated.models.RadioLogo
import com.slouchcat.core.api.generated.models.RadioNow
import com.slouchcat.core.api.generated.models.RadioNowEntry
import com.slouchcat.core.api.generated.models.RadioPreferences
import com.slouchcat.core.api.generated.models.RadioStation
import com.slouchcat.core.api.generated.models.RadioStationState
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.AudioRepository
import com.slouchcat.core.media.data.RadioQuery
import com.slouchcat.core.media.data.RadioRepository
import okhttp3.HttpUrl
import java.math.BigDecimal
import java.time.OffsetDateTime

/*
 * Fakes and sample data of the audio pages (stage 7): an audiobook with chapters in a series, a
 * podcast with episodes, stations of the web radio. Invented names only.
 */

fun chapters(count: Int = 12, minutes: Int = 30) = (0 until count).map { i ->
    Chapter(startMs = i * minutes * 60_000L, endMs = (i + 1) * minutes * 60_000L, title = "Kapitel ${i + 1}")
}

fun audiobook(
    n: Int = 40,
    title: String = "The Quiet Harbour",
    progress: Progress? = null,
    chapters: List<Chapter> = chapters(),
    seriesId: String? = "series-1",
    volume: Int? = 2,
) = film(n, title = title, progress = progress, withCast = false, trailers = 0, images = posters(ulid(n))).copy(
    kind = ItemKind.AUDIOBOOK,
    author = "Mira Lindqvist",
    narrator = "Jonas Brenner",
    year = 2021,
    runtimeMinutes = null,
    durationMs = chapters.lastOrNull()?.endMs ?: 21_600_000,
    chapters = chapters,
    seriesId = seriesId,
    seriesName = seriesId?.let { "Harbour Tales" },
    seriesIndex = volume?.let { BigDecimal(it) },
    libraryId = 4,
    genres = emptyList(),
)

fun episodeRef(n: Int, title: String = "Episode $n", progress: Progress? = null, minutes: Int = 42) = itemRef(
    n,
    title = title,
    kind = ItemKind.PODCAST_EPISODE,
    year = null,
    progress = progress,
    runtimeMinutes = minutes,
).copy(
    durationMs = minutes * 60_000L,
    publishedAt = OffsetDateTime.parse("2026-11-01T06:00:00Z").minusDays(n.toLong() * 7),
    parentId = ulid(60),
)

fun podcast(n: Int = 60, title: String = "Night Shift Stories", episodes: List<ItemRef> = (101..108).map { episodeRef(it) }) =
    film(n, title = title, withCast = false, trailers = 0, images = posters(ulid(n))).copy(
        kind = ItemKind.PODCAST,
        author = "Studio North",
        year = null,
        runtimeMinutes = null,
        episodes = episodes,
        episodeCount = episodes.size,
        genres = emptyList(),
        mediaFiles = emptyList(),
    )

fun podcastEpisode(n: Int = 101, progress: Progress? = null): ItemDetail =
    film(n, title = "Episode $n", progress = progress, withCast = false, trailers = 0, images = posters(ulid(n))).copy(
        kind = ItemKind.PODCAST_EPISODE,
        author = "Studio North",
        podcast = itemRef(60, title = "Night Shift Stories", kind = ItemKind.PODCAST),
        mediaFiles = emptyList(),
        durationMs = 42 * 60_000L,
        genres = emptyList(),
    )

fun audioSeries(volumes: List<ItemRef> = (41..43).map { itemRef(it, title = "Harbour Tales ${it - 40}", kind = ItemKind.AUDIOBOOK, author = "Mira Lindqvist") }) = AudioSeries(
    id = "series-1",
    libraryId = 4,
    name = "Harbour Tales",
    durationMs = volumes.size * 6 * 3_600_000L,
    volumes = volumes.mapIndexed { i, v -> AudioSeriesVolume(v.copy(durationMs = 6 * 3_600_000L), BigDecimal(i + 1), "Jonas Brenner") },
    spellings = emptyList(),
    author = "Mira Lindqvist",
    narrator = "Jonas Brenner",
    cover = volumes.firstOrNull(),
    next = volumes.getOrNull(1),
)

class FakeAudio : AudioRepository {
    var series: ApiResult<AudioSeries> = ApiResult.Ok(audioSeries())
    var episodes: List<ItemRef> = (101..108).map { episodeRef(it) }
    var episodesFailure: ApiResult<Page<ItemRef>>? = null
    val asked = mutableListOf<String>()

    override suspend fun series(server: HttpUrl, libraryId: Long, seriesId: String): ApiResult<AudioSeries> {
        asked += "series:$libraryId:$seriesId"
        return series
    }

    override suspend fun episodes(server: HttpUrl, podcastId: String, offset: Int, limit: Int): ApiResult<Page<ItemRef>> {
        asked += "episodes:$podcastId:$offset"
        episodesFailure?.let { return it }
        return ApiResult.Ok(Page(episodes.drop(offset).take(limit), episodes.size))
    }
}

fun radioStation(id: Long, name: String = "Radio $id", country: String = "CH", favorite: Boolean = false, genres: List<String> = listOf("pop")) = RadioStation(
    id = id,
    name = name,
    languages = listOf("de"),
    genres = genres,
    kinds = emptyList(),
    moods = emptyList(),
    hls = false,
    showsTitles = true,
    favorite = favorite,
    recommended = false,
    state = RadioStationState.OK,
    country = country,
    logo = RadioLogo("radio/images/$id/logo"),
)

class FakeRadio : RadioRepository {
    var stations: List<RadioStation> = (1L..5L).map { radioStation(it, favorite = it == 2L) }
    var favorites: ApiResult<List<RadioStation>> = ApiResult.Ok(listOf(radioStation(2, favorite = true)))
    var preferences: ApiResult<RadioPreferences> = ApiResult.Ok(RadioPreferences(defaultCountry = "CH", effectiveCountry = "CH"))
    var favoriteAnswer: ApiResult<Unit> = ApiResult.Ok(Unit)
    var now: ApiResult<RadioNow> = ApiResult.Ok(RadioNow(true, listOf(RadioNowEntry(1, "Artist - Song", "Artist", "Song"))))
    val queries = mutableListOf<RadioQuery>()
    val calls = mutableListOf<String>()

    override suspend fun stations(server: HttpUrl, query: RadioQuery, offset: Int, limit: Int): ApiResult<Page<RadioStation>> {
        queries += query
        val list = stations.filter { (query.country == null || it.country == query.country) && (!query.favorites || it.favorite) && (query.text.isNullOrBlank() || it.name.contains(query.text!!, ignoreCase = true)) }
        return ApiResult.Ok(Page(list.drop(offset).take(limit), list.size))
    }

    override suspend fun facets(server: HttpUrl, query: RadioQuery) = ApiResult.Ok(
        RadioFacets(
            country = stations.groupingBy { it.country.orEmpty() }.eachCount().map { (c, n) -> RadioFacetCount(c, n) },
            region = emptyList(),
            language = emptyList(),
            genre = emptyList(),
            kind = emptyList(),
            mood = emptyList(),
            quality = emptyList(),
            titles = 0,
            offerEmpty = stations.isEmpty(),
        ),
    )

    override suspend fun favorites(server: HttpUrl) = favorites

    override suspend fun setFavorite(server: HttpUrl, stationId: Long, on: Boolean): ApiResult<Unit> {
        calls += "favorite:$stationId:$on"
        return favoriteAnswer
    }

    override suspend fun preferences(server: HttpUrl) = preferences

    override suspend fun now(server: HttpUrl, stationId: Long) = now
}
