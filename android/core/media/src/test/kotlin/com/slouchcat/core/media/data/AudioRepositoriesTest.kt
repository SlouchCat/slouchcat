package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.models.ItemDetail
import com.slouchcat.core.api.generated.models.ItemImages
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemRef
import com.slouchcat.core.api.generated.models.MediaFileInfo
import com.slouchcat.core.api.generated.models.Progress
import com.slouchcat.core.api.generated.models.RadioSort
import com.slouchcat.core.media.MediaData
import com.slouchcat.core.media.PlayTarget
import com.slouchcat.core.media.audioPlayTarget
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.math.BigDecimal

/** The audio and radio repositories against a fake server, and the big button of audio titles. */
class AudioRepositoriesTest {
    private val server = MockWebServer()
    private val http = SlouchcatHttp.client(ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0"))
    private val data = MediaData.api(ApiCaller(SessionEvents()), http)

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    private fun json(body: String, code: Int = 200) =
        server.enqueue(MockResponse(code = code, body = body, headers = okhttp3.Headers.headersOf("Content-Type", "application/json")))

    private val station = """{"id":7,"name":"Testradio Eins","languages":["de"],"genres":["pop"],"kinds":[],"moods":[],"hls":false,""" +
        """"showsTitles":true,"favorite":true,"recommended":false,"state":"ok","country":"CH"}"""

    @Test
    fun theEpisodesOfAPodcastInPages() = runTest {
        json("""{"episodes":[{"id":"01J00000000000000000000002","kind":"podcast_episode","title":"Folge 2","inWatchlist":false,"images":{}}],"total":51,"offset":50,"limit":50}""")
        val page = (data.audio.episodes(server.url("/"), "01J00000000000000000000001", offset = 50, limit = 50) as ApiResult.Ok).value
        assertEquals(51, page.total)
        assertEquals(listOf("Folge 2"), page.items.map { it.title })
        val url = server.takeRequest().url
        assertEquals("/api/v1/items/01J00000000000000000000001/episodes", url.encodedPath)
        assertEquals("50", url.queryParameter("offset"))
    }

    @Test
    fun aSeriesWithFewerThanTwoVisibleVolumesIsNotFound() = runTest {
        json("""{"type":"about:blank","title":"x","status":404,"code":"not_found"}""", code = 404)
        assertEquals(ApiResult.NotFound, data.audio.series(server.url("/"), 3, "s-1"))
        assertEquals("/api/v1/libraries/3/series/s-1", server.takeRequest().url.encodedPath)
    }

    @Test
    fun stationsAskWithTheirFilter() = runTest {
        json("""{"items":[$station],"total":1,"offset":0}""")
        val query = RadioQuery(text = "eins", country = "CH", favorites = true, sort = RadioSort.NAME)
        val page = (data.radio.stations(server.url("/"), query, offset = 0, limit = 500) as ApiResult.Ok).value
        assertEquals("Testradio Eins", page.items.single().name)
        val url = server.takeRequest().url
        assertEquals("/api/v1/radio/stations", url.encodedPath)
        assertEquals("eins", url.queryParameter("q"))
        assertEquals("CH", url.queryParameter("country"))
        assertEquals("true", url.queryParameter("favorites"))
        assertEquals("name", url.queryParameter("sort"))
        // The server's bound, not more.
        assertEquals("120", url.queryParameter("limit"))
    }

    @Test
    fun aFavouriteIsAddedAndRemoved() = runTest {
        server.enqueue(MockResponse(code = 204))
        server.enqueue(MockResponse(code = 204))
        assertTrue(data.radio.setFavorite(server.url("/"), 7, on = true) is ApiResult.Ok)
        assertTrue(data.radio.setFavorite(server.url("/"), 7, on = false) is ApiResult.Ok)
        val add = server.takeRequest()
        val remove = server.takeRequest()
        assertEquals("PUT /api/v1/radio/favorites/7", "${add.method} ${add.url.encodedPath}")
        assertEquals("DELETE /api/v1/radio/favorites/7", "${remove.method} ${remove.url.encodedPath}")
    }

    @Test
    fun whatPlaysNow() = runTest {
        json("""{"live":true,"entries":[{"at":1700000000000,"text":"Artist - Song","artist":"Artist","title":"Song"}]}""")
        val now = (data.radio.now(server.url("/"), 7) as ApiResult.Ok).value
        assertEquals("Song", now.propertyEntries.single().title)
        assertEquals("/api/v1/radio/stations/7/now", server.takeRequest().url.encodedPath)
    }

    private fun detail(kind: ItemKind, progress: Progress? = null, files: Boolean = true, episodes: List<ItemRef>? = null) = ItemDetail(
        id = "01J00000000000000000000001",
        kind = kind,
        title = "Silo",
        inWatchlist = false,
        images = ItemImages(),
        recommendedByMe = false,
        canRecommend = false,
        collections = emptyList(),
        genres = emptyList(),
        mediaFiles = if (files) listOf(MediaFileInfo(id = "f1", container = "mp3", sizeBytes = 1, available = true)) else emptyList(),
        progress = progress,
        episodes = episodes,
    )

    private fun progress(position: Long, completed: Boolean = false) =
        Progress(positionMs = position, durationMs = 100_000, percent = BigDecimal(position / 1000), completed = completed, updatedAt = java.time.OffsetDateTime.now())

    @Test
    fun theBigButtonOfAudio() {
        assertEquals(PlayTarget.Kind.Play, audioPlayTarget(detail(ItemKind.AUDIOBOOK))?.kind)
        assertEquals(PlayTarget.Kind.Resume, audioPlayTarget(detail(ItemKind.AUDIOBOOK, progress(40_000)))?.kind)
        assertEquals(PlayTarget.Kind.Again, audioPlayTarget(detail(ItemKind.PODCAST_EPISODE, progress(100_000, completed = true), files = false))?.kind)
        // An audiobook without a file cannot play; an episode streams from its feed.
        assertNull(audioPlayTarget(detail(ItemKind.AUDIOBOOK, files = false)))
        assertEquals(PlayTarget.Kind.Play, audioPlayTarget(detail(ItemKind.PODCAST_EPISODE, files = false))?.kind)
        val newest = ItemRef(id = "01J00000000000000000000009", kind = ItemKind.PODCAST_EPISODE, title = "Neu", inWatchlist = false, images = ItemImages())
        val podcast = audioPlayTarget(detail(ItemKind.PODCAST, episodes = listOf(newest)))
        assertEquals("01J00000000000000000000009", podcast?.id)
        assertEquals(PlayTarget.Kind.Episode, podcast?.kind)
        assertNull(audioPlayTarget(detail(ItemKind.PODCAST, episodes = emptyList())))
        assertNull(audioPlayTarget(detail(ItemKind.MOVIE)))
    }
}
