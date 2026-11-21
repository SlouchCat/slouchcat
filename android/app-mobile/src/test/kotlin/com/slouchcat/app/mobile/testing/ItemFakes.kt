package com.slouchcat.app.mobile.testing

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.CastMember
import com.slouchcat.core.api.generated.models.CollectionRef
import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
import com.slouchcat.core.api.generated.models.NextUp
import com.slouchcat.core.api.generated.models.Person
import com.slouchcat.core.api.generated.models.PersonCreditsInner
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.SeriesRef
import com.slouchcat.core.api.generated.models.Trailer
import com.slouchcat.core.api.generated.models.UserPickResult
import com.slouchcat.core.media.data.DetailRepository
import com.slouchcat.core.media.data.PersonRepository
import kotlinx.coroutines.CompletableDeferred
import okhttp3.HttpUrl
import java.net.URI
import java.time.LocalDate
import java.time.OffsetDateTime

/*
 * Fakes and sample data of the detail pages and the person page (stage 4, part b). The film,
 * the series with two seasons and the person carry images and a cast, which the test server's
 * synthetic media lack.
 */

/** Poster, backdrop and logo of [id] as the server lists them. */
fun detailImages(id: String) = posters(id).copy(
    backdropSmall = image("items/$id/images/backdrop/w780", 780),
    backdropLarge = image("items/$id/images/backdrop/w1920", 1920),
    logo = image("items/$id/images/logo/w500", 500),
)

fun portrait(person: Long): ImageRef = image("people/$person/image", 300)

val LONG_OVERVIEW = "Four years after the events of the last film the hero lives alone. " +
    "He fights crime in a city that no longer knows his name, while an old enemy returns. " +
    "Friends become strangers, strangers become friends, and every choice has a price that " +
    "somebody else will pay. The summary goes on long enough to need more than four lines."

fun cast() = listOf(
    CastMember(101, "Ann Actor", CastMember.Role.CAST, character = "The Hero", image = portrait(101)),
    CastMember(102, "Ben Player", CastMember.Role.CAST, character = "The Friend", image = portrait(102)),
    CastMember(103, "Cid Director", CastMember.Role.DIRECTOR),
)

fun mediaFile(id: String, label: String? = null) = MediaFileInfo(
    id = id,
    container = "mkv",
    sizeBytes = 2_100_000_000,
    available = true,
    videoCodec = "hevc",
    audioCodec = "eac3",
    width = 1920,
    height = 1080,
    versionLabel = label,
)

fun film(
    n: Int = 1,
    title: String = "Night Train",
    progress: Progress? = null,
    inWatchlist: Boolean = false,
    recommended: Boolean = false,
    canRecommend: Boolean = true,
    overview: String? = LONG_OVERVIEW,
    withCast: Boolean = true,
    images: ItemImages = detailImages(ulid(n)),
    trailers: Int = 1,
) = ItemDetail(
    id = ulid(n),
    kind = ItemKind.MOVIE,
    title = title,
    inWatchlist = inWatchlist,
    images = images,
    recommendedByMe = recommended,
    canRecommend = canRecommend,
    collections = listOf(
        CollectionRef(ulid(900), "Night Train Collection", 2, members = listOf(itemRef(n, title = title, images = posters(ulid(n))), itemRef(2, title = "Night Train 2", images = posters(ulid(2))))),
    ),
    genres = listOf("Action", "Drama"),
    mediaFiles = listOf(mediaFile("f1", "Kinofassung"), mediaFile("f2", "Director's Cut")),
    year = 2026,
    overview = overview,
    runtimeMinutes = 145,
    rating = 12,
    progress = progress,
    tagline = "A new day begins.",
    cast = if (withCast) cast() else null,
    trailerCount = trailers,
)

fun episode(season: Int, number: Int, seriesId: String, seasonId: String, progress: Progress? = null, title: String = "Episode $season-$number") = ItemRef(
    id = ulid(1000 + season * 100 + number),
    kind = ItemKind.EPISODE,
    title = title,
    inWatchlist = false,
    images = ItemImages(backdropSmall = image("items/${ulid(1000 + season * 100 + number)}/images/backdrop/w780", 780)),
    overview = "What happens in episode $number.",
    runtimeMinutes = 33,
    progress = progress,
    parentId = seasonId,
    seasonNumber = season,
    episodeNumber = number,
    seriesRef = SeriesRef(seriesId, "Quiet Ring", posters(seriesId)),
)

val SERIES_ID = ulid(200)
val SEASON_1 = ulid(201)
val SEASON_2 = ulid(202)

fun seasonRef(n: Int, watched: Int = 0, episodes: Int = 3) = ItemRef(
    id = if (n == 1) SEASON_1 else SEASON_2,
    kind = ItemKind.SEASON,
    title = "Season $n",
    inWatchlist = false,
    images = posters(if (n == 1) SEASON_1 else SEASON_2),
    parentId = SERIES_ID,
    seasonNumber = n,
    childCount = episodes,
    watchedCount = watched,
)

