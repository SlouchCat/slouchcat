package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AnnouncementsApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.NotifyApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.apis.WishesApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSettingUpdate
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.PushSubscriptionRequest
import com.slouchcat.core.api.generated.models.PushSubscriptionRequestKeys
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.api.generated.models.WishState
import com.slouchcat.core.api.push.PushMessage
import com.slouchcat.core.api.push.PushRelay
import com.slouchcat.core.api.push.RelayResult
import com.slouchcat.core.api.push.WebPush
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import kotlinx.serialization.json.longOrNull
import okhttp3.Cookie
import okhttp3.CookieJar
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.net.URI

/**
 * Stage 8 against a real server (task android:contract): the wishes through the Seerr and TMDB
 * mocks of the web app's e2e suite (`listWishes`, `createWish`, `withdrawWish`,
 * `getWishDiscover`, `searchWishTitles`, `searchExternalTitles`, `getExternalTitle`,
 * `getWishSeasons`, `getWishQuota`), the inbox (`listNotifications`, `markAllNotificationsRead`,
 * `listMyAnnouncements`), the channels (`getMyChannels`, `updateMyChannel`, `testMyChannel`) and
 * push end to end without Google: `getPushPublicKey`, the local push relay (`relay/cmd/push`)
 * registers a token of `relay/cmd/fcmfake`, `createPushSubscription` with the app's own key,
 * `testMyChannel(push)`, the message arrives at the fake as `data.wp` and the app's code decrypts
 * `{title, body, url}`; `deletePushSubscription` cleans up. Everything it changes it puts back.
 */
