package com.slouchcat.app.tv.media

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.media.Loadable
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.MediaItem
import com.slouchcat.core.media.PlayTarget
import com.slouchcat.core.media.ServerImages
import com.slouchcat.core.media.WatchedMark
import com.slouchcat.core.media.playTarget
import com.slouchcat.core.media.toLoadable
import com.slouchcat.core.media.watchedMark
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.Headers.Companion.headersOf
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * The TV app reads through the shared data layer of `:core:media` (concept 26, 5): the repository
 * of the detail page against a fake server, the card model and the rules of the check mark and the
 * big button, the same the phone app uses.
 */
class SharedMediaDataTest {
    private val server = MockWebServer()
    private val identity = ClientIdentity(ClientIdentity.Kind.TV, "0.1.0")
    private val data = MediaData.api(ApiCaller(SessionEvents()), SlouchcatHttp.client(identity))

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    @Test
    fun aDetailPageOfTheTvReadsTheSharedRepository() = runTest {
        server.enqueue(
            MockResponse(
                body = """{"id":"01J00000000000000000000001","kind":"movie","title":"Nachtzug","inWatchlist":true,"recommendedByMe":false,"canRecommend":true,"collections":[],"genres":[],
                    "images":{"poster":{"url":"/api/v1/items/01J00000000000000000000001/images/poster/w400?v=1&p=1","width":400,"height":600}},
                    "mediaFiles":[{"id":"01J0000000000000000000000F","container":"mkv","sizeBytes":1,"available":true}],
                    "progress":{"positionMs":600000,"durationMs":6000000,"percent":10,"completed":false,"updatedAt":"2026-11-20T20:00:00Z"}}""",
                headers = headersOf("Content-Type", "application/json"),
            ),
        )
        val result = data.detail.item(server.url("/"), "01J00000000000000000000001")
        assertEquals("/api/v1/items/01J00000000000000000000001", server.takeRequest().url.encodedPath)
        val detail = (result.toLoadable() as Loadable.Ready).value

        val card = MediaItem.from(detail, ServerImages(server.url("/")))
        assertEquals("Nachtzug", card.title)
        assertTrue(card.poster.pick(400)!!.startsWith(server.url("/api/v1/items/").toString()))
        assertEquals(0.1f, card.progress!!, 0.001f)
        assertEquals(90, card.remainingMinutes)
        assertEquals(WatchedMark.None, detail.watchedMark)
        assertEquals(PlayTarget.Kind.Resume, playTarget(detail)!!.kind)
    }

    @Test
    fun aTitleTheProfileDoesNotSeeIsNotFoundOnTheTvToo() = runTest {
        server.enqueue(MockResponse(code = 404, body = """{"type":"about:blank","title":"x","status":404,"code":"not_found"}"""))
        assertEquals(ApiResult.NotFound, data.detail.item(server.url("/"), "01J00000000000000000000009"))
    }
}
