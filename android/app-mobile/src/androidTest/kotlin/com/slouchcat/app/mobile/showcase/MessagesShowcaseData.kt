package com.slouchcat.app.mobile.showcase

import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSetting
import com.slouchcat.core.api.generated.models.ChannelSettingUpdate
import com.slouchcat.core.api.generated.models.ChannelSettings
import com.slouchcat.core.api.generated.models.DiscoverEntry
import com.slouchcat.core.api.generated.models.DiscoverPage
import com.slouchcat.core.api.generated.models.DiscoverRow
import com.slouchcat.core.api.generated.models.DiscoverRowKey
import com.slouchcat.core.api.generated.models.EmailChannel
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.ExternalTitle
import com.slouchcat.core.api.generated.models.ExternalTitleImages
import com.slouchcat.core.api.generated.models.ExternalTitleRef
import com.slouchcat.core.api.generated.models.ImageRef
import com.slouchcat.core.api.generated.models.Notification
import com.slouchcat.core.api.generated.models.NotificationPage
import com.slouchcat.core.api.generated.models.NotificationWish
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.api.generated.models.PushChannel
import com.slouchcat.core.api.generated.models.PushSubscriptionRequest
import com.slouchcat.core.api.generated.models.PushSubscriptionSummary
import com.slouchcat.core.api.generated.models.TelegramChannel
import com.slouchcat.core.api.generated.models.Wish
import com.slouchcat.core.api.generated.models.WishCounts
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishDiscover
import com.slouchcat.core.api.generated.models.WishMark
import com.slouchcat.core.api.generated.models.WishPage
import com.slouchcat.core.api.generated.models.WishQuota
import com.slouchcat.core.api.generated.models.WishQuotaPart
import com.slouchcat.core.api.generated.models.WishRequestCard
import com.slouchcat.core.api.generated.models.WishRights
import com.slouchcat.core.api.generated.models.WishSeasonOffer
import com.slouchcat.core.api.generated.models.WishSeasonState
import com.slouchcat.core.api.generated.models.WishSeasons
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.media.DevicePush
import com.slouchcat.core.media.DevicePushState
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.ChannelsRepository
import com.slouchcat.core.media.data.NotificationsRepository
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.media.data.WishesRepository
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.HttpUrl
import java.time.OffsetDateTime

/**
 * Made-up wishes, inbox and channels of stage 8 for the showcase (invented titles, drawn pictures,
 * no server). [wishesOff]: Seerr is not set up; [emptyWishes]: no wish yet.
 */
class MessagesShowcaseData(private val wishesOff: Boolean = false, private val emptyWishes: Boolean = false) {
    private val at = OffsetDateTime.parse("2026-11-22T16:00:00Z")

    private fun image(id: String, kind: String, size: String, width: Int, height: Int) =
        ImageRef("/api/v1/items/$id/images/$kind/$size?v=showcase&p=${SHOWCASE_PROFILE.id}", width, height)

    private fun images(id: String) = ExternalTitleImages(
        posterSmall = image(id, "poster", "w200", 200, 300),
        poster = image(id, "poster", "w400", 400, 600),
        backdrop = image(id, "backdrop", "w1280", 1280, 720),
    )

    private val titles = listOf(
        Triple(9101L, "Glasvogel", ExternalMedia.MOVIE),
        Triple(9102L, "Kupfermond", ExternalMedia.MOVIE),
        Triple(9103L, "Nebelhafen", ExternalMedia.SHOW),
        Triple(9104L, "Sternenfeld", ExternalMedia.MOVIE),
        Triple(9105L, "Rotes Ufer", ExternalMedia.MOVIE),
        Triple(9106L, "Winterhaus", ExternalMedia.SHOW),
        Triple(9107L, "Die Salzinsel", ExternalMedia.MOVIE),
        Triple(9108L, "Lichtjahre", ExternalMedia.SHOW),
    )

    val names: Map<String, String> = titles.associate { (id, name, _) -> "t$id" to name }

    private fun ref(index: Int, wish: WishState? = null) = titles[index].let { (id, name, media) ->
        ExternalTitleRef(media, id, name, images("t$id"), hasTrailer = false, canRequest = wish == null, year = 2020 + index, minAge = 12, wish = wish?.let { WishMark(id, it) })
    }

