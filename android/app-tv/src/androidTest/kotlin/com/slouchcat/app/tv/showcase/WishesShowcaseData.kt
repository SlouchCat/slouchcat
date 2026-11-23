package com.slouchcat.app.tv.showcase

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DiscoverEntry
import com.slouchcat.core.api.generated.models.DiscoverPage
import com.slouchcat.core.api.generated.models.DiscoverRow
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitle
import com.slouchcat.core.api.generated.models.ExternalTitleImages
import com.slouchcat.core.api.generated.models.ExternalTitleRef
import com.slouchcat.core.api.generated.models.ExternalTitleSeason
import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.NotificationPage
import com.slouchcat.core.api.generated.models.NotificationWish
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.api.generated.models.Wish
import com.slouchcat.core.api.generated.models.WishCounts
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishDiscover
import com.slouchcat.core.api.generated.models.WishMark
import com.slouchcat.core.api.generated.models.WishPage
import com.slouchcat.core.api.generated.models.WishProgress
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishQuotaPart
import com.slouchcat.core.api.generated.models.WishRequestCard
import com.slouchcat.core.api.generated.models.WishRights
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.api.generated.models.WishSeasonState
import com.slouchcat.core.api.generated.models.WishSeasons
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.NotificationsRepository
import com.slouchcat.core.media.data.OPEN_WISH_STATES
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.media.data.WishesRepository
import okhttp3.HttpUrl
import java.time.OffsetDateTime

/**
 * Made-up wishes, inbox and announcement of stage 5 for the showcase (invented titles, no real
 * films): Seerr's rows of films and series, three own wishes (asked, on its way with its share, there),
 * the pages of the titles, the seasons of a series, the limit; the inbox with a new episode, a
 * fulfilled and a declined wish and an announcement. [off]: Seerr not set up; [empty]: no wish, an
 * empty inbox.
 */
class WishesShowcaseData(private val off: Boolean = false, private val empty: Boolean = false) {
    private val at: OffsetDateTime = OffsetDateTime.parse("2026-11-22T19:30:00Z")

    private fun images(media: ExternalMedia, id: Long) = ExternalTitleImages(
        posterSmall = ImageRef("/api/v1/titles/${media.value}/$id/images/poster/w200", 200, 300),
        poster = ImageRef("/api/v1/titles/${media.value}/$id/images/poster/w400", 400, 600),
        backdrop = ImageRef("/api/v1/titles/${media.value}/$id/images/backdrop/w1280", 1280, 720),
    )

    private class Made(val media: ExternalMedia, val id: Long, val title: String, val year: Int, val overview: String, val minutes: Int? = null, val seasons: Int = 0)

    private val made = listOf(
        Made(ExternalMedia.MOVIE, 7101, "Gläserne Stadt", 2026, "Eine Architektin kehrt in die Stadt ihrer Kindheit zurück, deren Türme aus Glas über Nacht zu singen beginnen.", minutes = 113),
        Made(ExternalMedia.MOVIE, 7102, "Der letzte Leuchtturm", 2025, "Ein Wärter, ein Sturm und ein Funkspruch, der nie hätte ankommen dürfen.", minutes = 98),
        Made(ExternalMedia.MOVIE, 7103, "Funkstille", 2025, "Zwei Brüder suchen den Sender, der ihre Stadt jede Nacht um drei weckt.", minutes = 104),
        Made(ExternalMedia.MOVIE, 7104, "Mondlauf", 2024, "Eine Läuferin trainiert auf einer Station am Rand des Mondes für ein Rennen, das es nicht mehr gibt.", minutes = 121),
        Made(ExternalMedia.MOVIE, 7105, "Kartenhaus", 2024, "Ein Spiel um ein altes Hotel, bei dem jede Karte eine Tür öffnet.", minutes = 92),
        Made(ExternalMedia.MOVIE, 7106, "Nebelwald", 2023, "Ein Dorf verschwindet jeden Herbst für drei Tage im Nebel.", minutes = 109),
        Made(ExternalMedia.MOVIE, 7107, "Die Brücke von Aaren", 2022, "Wer baute die Brücke, die auf keiner Karte steht?", minutes = 117),
        Made(ExternalMedia.SHOW, 7201, "Signalfeuer", 2026, "Eine Küstenwache und eine Reihe von Lichtern, die niemand angezündet hat.", seasons = 2),
        Made(ExternalMedia.SHOW, 7202, "Hafenlichter", 2024, "Diese Serie erzählt von drei Familien am Hafen einer kleinen Stadt im Norden.", seasons = 3),
        Made(ExternalMedia.SHOW, 7203, "Die stillen Jahre", 2023, "Ein Chor, ein Dorf und zehn Jahre ohne Musik.", seasons = 1),
        Made(ExternalMedia.SHOW, 7204, "Eiszeit-Chroniken", 2025, "Forscherinnen überwintern in einer Station unter dem Eis.", seasons = 4),
    )

