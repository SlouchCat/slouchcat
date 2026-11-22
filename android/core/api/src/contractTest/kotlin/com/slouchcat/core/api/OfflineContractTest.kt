package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.OfflineApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.DownloadQuality
import com.slouchcat.core.api.generated.models.DownloadRequest
import com.slouchcat.core.api.generated.models.DownloadRules
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.OfflineEvent
import com.slouchcat.core.api.generated.models.OfflineEventBatch
import com.slouchcat.core.api.generated.models.OfflineSyncRequest
import com.slouchcat.core.api.generated.models.OfflineSyncRequestHeldInner
import com.slouchcat.core.api.generated.models.RevocationCheck
import com.slouchcat.core.api.generated.models.SetupRequest
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import okhttp3.Request
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.time.Duration
import java.time.OffsetDateTime

/**
 * The offline operations of stage 6 against a real server (task android:contract, concept 27 12
 * «Vertrag»): options with three qualities, a download on the device of the app session, the
 * preparation seen through the sync, manifest and file with Range and the download token, the
 * rule, progress events, the public revocation list with the device id and the delete.
 */
class OfflineContractTest {
    private val base: HttpUrl = System.getProperty("slouchcat.contract.base").orEmpty().also {
        require(it.isNotEmpty()) { "run through `task android:contract`, which starts the server" }
    }.toHttpUrl()

    private val media = File(System.getProperty("slouchcat.contract.media").orEmpty().also { require(it.isNotEmpty()) { "run through `task android:contract`" } })

    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")

    private class Signed(val http: OkHttpClient, val deviceId: String, val profileId: Long)

    private fun signIn(): Signed {
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
        // A hang shows with its place and time instead of a read timeout somewhere later (code review m-10).
        val http = SlouchcatHttp.client(identity, credentials, builder = OkHttpClient.Builder().callTimeout(Duration.ofSeconds(CALL_TIMEOUT_S)))
        val auth = SlouchcatApi.create(::AuthApi, base, http)
        val profile = auth.listMyProfiles()!!.single { it.isPrimary }.id
        auth.selectProfile(profile)
        return Signed(http, login.deviceId, profile)
    }

    @Test
    fun aFilmIsOrderedPreparedFetchedSyncedRevokedAndDeleted() {
        val signed = signIn()
        val admin = DetailContractTest.Admin(base)
        admin.ensureLibrary(media, LIBRARY, "Playback")
        var created: String? = null
        try {
            created = orderPrepareFetchAndRevoke(signed) { created = it }
        } finally {
            // The server is left as it was: the download deleted, no preparation running, the app session ended.
            val offline = SlouchcatApi.create(::OfflineApi, base, signed.http)
            created?.let { id -> runCatching { offline.deleteDownload(id) } }
            val quiet = System.nanoTime() + TIMEOUT_NS
            while (admin.jobs(PREPARE_JOB, "running") + admin.jobs(PREPARE_JOB, "queued") > 0 && System.nanoTime() < quiet) Thread.sleep(POLL_MS)
            runCatching { SlouchcatApi.create(::AuthApi, base, signed.http).logout() }
        }
    }

