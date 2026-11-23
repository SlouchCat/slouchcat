package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.models.Channel
import com.slouchcat.core.api.generated.models.ChannelSettingUpdate
import com.slouchcat.core.api.generated.models.EventType
import com.slouchcat.core.api.generated.models.ExternalMedia
import com.slouchcat.core.api.generated.models.PushSubscriptionRequest
import com.slouchcat.core.api.generated.models.PushSubscriptionRequestKeys
import com.slouchcat.core.api.generated.models.WishCreate
import com.slouchcat.core.media.MediaData
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.net.URI

/** The repositories of stage 8 (wishes, inbox, channels, push) against a fake server. */
class MessagesRepositoriesTest {
    private val server = MockWebServer()
    private val http = SlouchcatHttp.client(ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0"))
    private val data = MediaData.api(ApiCaller(SessionEvents()), http)

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    private fun json(body: String, code: Int = 200) = server.enqueue(MockResponse(code = code, body = body, headers = okhttp3.Headers.headersOf("Content-Type", "application/json")))

    @Test
    fun theRightsComeWithTheSession() = runTest {
        json("""{"account":{"id":1,"email":"a@example.test","role":"user","isOwner":false,"maxRating":18,"allLibraries":true,"libraryIds":[]},"wishes":{"enabled":true,"canRequest":false,"movies":true,"shows":true,"reason":"no_mapping"}}""")
        val rights = (data.wishes.rights(server.url("/")) as ApiResult.Ok).value!!
        assertTrue(rights.enabled)
        assertEquals(false, rights.canRequest)
        assertEquals("/api/v1/auth/session", server.takeRequest().url.encodedPath)
        // An older server without wishes: no rights.
        json("""{"account":{"id":1,"email":"a@example.test","role":"user","isOwner":false,"maxRating":18,"allLibraries":true,"libraryIds":[]}}""")
        assertNull((data.wishes.rights(server.url("/")) as ApiResult.Ok).value)
    }

    @Test
    fun theOwnWishesAskWithTheirFilterAndARequestCarriesItsSeasons() = runTest {
        json("""{"items":[],"total":0,"counts":{"open":0,"pending":0,"processing":0,"partial":0,"available":0,"closed":0}}""")
        data.wishes.wishes(server.url("/"), WishFilter.Closed, offset = 50, limit = 50)
        val list = server.takeRequest().url
        assertEquals("/api/v1/wishes", list.encodedPath)
        assertEquals("closed", list.queryParameter("state"))
        assertEquals("50", list.queryParameter("offset"))
        json(
            """{"id":4,"media":"show","tmdbId":106,"title":"A","state":"queued","origin":"slouchcat","role":"owner","seasons":[{"number":2,"state":"queued"}],"canWithdraw":true,"canRequestAgain":false,"createdAt":"2026-11-20T18:00:00Z","updatedAt":"2026-11-20T18:00:00Z","events":[]}""",
            code = 201,
        )
        val made = data.wishes.create(server.url("/"), WishCreate(ExternalMedia.SHOW, 106, seasons = listOf(2), source = WishCreate.Source.DETAIL))
        assertEquals(4L, (made as ApiResult.Ok).value.id)
        val post = server.takeRequest()
        assertEquals("POST", post.method)
        val body = post.body!!.utf8()
        assertTrue(body, body.contains("\"seasons\":[2]") && body.contains("\"source\":\"detail\""))
        assertEquals("Slouchcat", post.headers["X-Requested-With"])
    }

    @Test
    fun aRefusedRequestKeepsItsCode() = runTest {
        json("""{"type":"about:blank","title":"x","status":429,"code":"quota_exceeded","needed":1}""", code = 429)
        val refused = data.wishes.create(server.url("/"), WishCreate(ExternalMedia.MOVIE, 79))
        assertEquals("quota_exceeded", (refused as ApiResult.Failed).code)
    }

    @Test
    fun theInboxAndTheAnnouncements() = runTest {
        json("""{"notifications":[{"id":1,"kind":"wish_available","createdAt":"2026-11-20T18:00:00Z","wish":{"id":3,"media":"movie","title":"X","state":"available"}}],"total":1,"unread":1,"offset":0,"limit":50}""")
        val page = (data.notifications.page(server.url("/"), 0, 50) as ApiResult.Ok).value
        assertEquals(1, page.unread)
        assertEquals("/api/v1/notifications", server.takeRequest().url.encodedPath)
        server.enqueue(MockResponse(code = 204))
        assertTrue(data.notifications.markAllRead(server.url("/")) is ApiResult.Ok)
        assertEquals("/api/v1/notifications/read-all", server.takeRequest().url.encodedPath)
        json("""[{"id":9,"title":"T","body":"B","publishedAt":"2026-11-20T18:00:00Z"}]""")
        assertEquals(9L, (data.notifications.announcements(server.url("/")) as ApiResult.Ok).value.single().id)
        server.enqueue(MockResponse(code = 204))
        data.notifications.acknowledge(server.url("/"), 9)
        server.takeRequest()
        assertEquals("/api/v1/announcements/9/ack", server.takeRequest().url.encodedPath)
    }

    @Test
    fun channelsAndThePushSubscription() = runTest {
        json("""{"publicKey":"BKEY"}""")
        assertEquals("BKEY", (data.channels.vapidKey(server.url("/")) as ApiResult.Ok).value)
        assertEquals("/api/v1/push/vapid-key", server.takeRequest().url.encodedPath)
        json("""{"id":5,"createdAt":"2026-11-20T18:00:00Z","thisDevice":true,"deviceName":"Phone"}""", code = 201)
        val request = PushSubscriptionRequest(URI("https://push.example.test/v1/push/1.abc"), PushSubscriptionRequestKeys("p256", "auth"), "Phone")
        assertEquals(5L, (data.channels.subscribe(server.url("/"), request) as ApiResult.Ok).value.id)
        val body = server.takeRequest().body!!.utf8()
        assertTrue(body, body.contains("\"keys\":{\"p256dh\":\"p256\",\"auth\":\"auth\"}"))
        json("""{"type":"about:blank","title":"x","status":403,"code":"primary_profile_required"}""", code = 403)
        val refused = data.channels.update(server.url("/"), Channel.EMAIL, ChannelSettingUpdate(true, setOf(EventType.NEW_CONTENT)))
        assertEquals(ApiResult.NotAllowed("primary_profile_required"), refused)
        assertEquals("/api/v1/me/channels/email", server.takeRequest().url.encodedPath)
    }
}