    /** What the pictures write on each title (`movie-7101`). */
    val names: Map<String, String> = made.associate { "${it.media.value}-${it.id}" to it.title }

    private fun made(id: Long) = made.first { it.id == id }

    private val wishes: MutableList<Wish> = if (empty) mutableListOf() else mutableListOf(
        wish(1, made(7101), WishState.PENDING),
        wish(2, made(7201), WishState.PROCESSING, seasons = listOf(1), progress = WishProgress(WishProgress.Stage.DOWNLOAD, at, percent = 64)),
        wish(3, made(7104), WishState.AVAILABLE),
    )

    private fun wish(id: Long, title: Made, state: WishState, seasons: List<Int> = emptyList(), progress: WishProgress? = null) = Wish(
        id = id,
        media = title.media,
        tmdbId = title.id,
        title = title.title,
        state = state,
        origin = Wish.Origin.SLOUCHCAT,
        role = Wish.Role.OWNER,
        seasons = seasons.map { WishSeasonState(it, state) },
        canWithdraw = state in OPEN_WISH_STATES && state != WishState.PROCESSING,
        canRequestAgain = false,
        createdAt = at.minusDays(id),
        updatedAt = at,
        events = emptyList(),
        year = title.year,
        external = ExternalTitleRef(title.media, title.id, title.title, images(title.media, title.id), hasTrailer = false, canRequest = false, year = title.year),
        progress = progress,
    )

    private fun ref(title: Made): ExternalTitleRef {
        val own = wishes.firstOrNull { it.tmdbId == title.id && it.state in OPEN_WISH_STATES }
        return ExternalTitleRef(title.media, title.id, title.title, images(title.media, title.id), hasTrailer = false, canRequest = own == null, year = title.year, wish = own?.let { WishMark(it.id, it.state) })
    }

    private val quota = WishQuota(unlimited = false, movie = WishQuotaPart(10, 7, 2, 0, 8), tv = WishQuotaPart(5, 7, 1, 0, 4))

    private fun row(key: DiscoverRowKey, ids: List<Long>) = DiscoverRow(key, ids.map { DiscoverEntry(ref(made(it)), seerrRequested = it == 7103L) }, emptyList(), stale = false)

    private fun counts(): WishCounts {
        val open = wishes.count { it.state in OPEN_WISH_STATES }
        val available = wishes.count { it.state == WishState.AVAILABLE }
        return WishCounts(open, wishes.count { it.state == WishState.PENDING }, wishes.count { it.state == WishState.PROCESSING }, 0, available, wishes.size - open - available)
    }