fun seasonDetail(n: Int, watched: Int = 0, resumeAt: Int? = null): ItemDetail {
    val ref = seasonRef(n, watched)
    val episodes = (1..3).map { e ->
        val p = when {
            e <= watched -> progress(100, minutes = 33, watched = true)
            e == resumeAt -> progress(40, minutes = 33)
            else -> null
        }
        episode(n, e, SERIES_ID, ref.id, p)
    }
    return ItemDetail(
        id = ref.id, kind = ItemKind.SEASON, title = ref.title, inWatchlist = false, images = ref.images,
        recommendedByMe = false, canRecommend = false, collections = emptyList(), genres = emptyList(), mediaFiles = emptyList(),
        parentId = SERIES_ID, seasonNumber = n, childCount = 3, watchedCount = watched, episodes = episodes,
        series = ItemRef(SERIES_ID, ItemKind.SERIES, "Quiet Ring", false, posters(SERIES_ID)),
    )
}

fun series(watched: Boolean = false, watchedCount: Int = 0, resume: Boolean = true) = ItemDetail(
    id = SERIES_ID,
    kind = ItemKind.SERIES,
    title = "Quiet Ring",
    inWatchlist = false,
    images = detailImages(SERIES_ID),
    recommendedByMe = false,
    canRecommend = true,
    collections = emptyList(),
    genres = listOf("Fantasy"),
    mediaFiles = emptyList(),
    year = 2022,
    overview = "Evil has had many names.",
    rating = 16,
    watched = watched,
    watchedCount = watchedCount,
    seasons = listOf(seasonRef(1, watched = 2), seasonRef(2)),
    nextUp = NextUp(
        if (resume) NextUp.Reason.RESUME else NextUp.Reason.START,
        episode(1, 3, SERIES_ID, SEASON_1, if (resume) progress(40, minutes = 33) else null, title = "Cowardice"),
    ),
    cast = cast(),
    trailerCount = 0,
)

fun person(id: Long = 101, credits: List<PersonCreditsInner> = listOf(
    PersonCreditsInner(itemRef(1, title = "Night Train", images = posters(ulid(1))), PersonCreditsInner.Role.CAST, character = "The Hero"),
    PersonCreditsInner(itemRef(1, title = "Night Train", images = posters(ulid(1))), PersonCreditsInner.Role.DIRECTOR),
    PersonCreditsInner(itemRef(2, title = "Night Train 2", images = posters(ulid(2))), PersonCreditsInner.Role.CAST, character = "The Hero"),
)) = Person(
    id = id,
    name = "Ann Actor",
    credits = credits,
    image = portrait(id),
    biography = LONG_OVERVIEW,
    biographyLang = "en",
    birthday = LocalDate.of(1996, 6, 1),
    placeOfBirth = "Example Town",
    knownForDepartment = "Acting",
    totalCredits = 12,
)

fun trailer(youtube: String = "abcDEF12345") = Trailer(1, "de", youtube, URI("https://www.youtube-nocookie.com/embed/$youtube"), Trailer.Source.TMDB, official = true)

/** Answers every `getItem` from [details] by id (404 for an unknown one) and records each change. */
class FakeDetails(vararg items: ItemDetail) : DetailRepository {
    val details: MutableMap<String, ApiResult<ItemDetail>> = items.associate { it.id to ApiResult.Ok(it) as ApiResult<ItemDetail> }.toMutableMap()
    var trailers: ApiResult<List<Trailer>> = ApiResult.Ok(listOf(trailer()))
    var watchlist: ApiResult<Unit> = ApiResult.Ok(Unit)
    var watched: ApiResult<Unit> = ApiResult.Ok(Unit)
    var pick: ApiResult<UserPickResult> = ApiResult.Ok(UserPickResult(OffsetDateTime.parse("2026-12-20T20:00:00Z"), 30))
    var unpick: ApiResult<Unit> = ApiResult.Ok(Unit)
    val asked = mutableListOf<String>()
    val changes = mutableListOf<String>()

    /** When set, a change waits for it (the answer of the server still under way). */
    var gate: CompletableDeferred<Unit>? = null

    override suspend fun item(server: HttpUrl, id: String): ApiResult<ItemDetail> {
        asked += id
        return details[id] ?: ApiResult.NotFound
    }

    override suspend fun setWatchlist(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit> {
        changes += "watchlist:$id:$on"
        gate?.await()
        return watchlist
    }

    override suspend fun trailers(server: HttpUrl, id: String) = trailers

    override suspend fun setWatched(server: HttpUrl, id: String, on: Boolean): ApiResult<Unit> {
        changes += "watched:$id:$on"
        gate?.await()
        return watched
    }

    override suspend fun recommend(server: HttpUrl, id: String): ApiResult<UserPickResult> {
        changes += "pick:$id"
        gate?.await()
        return pick
    }

    override suspend fun unrecommend(server: HttpUrl, id: String): ApiResult<Unit> {
        changes += "unpick:$id"
        gate?.await()
        return unpick
    }
}

class FakePeople(var answer: ApiResult<Person> = ApiResult.Ok(person())) : PersonRepository {
    val asked = mutableListOf<Long>()

    override suspend fun person(server: HttpUrl, id: Long): ApiResult<Person> {
        asked += id
        return answer
    }
}
