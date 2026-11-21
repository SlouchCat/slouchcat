package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.ProfilesApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.PlaybackCapabilities
import com.slouchcat.core.api.generated.models.PlaybackInfo
import com.slouchcat.core.api.generated.models.PlaybackPrefs
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.PlaybackSessionState
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.api.generated.models.SubtitleMode
import com.slouchcat.core.api.generated.models.Transcoding
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import okhttp3.Request
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.time.OffsetDateTime

/**
 * The player's operations against a real server (task android:contract, K27 stage 5): a start
 * with the phone app's capability profile plays the MKV of scripts/android-test-video.sh directly,
 * the same start without `mkv` as an HLS remux; the stream answers with its token, progress, the
 * session state and the stop; the profile's language preferences, `off` included (the value the
 * trimmer once turned into a boolean).
 */
class PlaybackContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val media = File(System.getProperty("slouchcat.contract.media").orEmpty().also { require(it.isNotEmpty()) { "run through `task android:contract`" } })

    /** The profile the phone app reports on the emulator (core/player CapabilitiesTest checks the rules make it). */
    private val phone: PlaybackCapabilities = SlouchcatApi.json.decodeFromString(
        PlaybackCapabilities.serializer(),
        File(System.getProperty("slouchcat.contract.fixtures").orEmpty(), "phone-capabilities.json").readText(),
    )

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    private fun appClient(): OkHttpClient {
        val setup = SlouchcatApi.create(::SetupApi, base, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = EMAIL, password = PASSWORD))
        val login = withinTheSignInLimit {
            SlouchcatApi.create(::AuthApi, base, SlouchcatHttp.client(identity))
                .appLogin(AppLoginRequest(EMAIL, PASSWORD, DEVICE_NAME, AppLoginRequest.Kind.PHONE))!!
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

    private fun absolute(path: String) = base.resolve(path.removePrefix("/"))!!

    private fun get(http: OkHttpClient, path: String, range: String? = null): Pair<Int, String> {
        val request = Request.Builder().url(absolute(path)).apply { if (range != null) header("Range", range) }.build()
        http.newCall(request).execute().use { response ->
            val body = if (range == null) response.body.string() else ""
            return response.code to body
        }
    }

    @Test
    fun aFilmDirectAndAsHlsWithProgressStateAndStopAndTheLanguages() {
        val http = appClient()
        DetailContractTest.Admin(base).ensureLibrary(media, LIBRARY, "Playback")
        val items = SlouchcatApi.create(::ItemsApi, base, http)
        val library = items.listVisibleLibraries()!!.single { it.name == LIBRARY }
        val film = items.listLibraryItems(library.id)!!.items.single { it.title == "Two Voices" }
        // The scan lists the file at once; its probe (codecs, tracks) follows as a job.
        val deadline = System.nanoTime() + PROBE_TIMEOUT_NS
        while (items.getItem(film.id)!!.mediaFiles.firstOrNull()?.videoCodec == null) {
            check(System.nanoTime() < deadline) { "the test film was not probed" }
            Thread.sleep(POLL_MS)
        }

        // The app's profile: the MKV plays directly, both audio tracks and the subtitles are listed.
        val direct = items.startPlayback(film.id, PlaybackRequest(capabilities = phone, positionMs = 0))!!
        assertEquals(PlaybackInfo.Mode.DIRECT, direct.mode)
        assertEquals(listOf("deu", "eng"), direct.tracks.audio.map { it.language })
        assertEquals(2, direct.tracks.subtitles.size)
        assertNotNull(direct.selection.audioTrack)
        assertEquals(3, direct.chapters.size)
        assertEquals(206, get(http, direct.streamUrl, "bytes=0-1023").first)

        val progress = items.reportProgress(
            film.id,
            ProgressEvent(
                positionMs = 12_000, durationMs = direct.mediaFile.durationMs ?: 60_000, playedMs = 12_000, seq = 1,
                state = ProgressEvent.State.PLAYING, clientTime = OffsetDateTime.now(),
                mediaFileId = direct.mediaFile.id, playbackSessionId = direct.playbackSessionId,
            ),
        )!!
        assertEquals(12_000L, progress.positionMs)
        assertEquals(PlaybackSessionState.State.PlaybackStateActive, items.getPlaybackSessionState(direct.playbackSessionId)!!.state)
        items.stopPlayback(direct.playbackSessionId)
        assertEquals(PlaybackSessionState.State.PlaybackStateStopped, items.getPlaybackSessionState(direct.playbackSessionId)!!.state)
        // The stream token died with the session.
        assertEquals(401, get(http, direct.streamUrl, "bytes=0-1023").first)
        assertEquals(12_000L, items.getItem(film.id)!!.progress?.positionMs)

        // A narrower profile (no MKV): HLS, a remux for the container, the master playlist answers.
        val narrow = phone.copy(containers = listOf("mp4"))
        val hls = items.startPlayback(film.id, PlaybackRequest(capabilities = narrow, audioTrack = direct.tracks.audio[1].index))!!
        assertEquals(PlaybackInfo.Mode.HLS, hls.mode)
        assertEquals(Transcoding.Mode.REMUX, hls.transcoding?.mode)
        assertTrue(hls.transcoding!!.reason, hls.transcoding!!.reason.contains("container"))
        assertEquals(direct.tracks.audio[1].index, hls.selection.audioTrack)
        assertEquals(12_000L, hls.resumePositionMs)
        val (status, master) = get(http, hls.streamUrl)
        assertEquals(200, status)
        assertTrue(master.startsWith("#EXTM3U"))
        items.stopPlayback(hls.playbackSessionId)

        // The languages of the profile, `off` as a word.
        val profiles = SlouchcatApi.create(::ProfilesApi, base, http)
        val before = profiles.getMyPlaybackPrefs()!!
        val wanted = PlaybackPrefs(linkedSetOf("en", "de"), SubtitleMode.OFF, linkedSetOf("de"))
        assertEquals(wanted, profiles.putMyPlaybackPrefs(wanted))
        val read = profiles.getMyPlaybackPrefs()!!
        assertEquals(listOf("en", "de"), read.audioLanguages.toList())
        assertEquals(SubtitleMode.OFF, read.subtitleMode)
        profiles.resetMySeriesTracks()
        profiles.putMyPlaybackPrefs(before)
    }

    private companion object {
        // The account of ContractTest: the server of the task has one owner.
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone · Android 16"
        const val LIBRARY = "Android playback media"
        const val PROBE_TIMEOUT_NS = 60_000_000_000L
        const val POLL_MS = 250L
    }
}