class MessagesContractTest {
    private val base: HttpUrl = prop("slouchcat.contract.base").toHttpUrl()
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    private fun prop(name: String): String = System.getProperty(name).orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which sets $name" }
    }

    private fun appClient(): OkHttpClient {
        val setup = SlouchcatApi.create(::SetupApi, base, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
        val login = withinTheSignInLimit {
            SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(identity)).appLogin(AppLoginRequest(EMAIL, PASSWORD, DEVICE_NAME, AppLoginRequest.Kind.PHONE))!!
        }
        val credentials = object : SessionCredentials {
            override val server = base
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val http = SlouchcatHttp.client(identity, credentials)
        val auth = SlouchcatApi.create(::AuthApi, base, http)
        auth.selectProfile(auth.listMyProfiles()!!.single { it.isPrimary }.id)
        return http
    }

    private fun refused(status: Int, code: String? = null, block: () -> Unit) {
        try {
            block()
            fail("the server took it")
        } catch (e: ClientException) {
            assertEquals(status, e.statusCode)
            if (code != null) assertTrue(e.response.toString(), (e.response as? com.slouchcat.core.api.generated.infrastructure.ClientError<*>)?.body.toString().contains(code))
        }
    }

    /** The administration over a browser session (cookie): Seerr, TMDB key and mapping are no app operations. */
    private inner class Admin {
        private val cookies = mutableListOf<Cookie>()
        private val http = OkHttpClient.Builder().cookieJar(
            object : CookieJar {
                override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
                    this@Admin.cookies.removeAll { old -> cookies.any { it.name == old.name } }
                    this@Admin.cookies += cookies
                }

                override fun loadForRequest(url: HttpUrl) = cookies.toList()
            },
        ).build()

        fun call(method: String, path: String, body: String? = null): JsonElement? {
            val request = Request.Builder()
                .url(base.newBuilder().addPathSegments(SlouchcatApi.BASE_PATH).addPathSegments(path.trimStart('/')).build())
                .header("X-Requested-With", "Slouchcat")
                .method(method, if (method == "GET") null else (body ?: "").toRequestBody("application/json".toMediaType()))
                .build()
            http.newCall(request).execute().use { response ->
                val text = response.body.string()
                check(response.isSuccessful) { "$method $path: ${response.code} $text" }
                return text.takeIf { it.isNotBlank() }?.let(Json::parseToJsonElement)
            }
        }

        fun signIn() {
            withinTheSignInLimit { call("POST", "auth/login", """{"email":"$EMAIL","password":"$PASSWORD"}""") }
            val primary = call("GET", "auth/profiles")!!.jsonArray.map { it.jsonObject }.single { it["isPrimary"]!!.jsonPrimitive.boolean }
            call("POST", "auth/profiles/${primary["id"]!!.jsonPrimitive.long}/select")
        }

        /** TMDB key, Seerr connected and tested, wishes on, the owner linked to a Seerr user (e2e `openWishes`). */
        fun openWishes(): Long {
            call("PUT", "admin/settings", """{"keys":{"tmdb":"$TMDB_KEY"}}""")
            call("PUT", "admin/seerr", """{"url":"${prop("slouchcat.contract.seerr")}","apiKey":"$SEERR_KEY","enabled":true}""")
            call("POST", "admin/seerr/test", "{}")
            call("PUT", "admin/wishes/settings", """{"enabled":true}""")
            val account = call("GET", "auth/session")!!.jsonObject["account"]!!.jsonObject["id"]!!.jsonPrimitive.long
            call("POST", "admin/wishes/mappings/users/refresh", "{}")
            val deadline = System.nanoTime() + WAIT_NS
            while ((call("GET", "admin/wishes/mappings/users") as JsonArray).none { it.jsonObject["id"]!!.jsonPrimitive.long == SEERR_USER }) {
                check(System.nanoTime() < deadline) { "Seerr's users did not arrive" }
                Thread.sleep(POLL_MS)
            }
            call("PUT", "admin/wishes/mappings/accounts/$account", """{"seerrUserId":$SEERR_USER}""")
            return account
        }

        /** Seerr (the mock) declines its open requests for [tmdbId]; the sync of the server then closes the wish. */
        fun decline(tmdbId: Long) {
            val seerr = prop("slouchcat.contract.seerr").toHttpUrl()
            val plain = OkHttpClient()
            val list = plain.newCall(Request.Builder().url(seerr.newBuilder().addPathSegment("__requests").build()).build()).execute()
                .use { Json.parseToJsonElement(it.body.string()).jsonArray }
            for (made in list.map { it.jsonObject }.filter { it["mediaId"]?.jsonPrimitive?.longOrNull == tmdbId }) {
                val body = """{"request":{"id":${made["id"]!!.jsonPrimitive.long},"status":3}}"""
                val request = Request.Builder().url(seerr.newBuilder().addPathSegment("__control").build())
                    .post(body.toRequestBody("application/json".toMediaType())).build()
                plain.newCall(request).execute().use { check(it.isSuccessful) { "the Seerr mock answered ${it.code}" } }
            }
            // A sync already running answers 409; the caller asks again until the inbox has it.
            runCatching { call("POST", "admin/seerr/sync", "{}") }
        }

        fun closeWishes(account: Long) {
            runCatching { call("PUT", "admin/wishes/mappings/accounts/$account", """{"seerrUserId":null}""") }
            runCatching { call("PUT", "admin/wishes/settings", """{"enabled":false}""") }
            runCatching { call("PUT", "admin/seerr", """{"clear":true,"enabled":false}""") }
            runCatching { call("PUT", "admin/settings", """{"clear":["tmdb"]}""") }
        }
    }

    @Test
    fun wishesThroughSeerr() {
        val http = appClient()
        val admin = Admin().also { it.signIn() }
        val auth = SlouchcatApi.create(::AuthApi, base, http)
        val wishes = SlouchcatApi.create(::WishesApi, base, http)
        var account: Long? = null
        try {
            account = admin.openWishes()
            val rights = auth.getSession()!!.wishes!!
            assertTrue(rights.enabled)
            assertTrue(rights.canRequest)
            assertNotNull(wishes.getWishDiscover())
            assertNotNull(wishes.getWishQuota())
            // The search of the wishes and the search's «Noch nicht in der Bibliothek».
            assertTrue(wishes.searchWishTitles("Glass", 1)!!.propertyEntries.any { it.title.tmdbId == FILM })
            val missing = wishes.searchExternalTitles("Glass", null, 0, 20)!!
            assertTrue(missing.items.any { it.tmdbId == FILM && it.canRequest })
            val title = wishes.getExternalTitle(ExternalMedia.MOVIE, FILM)!!
            assertTrue(title.canRequest)
            assertTrue(wishes.getWishSeasons(SHOW)!!.seasons.isNotEmpty())
            // Request, see it with its state, withdraw it.
            val wish = wishes.createWish(WishCreate(ExternalMedia.MOVIE, FILM, source = WishCreate.Source.DETAIL))!!
            assertEquals(FILM, wish.tmdbId)
            assertTrue(wishes.listWishes(null, 0, 50)!!.items.any { it.id == wish.id })
            assertEquals(wish.id, wishes.getExternalTitle(ExternalMedia.MOVIE, FILM)!!.wish?.id)
            wishes.withdrawWish(wish.id)
            assertEquals(WishState.WITHDRAWN, wishes.listWishes(null, 0, 50)!!.items.single { it.id == wish.id }.state)
            refused(404) { wishes.withdrawWish(999_999) }

            // A second wish Seerr declines: the inbox of the profile gets it; read one, then all.
            val declined = wishes.createWish(WishCreate(ExternalMedia.MOVIE, SECOND_FILM, source = WishCreate.Source.DETAIL))!!
            admin.decline(SECOND_FILM)
            val discover = SlouchcatApi.create(::DiscoverApi, base, http)
            val deadline = System.nanoTime() + WAIT_NS
            while (discover.listNotifications(0, 50)!!.notifications.none { it.wish?.id == declined.id }) {
                check(System.nanoTime() < deadline) { "the declined wish did not reach the inbox" }
                runCatching { admin.call("POST", "admin/seerr/sync", "{}") }
                Thread.sleep(POLL_MS)
            }
            val page = discover.listNotifications(0, 50)!!
            assertTrue(page.unread >= 1)
            val entry = page.notifications.first { it.wish?.id == declined.id }
            discover.markNotificationRead(entry.id)
            assertEquals(page.unread - 1, discover.listNotifications(0, 50)!!.unread)
            discover.markAllNotificationsRead()
            assertEquals(0, discover.listNotifications(0, 50)!!.unread)
        } finally {
            account?.let { admin.closeWishes(it) }
            runCatching { SlouchcatApi.create(::AuthApi, base, http).logout() }
        }
        // Seerr gone: the server says the wishes are off.
        assertFalse(SlouchcatApi.create(::AuthApi, base, appClient()).getSession()!!.wishes?.enabled ?: false)
    }

    @Test
    fun inboxAnnouncementsAndChannels() {
        val http = appClient()
        val discover = SlouchcatApi.create(::DiscoverApi, base, http)
        val notify = SlouchcatApi.create(::NotifyApi, base, http)
        val page = discover.listNotifications(0, 50)!!
        assertTrue(page.unread <= page.total)
        // An announcement of the main user: listed for the profile, «Verstanden» takes it away.
        val admin = Admin().also { it.signIn() }
        val created = admin.call("POST", "admin/announcements", """{"title":"Contract maintenance","body":"An hour off.","publish":true}""")!!.jsonObject
        val id = created["id"]!!.jsonPrimitive.long
        val announcements = SlouchcatApi.create(::AnnouncementsApi, base, http)
        assertTrue(announcements.listMyAnnouncements()!!.any { it.id == id })
        announcements.ackAnnouncement(id)
        assertFalse(announcements.listMyAnnouncements()!!.any { it.id == id })
        runCatching { admin.call("DELETE", "admin/announcements/$id") }
        val channels = notify.getMyChannels()!!.channels
        assertEquals(listOf(Channel.EMAIL, Channel.PUSH, Channel.TELEGRAM), channels.map { it.channel })
        val email = channels.single { it.channel == Channel.EMAIL }
        val changed = notify.updateMyChannel(Channel.EMAIL, ChannelSettingUpdate(email.enabled, email.events + EventType.WISH_STATUS))!!
        assertTrue(EventType.WISH_STATUS in changed.events)
        notify.updateMyChannel(Channel.EMAIL, ChannelSettingUpdate(email.enabled, email.events))
    }

    @Test
    fun pushEndToEndThroughTheRelayWithoutGoogle() {
        val relayBase = prop("slouchcat.contract.relay").toHttpUrl()
        val fcm = prop("slouchcat.contract.fcmfake").toHttpUrl()
        val http = appClient()
        val notify = SlouchcatApi.create(::NotifyApi, base, http)
        val vapid = notify.getPushPublicKey()!!.publicKey
        val token = "contract-" + System.nanoTime()
        val endpoint = (PushRelay(relayBase, OkHttpClient()).register(token, vapid) as RelayResult.Registered).endpoint
        val keys = WebPush.generate()
        val subscription = notify.createPushSubscription(
            PushSubscriptionRequest(URI(endpoint), PushSubscriptionRequestKeys(WebPush.p256dh(keys), WebPush.auth(keys)), DEVICE_NAME),
        )!!
        assertTrue(subscription.thisDevice)
        val push = notify.getMyChannels()!!.channels.single { it.channel == Channel.PUSH }
        assertTrue(push.push!!.subscriptions.any { it.id == subscription.id && it.thisDevice })
        try {
            if (!push.enabled) notify.updateMyChannel(Channel.PUSH, ChannelSettingUpdate(true, push.events))
            val sent = System.nanoTime()
            notify.testMyChannel(Channel.PUSH)
            val wp = waitForMessage(fcm, token)
            val ms = (System.nanoTime() - sent) / 1_000_000
            val body = PushMessage.body(wp)!!
            assertEquals("the relay's record size", 2700, body.size)
            val message = PushMessage.parse(WebPush.decrypt(body, keys))!!
            assertTrue(message.title.isNotBlank())
            assertEquals("/", PushMessage.path(message.url, base))
            println("push end to end: server to fcmfake in $ms ms, title «${message.title}»")
        } finally {
            if (!push.enabled) notify.updateMyChannel(Channel.PUSH, ChannelSettingUpdate(false, push.events))
            notify.deletePushSubscription(subscription.id)
        }
        assertFalse(notify.getMyChannels()!!.channels.single { it.channel == Channel.PUSH }.push!!.subscriptions.any { it.id == subscription.id })
    }

    /** `data.wp` of the first message `fcmfake` took for [token]. */
    private fun waitForMessage(fcm: HttpUrl, token: String): String {
        val client = OkHttpClient()
        val deadline = System.nanoTime() + WAIT_NS
        while (true) {
            val url = fcm.newBuilder().addPathSegment("messages").addQueryParameter("token", token).build()
            val received = client.newCall(Request.Builder().url(url).build()).execute().use { Json.parseToJsonElement(it.body.string()) }
            val first = (received as? JsonArray)?.firstOrNull()?.jsonObject
            first?.get("data")?.jsonObject?.get("wp")?.jsonPrimitive?.content?.let { return it }
            check(System.nanoTime() < deadline) { "no message at fcmfake for the token" }
            Thread.sleep(POLL_MS)
        }
    }

    private companion object {
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone · Android 16"

        // The mocks of web/e2e (seerr-server.mjs, tmdb-server.mjs): their keys, a user that needs an
        // approval, a film and a series only the TMDB mock knows.
        const val SEERR_KEY = "e2e-seerr-key-4f2a9c71d3b8"
        const val TMDB_KEY = "e2e-tmdb-key-8c41d0a7f2b9"
        const val SEERR_USER = 3L
        const val FILM = 9001L
        const val SECOND_FILM = 9002L
        const val SHOW = 9100L
        const val WAIT_NS = 30_000_000_000L
        const val POLL_MS = 250L
    }
}