    val repository: WishesRepository = object : WishesRepository {
        override suspend fun rights(server: HttpUrl): ApiResult<WishRights?> =
            ApiResult.Ok(WishRights(enabled = !off, canRequest = !off, movies = true, shows = true, discover = !off))

        override suspend fun discover(server: HttpUrl) = ApiResult.Ok(
            WishDiscover(
                listOf(
                    row(DiscoverRowKey.POPULAR_MOVIES, listOf(7101, 7102, 7103, 7104, 7105, 7106, 7107)),
                    row(DiscoverRowKey.POPULAR_SHOWS, listOf(7202, 7201, 7203, 7204)),
                    row(DiscoverRowKey.UPCOMING_MOVIES, listOf(7106, 7107, 7105)),
                ),
            ),
        )

        override suspend fun search(server: HttpUrl, query: String, page: Int): ApiResult<DiscoverPage> {
            val hits = made.filter { it.title.contains(query, ignoreCase = true) || it.overview.contains(query, ignoreCase = true) }
            return ApiResult.Ok(DiscoverPage(hits.map { DiscoverEntry(ref(it), false) }, page, stale = false, fetchedAt = at, totalPages = 1))
        }

        override suspend fun missing(server: HttpUrl, query: String, media: ExternalMedia?, offset: Int, limit: Int) =
            ApiResult.Ok(Page(made.filter { it.title.contains(query, ignoreCase = true) }.map(::ref), 0))

        override suspend fun title(server: HttpUrl, media: ExternalMedia, tmdbId: Long): ApiResult<ExternalTitle> {
            val title = made.firstOrNull { it.id == tmdbId } ?: return ApiResult.NotFound
            val own = wishes.firstOrNull { it.tmdbId == tmdbId && it.state in OPEN_WISH_STATES }
            return ApiResult.Ok(
                ExternalTitle(
                    media = title.media,
                    tmdbId = title.id,
                    title = title.title,
                    images = images(title.media, title.id),
                    hasTrailer = false,
                    canRequest = own == null,
                    genres = listOf("Drama"),
                    seasons = (1..title.seasons).map { ExternalTitleSeason(it, 8, ExternalTitleSeason.State.MISSING) },
                    trailers = emptyList(),
                    request = WishRequestCard(own == null, autoApprove = false, quota = quota),
                    year = title.year,
                    minAge = 16,
                    wish = own?.let { WishMark(it.id, it.state) },
                    overview = title.overview,
                    runtimeMinutes = title.minutes,
                ),
            )
        }

        override suspend fun seasons(server: HttpUrl, tmdbId: Long) =
            ApiResult.Ok(WishSeasons(WishSeasons.Source.SEERR, (1..made(tmdbId).seasons).map { WishSeasonOffer(it, 8, WishSeasonOffer.State.MISSING) }))

        override suspend fun wishes(server: HttpUrl, filter: WishFilter, offset: Int, limit: Int) =
            ApiResult.Ok(WishPage(wishes.drop(offset).take(limit), wishes.size, counts()))

        override suspend fun quota(server: HttpUrl) = ApiResult.Ok(quota)

        override suspend fun create(server: HttpUrl, request: WishCreate): ApiResult<Wish> = ApiResult.Offline

        override suspend fun withdraw(server: HttpUrl, id: Long): ApiResult<Unit> = ApiResult.Offline
    }

    private val inbox: List<Notification> = if (empty) emptyList() else listOf(
        Notification(1, Notification.Kind.NEW_EPISODE, at.minusHours(2), item = BrowseShowcaseData.episode, parent = BrowseShowcaseData.series[1]),
        Notification(2, Notification.Kind.WISH_AVAILABLE, at.minusDays(1), wish = NotificationWish(3, ExternalMedia.MOVIE, "Mondlauf", WishState.AVAILABLE)),
        Notification(3, Notification.Kind.WISH_DECLINED, at.minusDays(3), readAt = at.minusDays(2), wish = NotificationWish(4, ExternalMedia.MOVIE, "Kartenhaus", WishState.DECLINED)),
    )

    private val announcements: List<ProfileAnnouncement> =
        if (empty) emptyList() else listOf(ProfileAnnouncement(1, "Wartung am Sonntag 02:00 bis 03:00", "Slouchcat ist in dieser Zeit nicht erreichbar.", at))

    val notifications: NotificationsRepository = object : NotificationsRepository {
        override suspend fun page(server: HttpUrl, offset: Int, limit: Int) =
            ApiResult.Ok(NotificationPage(inbox.drop(offset).take(limit), inbox.size, inbox.count { it.readAt == null }, offset, limit))

        override suspend fun markRead(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)

        override suspend fun markAllRead(server: HttpUrl) = ApiResult.Ok(Unit)

        override suspend fun announcements(server: HttpUrl) = ApiResult.Ok(announcements)

        override suspend fun acknowledge(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
    }

    /** The pages of [PagesShowcaseData] with these wishes and this inbox. */
    fun data(base: MediaData): MediaData = PagesShowcaseData.data(base).copy(wishes = repository, notifications = notifications)
}
