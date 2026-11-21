package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.DiscoverApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.ItemKind
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.NextUp
import com.slouchcat.core.api.generated.models.SetupRequest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
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
import java.io.File

/**
 * The detail pages' operations against a real server (task android:contract, concept 27 stage 4):
 * `getItem` of a film and of a series with seasons and episodes, `getItemTrailers`, the watchlist,
 * the check mark and the user pick, and `getPerson` (404 for a person without visible titles).
 * The task writes the media tree of scripts/android-test-media.sh below the server's browsable
 * root and passes it as `slouchcat.contract.media`; the test adds it as a library through the
 * administration (a browser session, as an administrator does) and reads it as the app.
 */
class DetailContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val media: File = File(
        System.getProperty("slouchcat.contract.media").orEmpty().also {
            require(it.isNotEmpty()) { "run through `task android:contract`, which writes the test media" }
        },
    )

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    /** The app's session of the owner's primary profile (Bearer, no gate: the server is local). */
    private fun appClient(): OkHttpClient {
        val login = SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(identity))
            .appLogin(AppLoginRequest(EMAIL, PASSWORD, DEVICE_NAME, AppLoginRequest.Kind.PHONE))!!
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

    @Test
    fun aFilmASeriesWithEpisodesAndAPerson() {
        val setup = SlouchcatApi.create(::SetupApi, base, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
        Admin(base).ensureLibrary(media)

        val http = appClient()
        val items = SlouchcatApi.create(::ItemsApi, base, http)
        val library = items.listVisibleLibraries()!!.single { it.name == LIBRARY }
        val titles = items.listLibraryItems(library.id)!!.items
        val filmRef = titles.single { it.title == "Night Train" }
        val seriesRef = titles.single { it.title == "Quiet Ring" }

        // A film: its file, the fields the page reads, no trailer without TMDB.
        val film = items.getItem(filmRef.id)!!
        assertEquals(ItemKind.MOVIE, film.kind)
        assertEquals(2024, film.year)
        assertEquals(1, film.mediaFiles.size)
        assertTrue(film.canRecommend)
        assertTrue(items.getItemTrailers(film.id)!!.isEmpty())

        // A series: two seasons, the first episode to start, the episodes of a season.
        val series = items.getItem(seriesRef.id)!!
        assertEquals(ItemKind.SERIES, series.kind)
        assertEquals(listOf(2, 1), series.seasons!!.map { it.childCount })
        val next = series.nextUp!!
        assertEquals(NextUp.Reason.START, next.reason)
        val season = items.getItem(series.seasons!!.first().id)!!
        assertEquals(ItemKind.SEASON, season.kind)
        val episodes = season.episodes!!
        assertEquals(listOf(1, 2), episodes.map { it.episodeNumber })
        assertEquals(listOf("Arrival", "Cowardice"), episodes.map { it.title })
        assertEquals(next.episode.id, episodes.first().id)
        val episode = items.getItem(episodes.first().id)!!
        assertEquals(ItemKind.EPISODE, episode.kind)
        assertEquals(series.id, episode.series?.id ?: episode.seriesRef?.id)
        assertEquals(episodes[1].id, episode.nextEpisode?.id)

        // The check mark: an episode moves the series' next episode; a season clears its episodes.
        items.setItemWatched(episode.id, ItemWatchedRequest(true))
        val after = items.getItem(series.id)!!
        assertEquals(1, after.watchedCount)
        assertEquals(episodes[1].id, after.nextUp!!.episode.id)
        assertNotNull(items.getItem(episode.id)!!.progress?.watchedAt)
        items.setItemWatched(season.id, ItemWatchedRequest(false))
        assertEquals(0, items.getItem(series.id)!!.watchedCount)

        // Watchlist and user pick, and back.
        val discover = SlouchcatApi.create(::DiscoverApi, base, http)
        discover.addToWatchlist(film.id)
        assertTrue(items.getItem(film.id)!!.inWatchlist)
        discover.removeFromWatchlist(film.id)
        assertFalse(items.getItem(film.id)!!.inWatchlist)
        assertTrue(discover.recommendItem(film.id)!!.retentionDays >= 7)
        assertTrue(items.getItem(film.id)!!.recommendedByMe)
        discover.unrecommendItem(film.id)
        assertFalse(items.getItem(film.id)!!.recommendedByMe)

        // A person: without TMDB the titles have no cast, so the person is unknown here: 404, not 403.
        try {
            items.getPerson(987_654)
            fail("an unknown person answered")
        } catch (e: ClientException) {
            assertEquals(404, e.statusCode)
        }
        // Not visible is 404 as well (rules 2 and 12).
        try {
            items.getItem("01J0000000000000000000ZZZZ")
            fail("an unknown title answered")
        } catch (e: ClientException) {
            assertEquals(404, e.statusCode)
        }
    }

    /**
     * The administration as a browser uses it: a cookie session of the owner with the primary
     * profile. Only for preparing the server; the generated client of the app has no admin calls.
     */
    private class Admin(private val base: HttpUrl) {
        private val cookies = mutableListOf<Cookie>()
        private val http = OkHttpClient.Builder()
            .cookieJar(object : CookieJar {
                override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
                    this@Admin.cookies.removeAll { old -> cookies.any { it.name == old.name } }
                    this@Admin.cookies += cookies
                }

                override fun loadForRequest(url: HttpUrl) = cookies.toList()
            })
            .build()

        private fun call(method: String, path: String, body: String? = null): JsonElement? {
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

        private fun libraries(): JsonArray = call("GET", "admin/libraries")!!.jsonArray

        /** The library [LIBRARY] with the film and series folders of [media], scanned. */
        fun ensureLibrary(media: File) {
            call("POST", "auth/login", """{"email":"$EMAIL","password":"$PASSWORD"}""")
            val primary = call("GET", "auth/profiles")!!.jsonArray.map { it.jsonObject }.single { it["isPrimary"]!!.jsonPrimitive.boolean }
            call("POST", "auth/profiles/${primary["id"]!!.jsonPrimitive.long}/select")
            if (libraries().none { it.jsonObject["name"]?.jsonPrimitive?.content == LIBRARY }) {
                val movies = Json.encodeToString(File(media, "Movies").absolutePath)
                val tv = Json.encodeToString(File(media, "TV").absolutePath)
                call(
                    "POST",
                    "admin/libraries",
                    """{"name":"$LIBRARY","kind":"video","roots":[{"path":$movies,"content":"movie","providers":["tmdb"]},{"path":$tv,"content":"series","providers":["tmdb"]}]}""",
                )
            }
            val deadline = System.nanoTime() + SCAN_TIMEOUT_NS
            while (true) {
                val library = libraries().map { it.jsonObject }.single { it["name"]?.jsonPrimitive?.content == LIBRARY }
                val state = (library["scan"] as JsonObject)["state"]!!.jsonPrimitive.content
                check(state != "failed" && state != "aborted") { "the scan of the test library $state" }
                if (state == "done") return
                check(System.nanoTime() < deadline) { "the scan of the test library did not finish" }
                Thread.sleep(POLL_MS)
            }
        }
    }

    private companion object {
        // The account of ContractTest: the server of the task has one owner.
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone · Android 16"
        const val LIBRARY = "Android test media"
        const val SCAN_TIMEOUT_NS = 120_000_000_000L
        const val POLL_MS = 250L
    }
}