    private fun orderPrepareFetchAndRevoke(signed: Signed, ordered: (String) -> Unit): String {
        val http = signed.http
        val items = SlouchcatApi.create(::ItemsApi, base, http)
        val library = items.listVisibleLibraries()!!.single { it.name == LIBRARY }
        val film = items.listLibraryItems(library.id)!!.items.single { it.title == "Two Voices" }
        val probed = System.nanoTime() + TIMEOUT_NS
        while (items.getItem(film.id)!!.mediaFiles.firstOrNull()?.videoCodec == null) {
            check(System.nanoTime() < probed) { "the test film was not probed" }
            Thread.sleep(POLL_MS)
        }
        val offline = SlouchcatApi.create(::OfflineApi, base, http)

        val options = offline.getDownloadOptions(film.id)!!
        assertEquals(setOf(DownloadQuality.HIGH, DownloadQuality.MEDIUM, DownloadQuality.LOW), options.qualities.map { it.quality }.toSet())
        assertTrue(options.qualities.all { it.estimatedBytes > 0 })

        val created = offline.createDownloads(film.id, DownloadRequest(quality = DownloadQuality.LOW))!!.downloads.single()
        ordered(created.id)
        assertTrue(signed.profileId in created.allowedProfileIds)

        // The sync reports the server's state until the copy is ready.
        val ready = System.nanoTime() + TIMEOUT_NS
        var state = created
        while (state.state != DownloadStateValue.READY) {
            check(System.nanoTime() < ready) { "the download was not prepared: ${state.state} ${state.error}" }
            Thread.sleep(POLL_MS)
            val sync = offline.syncOffline(OfflineSyncRequest(held = listOf(OfflineSyncRequestHeldInner(created.id, false)), network = OfflineSyncRequest.Network.WIFI))!!
            state = sync.downloads.single { it.id == created.id }
            assertTrue(sync.profiles.any { it.id == signed.profileId })
        }

        val manifest = offline.getDownloadManifest(created.id)!!
        assertTrue(manifest.segments.isNotEmpty())
        assertEquals(manifest.sizeBytes, manifest.segments.last().offset + manifest.segments.last().length)
        val file = base.resolve(manifest.fileUrl.removePrefix("/"))!!
        // The rest of the file from a byte on, with the strong ETag (resume, 9.2 point 3).
        val etag = http.newCall(Request.Builder().url(file).header("Range", "bytes=0-99").build()).execute().use { response ->
            assertEquals(206, response.code)
            assertEquals(100, response.body.bytes().size)
            response.header("ETag")!!
        }
        http.newCall(Request.Builder().url(file).header("Range", "bytes=100-").header("If-Range", etag).build()).execute().use { response ->
            assertEquals(206, response.code)
            assertEquals(manifest.sizeBytes - 100, response.body.bytes().size.toLong())
        }

        // The rule of auto-downloads for this profile and device.
        val rule = DownloadRules(enabled = true, nextEpisodes = true, newPodcastEpisodes = false, wifiOnly = true, maxCount = 2, quality = DownloadQuality.LOW)
        assertEquals(rule, offline.setDownloadRules(rule))
        assertEquals(rule, offline.getDownloadRules())
        offline.setDownloadRules(rule.copy(enabled = false))

        // Offline progress, uploaded at the next contact.
        val result = offline.uploadOfflineEvents(
            OfflineEventBatch(
                sentAt = OffsetDateTime.now(),
                events = listOf(
                    OfflineEvent(
                        profileId = signed.profileId, itemId = film.id, playbackId = "01HZY0000000000000000000P1", positionMs = 5_000,
                        durationMs = manifest.durationMs.coerceAtLeast(1), playedMs = 5_000, seq = 1, state = OfflineEvent.State.PAUSED,
                        clientTime = OffsetDateTime.now(), downloadId = created.id,
                    ),
                ),
            ),
        )!!
        assertEquals(1, result.accepted + result.duplicates)

        // Complete, then the public revocation list (device id, no session) names nothing.
        offline.syncOffline(OfflineSyncRequest(held = listOf(OfflineSyncRequestHeldInner(created.id, true))))
        val device = object : SessionCredentials {
            override val server = base
            override val bearerToken: String? = null
            override val deviceId: String = signed.deviceId
        }
        val public = SlouchcatApi.create(::OfflineApi, base, SlouchcatHttp.client(identity, device))
        assertEquals(emptyList<String>(), public.checkOfflineRevocations(RevocationCheck(listOf(created.id)))!!.revoked)

        // Deleted in the server: the next revocation check names it.
        offline.deleteDownload(created.id)
        assertEquals(listOf(created.id), public.checkOfflineRevocations(RevocationCheck(listOf(created.id)))!!.revoked)
        return created.id
    }

    private companion object {
        const val EMAIL = "contract-owner@example.test"
        const val PASSWORD = "contract-password-0123"
        const val DEVICE_NAME = "Contract phone offline · Android 16"
        const val LIBRARY = "Android playback media"
        const val TIMEOUT_NS = 120_000_000_000L
        const val POLL_MS = 1_000L
        const val CALL_TIMEOUT_S = 60L
        const val PREPARE_JOB = "download.prepare"
    }
}
