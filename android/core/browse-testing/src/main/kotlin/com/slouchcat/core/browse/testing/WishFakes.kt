package com.slouchcat.core.browse.testing

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
import com.slouchcat.core.api.generated.models.ItemRef
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
import com.slouchcat.core.media.data.OPEN_WISH_STATES
import com.slouchcat.core.media.Page
import com.slouchcat.core.media.data.ChannelsRepository
import com.slouchcat.core.media.data.NotificationsRepository
import com.slouchcat.core.media.data.WishFilter
import com.slouchcat.core.media.data.WishesRepository
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.HttpUrl
import java.time.OffsetDateTime

/* Test doubles of stage 8 (wishes, inbox, channels, push of the device) for both apps. */

val AT: OffsetDateTime = OffsetDateTime.parse("2026-11-20T18:00:00Z")

fun externalImages(media: ExternalMedia, tmdbId: Long) = ExternalTitleImages(
    posterSmall = image("titles/${media.value}/$tmdbId/images/poster/w200", 200),
    poster = image("titles/${media.value}/$tmdbId/images/poster/w400", 400),
    backdrop = image("titles/${media.value}/$tmdbId/images/backdrop/w1280", 1280),
)

fun externalRef(tmdbId: Long, title: String = "Title $tmdbId", media: ExternalMedia = ExternalMedia.MOVIE, year: Int? = 2024, wish: WishState? = null, canRequest: Boolean = true) =
    ExternalTitleRef(media, tmdbId, title, externalImages(media, tmdbId), hasTrailer = false, canRequest = canRequest, year = year, wish = wish?.let { WishMark(tmdbId * 10, it) })

fun rights(enabled: Boolean = true, canRequest: Boolean = true, discover: Boolean = true) =
    WishRights(enabled, canRequest, movies = true, shows = true, discover = discover, reason = if (canRequest) null else WishRights.Reason.NO_MAPPING)

fun quota(used: Int = 1, limit: Int = 5) = WishQuota(
    unlimited = false,
    movie = WishQuotaPart(limit, 7, used, 0, limit - used),
    tv = WishQuotaPart(limit, 7, used, 0, limit - used),
)

fun externalTitle(tmdbId: Long, title: String = "Title $tmdbId", media: ExternalMedia = ExternalMedia.MOVIE, wish: WishState? = null, canRequest: Boolean = wish == null) = ExternalTitle(
    media = media,
    tmdbId = tmdbId,
    title = title,
    images = externalImages(media, tmdbId),
    hasTrailer = false,
    canRequest = canRequest,
    genres = listOf("Drama"),
    seasons = emptyList(),
    trailers = emptyList(),
    request = WishRequestCard(canRequest, autoApprove = false, quota = quota()),
    year = 2024,
    minAge = 16,
    wish = wish?.let { WishMark(tmdbId * 10, it) },
    runtimeMinutes = if (media == ExternalMedia.MOVIE) 113 else null,
)

fun wish(id: Long, title: String = "Wish $id", state: WishState = WishState.PENDING, media: ExternalMedia = ExternalMedia.MOVIE, item: ItemRef? = null, seasons: List<Int> = emptyList(), joined: Boolean = false) = Wish(
    id = id,
    media = media,
    tmdbId = id + 1000,
    title = title,
    state = state,
    origin = Wish.Origin.SLOUCHCAT,
    role = if (joined) Wish.Role.JOINED else Wish.Role.OWNER,
    seasons = seasons.map { WishSeasonState(it, state) },
    canWithdraw = state in OPEN_WISH_STATES && state != WishState.PROCESSING,
    canRequestAgain = false,
    createdAt = AT,
    updatedAt = AT,
    events = emptyList(),
    year = 2024,
    external = externalRef(id + 1000, title, media),
    item = item,
)

