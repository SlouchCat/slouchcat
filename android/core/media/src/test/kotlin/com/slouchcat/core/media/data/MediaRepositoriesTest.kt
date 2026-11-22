package com.slouchcat.core.media.data

import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.models.HomeListKey
import com.slouchcat.core.api.generated.models.LibraryItemStatus
import com.slouchcat.core.api.generated.models.LibrarySort
import com.slouchcat.core.media.MediaData
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/** The repositories over the generated client against a fake server: what they ask and what they make of the answer. */
class MediaRepositoriesTest {
    private val server = MockWebServer()
    private val http = SlouchcatHttp.client(ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0"))
    private val data = MediaData.api(ApiCaller(SessionEvents()), http)

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    private fun json(body: String, code: Int = 200) = server.enqueue(MockResponse(code = code, body = body, headers = okhttp3.Headers.headersOf("Content-Type", "application/json")))

    private val oneTitle = """{"items":[{"id":"01J00000000000000000000001","kind":"movie","title":"Lumen","inWatchlist":false,"images":{}}],"total":7,"offset":0,"limit":1}"""

    @Test
    fun aRowOfTheStartPageIsOnePageOfTheWholeList() = runTest {
        json(oneTitle)
        val page = (data.home.homeRow(server.url("/"), HomeListKey.RECENT, offset = 0, limit = 1) as ApiResult.Ok).value
        assertEquals(listOf("Lumen"), page.items.map { it.title })
        assertEquals(7, page.total)
        val request = server.takeRequest()
        assertEquals("/api/v1/home/rows/recent", request.url.encodedPath)
        assertEquals("1", request.url.queryParameter("limit"))
    }

    @Test
    fun theLibraryAsksWithItsFilters() = runTest {
        json("""{"items":[],"total":0,"offset":0,"limit":50,"facets":{"letters":[],"genres":[],"ratings":[]}}""")
        val query = LibraryQuery(sort = LibrarySort.ADDED, descending = true, genre = "Drama", yearFrom = 2010, yearTo = 2019, status = listOf(LibraryItemStatus.UNWATCHED), maxRating = 12)
        val page = (data.library.items(server.url("/"), library = 3, query = query, offset = 50, limit = 50) as ApiResult.Ok).value
        assertEquals(0, page.total)
        val url = server.takeRequest().url
        assertEquals("/api/v1/libraries/3/items", url.encodedPath)
        assertEquals("added", url.queryParameter("sort"))
        assertEquals("desc", url.queryParameter("order"))
        assertEquals("Drama", url.queryParameter("genre"))
        assertEquals("2010", url.queryParameter("yearFrom"))
        assertEquals("2019", url.queryParameter("yearTo"))
        assertEquals("unwatched", url.queryParameter("status"))
        assertEquals("12", url.queryParameter("maxRating"))
        assertEquals("50", url.queryParameter("offset"))
    }

    @Test
    fun aTitleTheProfileDoesNotSeeIsNotFound() = runTest {
        json("""{"type":"about:blank","title":"x","status":404,"code":"not_found"}""", code = 404)
        assertSame(ApiResult.NotFound, data.detail.item(server.url("/"), "01J00000000000000000000009"))
    }

    @Test
    fun merkenAndBackAreTwoCalls() = runTest {
        server.enqueue(MockResponse(code = 204))
        server.enqueue(MockResponse(code = 204))
        assertTrue(data.items.setWatchlist(server.url("/"), "01J00000000000000000000001", on = true) is ApiResult.Ok)
        assertTrue(data.myLists.setWatchlist(server.url("/"), "01J00000000000000000000001", on = false) is ApiResult.Ok)
        val add = server.takeRequest()
        val remove = server.takeRequest()
        assertEquals("PUT" to "/api/v1/items/01J00000000000000000000001/watchlist", add.method to add.url.encodedPath)
        assertEquals("DELETE" to "/api/v1/items/01J00000000000000000000001/watchlist", remove.method to remove.url.encodedPath)
    }

    /** A copy with other repositories keeps the counts; clearing forgets them all (another profile). */
    @Test
    fun theCountsGoWithTheDataAndClearTogether() {
        val fake = data.copy(home = data.home)
        assertSame(data.watchlistCount, fake.watchlistCount)
        fake.watchlistCount.set(4)
        fake.collectionsCount.set(2)
        fake.watchlistCount.adjust(-5)
        assertSame(0, fake.watchlistCount.count.value)
        fake.clearCounts()
        assertNull(fake.watchlistCount.count.value)
        assertNull(fake.collectionsCount.count.value)
        // Unknown stays unknown.
        fake.watchlistCount.adjust(1)
        assertNull(fake.watchlistCount.count.value)
    }
}