    val requestedId = 9101L
    /** A wish Seerr already approved: no «Zurückziehen» (code review M-1). */
    val approvedId = 9102L
    val seriesId = 9103L

    private val wishes = if (emptyWishes) emptyList() else listOf(
        wish(0, WishState.PENDING),
        wish(1, WishState.PROCESSING),
        wish(2, WishState.PARTIAL, seasons = listOf(1, 2)),
        wish(4, WishState.AVAILABLE),
        wish(6, WishState.DECLINED),
    )

    private fun wish(index: Int, state: WishState, seasons: List<Int> = emptyList()) = titles[index].let { (id, name, media) ->
        Wish(
            id = id, media = media, tmdbId = id, title = name, state = state, origin = Wish.Origin.SLOUCHCAT, role = Wish.Role.OWNER,
            seasons = seasons.map { WishSeasonState(it, state) }, canWithdraw = state == WishState.PENDING, canRequestAgain = false,
            createdAt = at.minusDays(index.toLong()), updatedAt = at, events = emptyList(), year = 2020 + index, external = ref(index),
        )
    }

    private val quota = WishQuota(unlimited = false, movie = WishQuotaPart(5, 7, 1, 0, 4), tv = WishQuotaPart(5, 7, 1, 0, 4))

    private fun title(id: Long): ExternalTitle? {
        val index = titles.indexOfFirst { it.first == id }.takeIf { it >= 0 } ?: return null
        val (_, name, media) = titles[index]
        val open = wishes.firstOrNull { it.tmdbId == id && (it.state == WishState.PENDING || it.state == WishState.PROCESSING) }
        return ExternalTitle(
            media = media, tmdbId = id, title = name, images = images("t$id"), hasTrailer = false, canRequest = open == null,
            genres = listOf("Drama"), seasons = emptyList(), trailers = emptyList(),
            request = WishRequestCard(open == null, autoApprove = false, quota = quota), year = 2020 + index, minAge = 12,
            wish = open?.let { WishMark(it.id, it.state) }, runtimeMinutes = if (media == ExternalMedia.MOVIE) 113 else null,
            overview = "Eine Vogelwarte an der Küste, ein Sommer ohne Wind und ein Fund im Sand, der das Dorf entzweit.",
        )
    }