/** Seerr and TMDB in memory: rights, rows, search, titles, seasons, wishes and the limit. */
class FakeWishes(
    var rightsAnswer: ApiResult<WishRights?> = ApiResult.Ok(rights()),
    var rows: MutableList<DiscoverRow> = mutableListOf(
        DiscoverRow(DiscoverRowKey.TRENDING, listOf(DiscoverEntry(externalRef(1, "Trend One"), false), DiscoverEntry(externalRef(2, "Trend Two", ExternalMedia.SHOW), true)), emptyList(), stale = false),
        DiscoverRow(DiscoverRowKey.MOVIE_GENRES, emptyList(), emptyList(), stale = false),
    ),
    var wishList: MutableList<Wish> = mutableListOf(),
    var titles: MutableMap<Long, ExternalTitle> = mutableMapOf(),
    var seasonOffers: List<WishSeasonOffer> = listOf(
        WishSeasonOffer(1, 8, WishSeasonOffer.State.AVAILABLE),
        WishSeasonOffer(2, 8, WishSeasonOffer.State.MISSING),
        WishSeasonOffer(3, 6, WishSeasonOffer.State.REQUESTED),
    ),
) : WishesRepository {
    /** When set, every answer is this failure. */
    var failure: ApiResult<Nothing>? = null

    /** When set, only creating and withdrawing fail. */
    var changeFailure: ApiResult<Nothing>? = null
    var discoverFailure: ApiResult<Nothing>? = null
    var searchHits: List<ExternalTitleRef> = listOf(externalRef(7, "Found Seven"), externalRef(8, "Found Eight", ExternalMedia.SHOW))
    val calls = mutableListOf<String>()
    val created = mutableListOf<WishCreate>()

    override suspend fun rights(server: HttpUrl): ApiResult<WishRights?> {
        calls += "rights"
        return failure ?: rightsAnswer
    }

    override suspend fun discover(server: HttpUrl): ApiResult<WishDiscover> {
        calls += "discover"
        (failure ?: discoverFailure)?.let { return it }
        return ApiResult.Ok(WishDiscover(rows))
    }

    override suspend fun search(server: HttpUrl, query: String, page: Int): ApiResult<DiscoverPage> {
        calls += "search:$query:$page"
        failure?.let { return it }
        return ApiResult.Ok(DiscoverPage(searchHits.map { DiscoverEntry(it, false) }, page, stale = false, fetchedAt = AT, totalPages = 1))
    }

    override suspend fun missing(server: HttpUrl, query: String, media: ExternalMedia?, offset: Int, limit: Int): ApiResult<Page<ExternalTitleRef>> {
        calls += "missing:$query"
        failure?.let { return it }
        val hits = searchHits.filter { it.title.contains(query, ignoreCase = true) }
        return ApiResult.Ok(Page(hits.drop(offset).take(limit), hits.size))
    }

    override suspend fun title(server: HttpUrl, media: ExternalMedia, tmdbId: Long): ApiResult<ExternalTitle> {
        calls += "title:${media.value}:$tmdbId"
        failure?.let { return it }
        val open = wishList.firstOrNull { it.tmdbId == tmdbId && it.state in OPEN_WISH_STATES }
        val title = titles[tmdbId] ?: return ApiResult.NotFound
        return ApiResult.Ok(title.copy(wish = open?.let { WishMark(it.id, it.state) }, canRequest = open == null && title.canRequest))
    }

    override suspend fun seasons(server: HttpUrl, tmdbId: Long): ApiResult<WishSeasons> {
        calls += "seasons:$tmdbId"
        failure?.let { return it }
        return ApiResult.Ok(WishSeasons(WishSeasons.Source.SEERR, seasonOffers))
    }

    override suspend fun wishes(server: HttpUrl, filter: WishFilter, offset: Int, limit: Int): ApiResult<WishPage> {
        calls += "wishes:$filter:$offset"
        failure?.let { return it }
        val shown = wishList.filter {
            when (filter) {
                WishFilter.All -> true
                WishFilter.Open -> it.state in OPEN_WISH_STATES
                WishFilter.Available -> it.state == WishState.AVAILABLE
                WishFilter.Closed -> it.state !in OPEN_WISH_STATES && it.state != WishState.AVAILABLE
            }
        }
        return ApiResult.Ok(WishPage(shown.drop(offset).take(limit), shown.size, counts()))
    }

    fun counts(): WishCounts {
        val open = wishList.count { it.state in OPEN_WISH_STATES }
        val available = wishList.count { it.state == WishState.AVAILABLE }
        return WishCounts(open, wishList.count { it.state == WishState.PENDING }, wishList.count { it.state == WishState.PROCESSING }, wishList.count { it.state == WishState.PARTIAL }, available, wishList.size - open - available)
    }

    override suspend fun quota(server: HttpUrl): ApiResult<WishQuota> = failure ?: ApiResult.Ok(quota())

    override suspend fun create(server: HttpUrl, request: WishCreate): ApiResult<Wish> {
        calls += "create:${request.tmdbId}"
        (changeFailure ?: failure)?.let { return it }
        created += request
        val id = (wishList.maxOfOrNull { it.id } ?: 0) + 1
        val title = titles[request.tmdbId]?.title ?: "Title ${request.tmdbId}"
        val made = wish(id, title, WishState.PENDING, request.media, seasons = request.seasons.orEmpty()).copy(tmdbId = request.tmdbId)
        wishList.add(0, made)
        return ApiResult.Ok(made)
    }

    override suspend fun withdraw(server: HttpUrl, id: Long): ApiResult<Unit> {
        calls += "withdraw:$id"
        (changeFailure ?: failure)?.let { return it }
        val index = wishList.indexOfFirst { it.id == id }
        if (index < 0) return ApiResult.NotFound
        wishList[index] = wishList[index].copy(state = WishState.WITHDRAWN, canWithdraw = false)
        return ApiResult.Ok(Unit)
    }
}

