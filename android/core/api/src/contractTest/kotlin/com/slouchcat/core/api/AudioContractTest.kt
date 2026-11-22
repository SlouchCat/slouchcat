package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.RadioApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.infrastructure.ServerException
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.SetupRequest
import kotlinx.serialization.json.Json
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
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.math.BigDecimal
import java.time.OffsetDateTime

/**
 * The operations of stage 7 against a real server (task android:contract): the audiobook of two files
 * of scripts/android-test-video.sh plays as one HLS timeline with a chapter per file, its progress
 * keeps the speed; the podcast folder lists its episodes in pages and takes the check mark «gehört»;
 * a series of one volume is not there; the radio of a server without a catalogue answers its lists,
 * favourites and preferences, and a listen of an unknown station is 404. The test removes its two
 * libraries and its marks again.
 */
class AudioContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val media = File(System.getProperty("slouchcat.contract.media").orEmpty().also { require(it.isNotEmpty()) { "run through `task android:contract`" } })

    private val phone: PlaybackCapabilities = SlouchcatApi.json.decodeFromString(
        PlaybackCapabilities.serializer(),
        File(System.getProperty("slouchcat.contract.fixtures").orEmpty(), "phone-capabilities.json").readText(),
    )

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

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

    /** The administration over a web session of the owner (the libraries of the test). */
    private inner class Admin {
        private val cookies = mutableListOf<Cookie>()
        private val http = OkHttpClient.Builder().cookieJar(object : CookieJar {
            override fun saveFromResponse(url: HttpUrl, cookies: List<Cookie>) {
                this@Admin.cookies.removeAll { old -> cookies.any { it.name == old.name } }
                this@Admin.cookies += cookies
            }

            override fun loadForRequest(url: HttpUrl) = cookies.toList()
        }).build()

        fun call(method: String, path: String, body: String? = null): JsonElement? {
            val request = Request.Builder()
                .url(base.newBuilder().addPathSegments(SlouchcatApi.BASE_PATH).addPathSegments(path).build())
                .header("X-Requested-With", "Slouchcat")
                .method(method, if (method == "GET" || method == "DELETE") null else (body ?: "").toRequestBody("application/json".toMediaType()))
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

        /** An audio library over [folder] of the media, scanned; its id. */
        fun library(name: String, kind: String, folder: String, content: String): Long {
            val path = Json.encodeToString(File(media, folder).absolutePath)
            val id = call("POST", "admin/libraries", """{"name":"$name","kind":"$kind","roots":[{"path":$path,"content":"$content","providers":["local"]}]}""")!!
                .jsonObject["id"]!!.jsonPrimitive.long
            val deadline = System.nanoTime() + TIMEOUT_NS
            while (true) {
                val library = call("GET", "admin/libraries")!!.jsonArray.map { it.jsonObject }.single { it["id"]!!.jsonPrimitive.long == id }
                val state = (library["scan"] as JsonObject)["state"]!!.jsonPrimitive.content
                check(state != "failed" && state != "aborted") { "the scan of $name $state" }
                if (state == "done") return id
                check(System.nanoTime() < deadline) { "the scan of $name did not finish" }
                Thread.sleep(POLL_MS)
            }
        }
    }

    private fun <T> status(block: () -> T): Int = try {
        block()
        200
    } catch (e: ClientException) {
        e.statusCode
    } catch (e: ServerException) {
        e.statusCode
    }

    @Test
    fun audiobookPodcastAndRadio() {
        val http = appClient()
        val admin = Admin().apply { signIn() }
        val books = admin.library(BOOKS, "audiobooks", "Playback/Audiobooks", "audiobook")
        val podcasts = admin.library(PODCASTS, "podcasts", "Playback/Podcasts", "podcast")
        val items = SlouchcatApi.create(::ItemsApi, base, http)
        try {
            // The audiobook of two files: one HLS timeline of both, a chapter per file, the speed kept.
            val visible = items.listVisibleLibraries()!!
            val book = items.listLibraryItems(visible.single { it.name == BOOKS }.id)!!.items.single { it.title == "The Quiet Harbour" }
            val deadline = System.nanoTime() + TIMEOUT_NS
            while ((items.getItem(book.id)!!.durationMs ?: 0) < 100_000) {
                check(System.nanoTime() < deadline) { "the audiobook was not probed" }
                Thread.sleep(POLL_MS)
            }
            val detail = items.getItem(book.id)!!
            assertEquals("Mira Lindqvist", detail.author)
            assertEquals(2, detail.chapters!!.size)
            val play = items.startPlayback(book.id, PlaybackRequest(capabilities = phone, positionMs = 0))!!
            assertEquals(PlaybackInfo.MediaKind.AUDIO, play.mediaKind)
            assertEquals(PlaybackInfo.Mode.HLS, play.mode)
            assertEquals(2, play.parts!!.size)
            assertTrue(play.chapters[1].startMs in 59_000L..61_000L)
            val progress = items.reportProgress(
                book.id,
                ProgressEvent(
                    positionMs = 70_000, durationMs = detail.durationMs!!, playedMs = 10_000, seq = 1, state = ProgressEvent.State.PAUSED,
                    clientTime = OffsetDateTime.now(), mediaFileId = play.mediaFile.id, playbackSessionId = play.playbackSessionId, playbackRate = BigDecimal("1.5"),
                ),
            )!!
            assertEquals(0, BigDecimal("1.5").compareTo(progress.playbackRate))
            items.stopPlayback(play.playbackSessionId)
            assertEquals(0, BigDecimal("1.5").compareTo(items.getItem(book.id)!!.progress!!.playbackRate))
            // A series of one volume is not there (404, rule 2).
            assertEquals(404, status { items.getAudioSeries(visible.single { it.name == BOOKS }.id, "no-series") })

            // The podcast folder: its episodes in pages, the check mark «gehört».
            val podcast = items.listLibraryItems(visible.single { it.name == PODCASTS }.id)!!.items.single { it.title == "Night Shift Stories" }
            val page = items.listPodcastEpisodes(podcast.id, 0, 1)!!
            assertEquals(2, page.total)
            assertEquals(1, page.episodes.size)
            val episode = page.episodes.single().id
            items.setItemWatched(episode, ItemWatchedRequest(true))
            assertTrue(items.getItem(episode)!!.progress?.watchedAt != null)
            items.setItemWatched(episode, ItemWatchedRequest(false))

            // The radio of a server without a catalogue: the lists answer, an unknown station is not there.
            val radio = SlouchcatApi.create(::RadioApi, base, http)
            assertTrue(status { radio.listRadioStations() } in setOf(200, 503))
            assertEquals(emptyList<Any>(), radio.listRadioFavorites())
            assertTrue(radio.getRadioPreferences()!!.defaultCountry.isNotEmpty())
            assertEquals(404, status { radio.startRadioListen(999_999) })
        } finally {
            // Clean up: the open sessions of the test and its libraries.
            runCatching { admin.call("DELETE", "admin/libraries/$books") }
            runCatching { admin.call("DELETE", "admin/libraries/$podcasts") }
        }
    }

    private companion object {
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone · Android 16"
        const val BOOKS = "Android audio books"
        const val PODCASTS = "Android audio podcasts"
        const val TIMEOUT_NS = 120_000_000_000L
        const val POLL_MS = 250L
    }
}