    private val wishesRepo = object : WishesRepository {
        override suspend fun rights(server: HttpUrl): ApiResult<WishRights?> =
            ApiResult.Ok(WishRights(enabled = !wishesOff, canRequest = !wishesOff, movies = true, shows = true, discover = !wishesOff))
        override suspend fun discover(server: HttpUrl) = ApiResult.Ok(
            WishDiscover(
                listOf(
                    DiscoverRow(DiscoverRowKey.TRENDING, (0..3).map { DiscoverEntry(ref(it, if (it == 0) WishState.PENDING else null), it == 1) }, emptyList(), stale = false),
                    DiscoverRow(DiscoverRowKey.POPULAR_MOVIES, (4..7).map { DiscoverEntry(ref(it), false) }, emptyList(), stale = false),
                ),
            ),
        )
        override suspend fun search(server: HttpUrl, query: String, page: Int) =
            ApiResult.Ok(DiscoverPage((0..3).map { DiscoverEntry(ref(it), false) }, 1, stale = false, fetchedAt = at, totalPages = 1))
        override suspend fun missing(server: HttpUrl, query: String, media: ExternalMedia?, offset: Int, limit: Int): ApiResult<Page<ExternalTitleRef>> {
            val hits = titles.indices.map { ref(it) }.filter { it.title.contains(query, ignoreCase = true) }
            return ApiResult.Ok(Page(hits, hits.size))
        }
        override suspend fun title(server: HttpUrl, media: ExternalMedia, tmdbId: Long) = title(tmdbId)?.let { ApiResult.Ok(it) } ?: ApiResult.NotFound
        override suspend fun seasons(server: HttpUrl, tmdbId: Long) = ApiResult.Ok(
            WishSeasons(
                WishSeasons.Source.SEERR,
                listOf(WishSeasonOffer(1, 8, WishSeasonOffer.State.AVAILABLE), WishSeasonOffer(2, 8, WishSeasonOffer.State.MISSING), WishSeasonOffer(3, 6, WishSeasonOffer.State.MISSING)),
            ),
        )
        override suspend fun wishes(server: HttpUrl, filter: WishFilter, offset: Int, limit: Int) =
            ApiResult.Ok(WishPage(wishes.drop(offset).take(limit), wishes.size, WishCounts(if (emptyWishes) 0 else 2, 1, 0, 1, if (emptyWishes) 0 else 1, if (emptyWishes) 0 else 1)))
        override suspend fun quota(server: HttpUrl) = ApiResult.Ok(quota)
        override suspend fun create(server: HttpUrl, request: WishCreate): ApiResult<Wish> = ApiResult.Failed(429, "quota_exceeded")
        override suspend fun withdraw(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
    }

    private val inbox = listOf(
        Notification(1, Notification.Kind.NEW_EPISODE, at.minusHours(2), item = BrowseShowcaseData.series[0]),
        Notification(2, Notification.Kind.WISH_AVAILABLE, at.minusDays(1), item = BrowseShowcaseData.films[2], wish = NotificationWish(3, ExternalMedia.MOVIE, BrowseShowcaseData.films[2].title, WishState.AVAILABLE)),
        Notification(3, Notification.Kind.WISH_DECLINED, at.minusDays(3), readAt = at, wish = NotificationWish(4, ExternalMedia.MOVIE, "Die Salzinsel", WishState.DECLINED)),
    )

    private val notificationsRepo = object : NotificationsRepository {
        override suspend fun page(server: HttpUrl, offset: Int, limit: Int) = ApiResult.Ok(NotificationPage(inbox.drop(offset).take(limit), inbox.size, 2, offset, limit))
        override suspend fun markRead(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
        override suspend fun markAllRead(server: HttpUrl) = ApiResult.Ok(Unit)
        override suspend fun announcements(server: HttpUrl) =
            ApiResult.Ok(listOf(ProfileAnnouncement(9, "Wartung am Sonntag 02:00 bis 03:00", "Der Server ist in dieser Zeit nicht erreichbar. Deine Downloads kannst du weiter abspielen.", at)))
        override suspend fun acknowledge(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
    }

    private val channels = listOf(
        ChannelSetting(Channel.EMAIL, true, setOf(EventType.NEW_CONTENT, EventType.WISH_AVAILABLE), true, email = EmailChannel("mara@example.test")),
        ChannelSetting(Channel.PUSH, true, setOf(EventType.NEW_CONTENT, EventType.WISH_AVAILABLE, EventType.ANNOUNCEMENTS), true, push = PushChannel(listOf(PushSubscriptionSummary(1, at, true, "Pixel 9 · Android 16")))),
        ChannelSetting(Channel.TELEGRAM, false, emptySet(), true, telegram = TelegramChannel(false, botUsername = "slouchcat_bot")),
    )

    private val channelsRepo = object : ChannelsRepository {
        override suspend fun channels(server: HttpUrl) = ApiResult.Ok(ChannelSettings(channels))
        override suspend fun update(server: HttpUrl, channel: Channel, update: ChannelSettingUpdate) = ApiResult.Ok(channels.single { it.channel == channel })
        override suspend fun test(server: HttpUrl, channel: Channel) = ApiResult.Ok(Unit)
        override suspend fun linkTelegram(server: HttpUrl) = ApiResult.Ok(channels.last())
        override suspend fun unlinkTelegram(server: HttpUrl) = ApiResult.Ok(Unit)
        override suspend fun vapidKey(server: HttpUrl) = ApiResult.Ok("B")
        override suspend fun subscribe(server: HttpUrl, request: PushSubscriptionRequest) = ApiResult.Ok(PushSubscriptionSummary(1, at, true))
        override suspend fun unsubscribe(server: HttpUrl, id: Long) = ApiResult.Ok(Unit)
    }

    /** This device's push, subscribed; the test sets other states. */
    val pushState = MutableStateFlow<DevicePushState>(DevicePushState.On)

    val devicePush = object : DevicePush {
        override val state = pushState
        override suspend fun enable() = Unit
        override suspend fun disable() = Unit
        override suspend fun refresh() = Unit
    }

    fun data(base: MediaData): MediaData = base.copy(wishes = wishesRepo, notifications = notificationsRepo, channels = channelsRepo)
}