fun notification(id: Long, kind: Notification.Kind = Notification.Kind.NEW_EPISODE, read: Boolean = false, item: ItemRef? = itemRef(id.toInt(), "Episode $id"), wishTitle: String? = null) = Notification(
    id = id,
    kind = kind,
    createdAt = AT.minusHours(id),
    readAt = if (read) AT else null,
    item = item,
    wish = wishTitle?.let { NotificationWish(id, ExternalMedia.MOVIE, it, WishState.AVAILABLE) },
)

fun announcement(id: Long, title: String = "Maintenance $id", body: String = "The server is down for an hour.") = ProfileAnnouncement(id, title, body, AT)

/** The inbox in memory. */
class FakeNotifications(
    var entries: MutableList<Notification> = mutableListOf(),
    var open: MutableList<ProfileAnnouncement> = mutableListOf(),
) : NotificationsRepository {
    var failure: ApiResult<Nothing>? = null
    var changeFailure: ApiResult<Nothing>? = null
    val calls = mutableListOf<String>()

    override suspend fun page(server: HttpUrl, offset: Int, limit: Int): ApiResult<NotificationPage> {
        calls += "page:$offset"
        failure?.let { return it }
        return ApiResult.Ok(NotificationPage(entries.drop(offset).take(limit), entries.size, entries.count { it.readAt == null }, offset, limit))
    }

    override suspend fun markRead(server: HttpUrl, id: Long): ApiResult<Unit> {
        calls += "read:$id"
        (changeFailure ?: failure)?.let { return it }
        entries.replaceAll { if (it.id == id) it.copy(readAt = AT) else it }
        return ApiResult.Ok(Unit)
    }

    override suspend fun markAllRead(server: HttpUrl): ApiResult<Unit> {
        calls += "readAll"
        (changeFailure ?: failure)?.let { return it }
        entries.replaceAll { it.copy(readAt = it.readAt ?: AT) }
        return ApiResult.Ok(Unit)
    }

    override suspend fun announcements(server: HttpUrl): ApiResult<List<ProfileAnnouncement>> {
        calls += "announcements"
        failure?.let { return it }
        return ApiResult.Ok(open.toList())
    }

    override suspend fun acknowledge(server: HttpUrl, id: Long): ApiResult<Unit> {
        calls += "ack:$id"
        (changeFailure ?: failure)?.let { return it }
        return if (open.removeAll { it.id == id }) ApiResult.Ok(Unit) else ApiResult.NotFound
    }
}

fun channelSettings(pushEnabled: Boolean = true, subscriptions: List<PushSubscriptionSummary> = emptyList(), telegramLinked: Boolean = false) = listOf(
    ChannelSetting(Channel.EMAIL, enabled = true, events = setOf(EventType.NEW_CONTENT, EventType.WISH_AVAILABLE), available = true, email = EmailChannel(EMAIL)),
    ChannelSetting(Channel.PUSH, enabled = pushEnabled, events = setOf(EventType.NEW_CONTENT, EventType.WISH_AVAILABLE, EventType.ANNOUNCEMENTS), available = true, push = PushChannel(subscriptions)),
    ChannelSetting(Channel.TELEGRAM, enabled = false, events = emptySet(), available = true, telegram = TelegramChannel(telegramLinked, botUsername = "slouchcat_bot")),
)

