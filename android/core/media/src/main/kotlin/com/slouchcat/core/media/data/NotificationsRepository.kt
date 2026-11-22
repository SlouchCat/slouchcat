package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.generated.apis.AnnouncementsApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.NotifyApi
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSetting
import com.slouchcat.core.api.generated.models.ChannelSettingUpdate
import com.slouchcat.core.api.generated.models.ChannelSettings
import com.slouchcat.core.api.generated.models.NotificationPage
import com.slouchcat.core.api.generated.models.ProfileAnnouncement
import com.slouchcat.core.api.generated.models.PushSubscriptionRequest
import com.slouchcat.core.api.generated.models.PushSubscriptionSummary
import com.slouchcat.core.api.map
import okhttp3.Call
import okhttp3.HttpUrl

/**
 * The inbox of the active profile (`listNotifications`, `markNotificationRead`,
 * `markAllNotificationsRead`) and the announcements of the server's main user
 * (`listMyAnnouncements`, `ackAnnouncement`). The server decides which entries a profile gets and
 * names the titles it may see (rule 12); a title it no longer sees is absent from the entry.
 */
interface NotificationsRepository {
    suspend fun page(server: HttpUrl, offset: Int, limit: Int): ApiResult<NotificationPage>

    suspend fun markRead(server: HttpUrl, id: Long): ApiResult<Unit>

    suspend fun markAllRead(server: HttpUrl): ApiResult<Unit>

    /** The open announcements of the profile, newest first. */
    suspend fun announcements(server: HttpUrl): ApiResult<List<ProfileAnnouncement>>

    /** «Verstanden»; 404 when the main user withdrew it meanwhile. */
    suspend fun acknowledge(server: HttpUrl, id: Long): ApiResult<Unit>
}

class ApiNotificationsRepository(private val api: ApiCaller, private val http: Call.Factory) : NotificationsRepository {
    private fun discover(server: HttpUrl) = SlouchcatApi.create(::DiscoverApi, server, http)

    override suspend fun page(server: HttpUrl, offset: Int, limit: Int) =
        api.call("notifications") { discover(server).listNotifications(offset, limit) }

    override suspend fun markRead(server: HttpUrl, id: Long) = api.call("notifications") { discover(server).markNotificationRead(id) }

    override suspend fun markAllRead(server: HttpUrl) = api.call("notifications") { discover(server).markAllNotificationsRead() }

    override suspend fun announcements(server: HttpUrl) =
        api.call("announcements") { SlouchcatApi.create(::AnnouncementsApi, server, http).listMyAnnouncements() }

    override suspend fun acknowledge(server: HttpUrl, id: Long) =
        api.call("announcements") { SlouchcatApi.create(::AnnouncementsApi, server, http).ackAnnouncement(id) }
}

/**
 * The channels of the account (concept 27, 2.5): e-mail, push and Telegram with their events
 * (`getMyChannels`, `updateMyChannel`, `testMyChannel`, `createTelegramLink`,
 * `deleteTelegramLink`), and the push subscription of this device (`getPushPublicKey`,
 * `createPushSubscription`, `deletePushSubscription`). Changing a channel needs the primary profile
 * or no profile (403 `primary_profile_required`); the own subscription an app session manages
 * with every profile (concept 27, 7.3).
 */
interface ChannelsRepository {
    suspend fun channels(server: HttpUrl): ApiResult<ChannelSettings>

    suspend fun update(server: HttpUrl, channel: Channel, update: ChannelSettingUpdate): ApiResult<ChannelSetting>

    suspend fun test(server: HttpUrl, channel: Channel): ApiResult<Unit>

    /** A new link code (an older one stops working). */
    suspend fun linkTelegram(server: HttpUrl): ApiResult<ChannelSetting>

    suspend fun unlinkTelegram(server: HttpUrl): ApiResult<Unit>

    /** The server's VAPID public key (base64url); 404 `push_unavailable` when the server has no push. */
    suspend fun vapidKey(server: HttpUrl): ApiResult<String>

    suspend fun subscribe(server: HttpUrl, request: PushSubscriptionRequest): ApiResult<PushSubscriptionSummary>

    suspend fun unsubscribe(server: HttpUrl, id: Long): ApiResult<Unit>
}

class ApiChannelsRepository(private val api: ApiCaller, private val http: Call.Factory) : ChannelsRepository {
    private fun notify(server: HttpUrl) = SlouchcatApi.create(::NotifyApi, server, http)

    override suspend fun channels(server: HttpUrl) = api.call("channels") { notify(server).getMyChannels() }

    override suspend fun update(server: HttpUrl, channel: Channel, update: ChannelSettingUpdate) =
        api.call("channels") { notify(server).updateMyChannel(channel, update) }

    override suspend fun test(server: HttpUrl, channel: Channel) = api.call("channels") { notify(server).testMyChannel(channel) }

    override suspend fun linkTelegram(server: HttpUrl) = api.call("channels") { notify(server).createTelegramLink() }

    override suspend fun unlinkTelegram(server: HttpUrl) = api.call("channels") { notify(server).deleteTelegramLink() }

    override suspend fun vapidKey(server: HttpUrl) = api.call("push") { notify(server).getPushPublicKey() }.map { it.publicKey }

    override suspend fun subscribe(server: HttpUrl, request: PushSubscriptionRequest) =
        api.call("push") { notify(server).createPushSubscription(request) }

    override suspend fun unsubscribe(server: HttpUrl, id: Long) = api.call("push") { notify(server).deletePushSubscription(id) }
}