/** The channels of the account and the push subscriptions of its devices in memory. */
class FakeChannels(var settings: MutableList<ChannelSetting> = channelSettings().toMutableList()) : ChannelsRepository {
    var failure: ApiResult<Nothing>? = null

    /** When set, updating, testing and linking fail with it. */
    var changeFailure: ApiResult<Nothing>? = null
    var vapid: ApiResult<String> = ApiResult.Ok("BPUBLICVAPIDKEY")
    val calls = mutableListOf<String>()
    val subscriptions = mutableListOf<PushSubscriptionRequest>()
    private var nextId = 1L

    override suspend fun channels(server: HttpUrl): ApiResult<ChannelSettings> {
        calls += "channels"
        failure?.let { return it }
        return ApiResult.Ok(ChannelSettings(settings.toList()))
    }

    override suspend fun update(server: HttpUrl, channel: Channel, update: ChannelSettingUpdate): ApiResult<ChannelSetting> {
        calls += "update:${channel.value}:${update.enabled}"
        (changeFailure ?: failure)?.let { return it }
        val index = settings.indexOfFirst { it.channel == channel }
        settings[index] = settings[index].copy(enabled = update.enabled, events = update.events)
        return ApiResult.Ok(settings[index])
    }

    override suspend fun test(server: HttpUrl, channel: Channel): ApiResult<Unit> {
        calls += "test:${channel.value}"
        return changeFailure ?: failure ?: ApiResult.Ok(Unit)
    }

    override suspend fun linkTelegram(server: HttpUrl): ApiResult<ChannelSetting> {
        calls += "link"
        (changeFailure ?: failure)?.let { return it }
        val index = settings.indexOfFirst { it.channel == Channel.TELEGRAM }
        settings[index] = settings[index].copy(telegram = TelegramChannel(false, linkCode = "ABC123", linkExpiresAt = AT.plusMinutes(10), botUsername = "slouchcat_bot"))
        return ApiResult.Ok(settings[index])
    }

    override suspend fun unlinkTelegram(server: HttpUrl): ApiResult<Unit> {
        calls += "unlink"
        (changeFailure ?: failure)?.let { return it }
        val index = settings.indexOfFirst { it.channel == Channel.TELEGRAM }
        settings[index] = settings[index].copy(telegram = TelegramChannel(false, botUsername = "slouchcat_bot"))
        return ApiResult.Ok(Unit)
    }

    override suspend fun vapidKey(server: HttpUrl): ApiResult<String> {
        calls += "vapid"
        return failure ?: vapid
    }

    override suspend fun subscribe(server: HttpUrl, request: PushSubscriptionRequest): ApiResult<PushSubscriptionSummary> {
        calls += "subscribe"
        (changeFailure ?: failure)?.let { return it }
        subscriptions += request
        val summary = PushSubscriptionSummary(nextId++, AT, thisDevice = true, deviceName = request.deviceName)
        replacePush(listOf(summary))
        return ApiResult.Ok(summary)
    }

    override suspend fun unsubscribe(server: HttpUrl, id: Long): ApiResult<Unit> {
        calls += "unsubscribe:$id"
        failure?.let { return it }
        replacePush(emptyList())
        return ApiResult.Ok(Unit)
    }

    /** The subscriptions the push channel lists (the server's view of the devices). */
    fun replacePush(list: List<PushSubscriptionSummary>) {
        val index = settings.indexOfFirst { it.channel == Channel.PUSH }
        if (index >= 0) settings[index] = settings[index].copy(push = PushChannel(list))
    }
}

/** This device's push in memory: [enable] and [disable] only change the state. */
class FakeDevicePush(initial: DevicePushState = DevicePushState.Off) : DevicePush {
    override val state = MutableStateFlow(initial)
    val calls = mutableListOf<String>()

    override suspend fun enable() {
        calls += "enable"
        state.value = DevicePushState.On
    }

    override suspend fun disable() {
        calls += "disable"
        state.value = DevicePushState.Off
    }

    override suspend fun refresh() {
        calls += "refresh"
    }
}
