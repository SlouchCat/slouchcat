package com.slouchcat.core.offline

import androidx.test.core.app.ApplicationProvider
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.generated.models.DownloadStateValue
import com.slouchcat.core.api.generated.models.PlaybackRequest
import com.slouchcat.core.api.generated.models.ProgressEvent
import com.slouchcat.core.api.generated.models.RevocationList
import com.slouchcat.core.offline.db.OfflineDatabase
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.async
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.yield
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

/** The engine over a real database (in memory), files in a temporary folder and a server in memory. */
@RunWith(RobolectricTestRunner::class)
class OfflineEngineTest {
    @get:Rule val temp = TemporaryFolder()
    private lateinit var db: OfflineDatabase
    private lateinit var files: OfflineFiles
    private lateinit var web: MockWebServer
    private val server = FakeOfflineServer()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var who: OfflineIdentity? = identity(1)
    private var network = NetworkKind.Wifi
    private var clock = 5_000L
    private var bootCount = 1
    private val body = ByteArray(1010) { it.toByte() }

    private val engine by lazy {
        OfflineEngine(
            store = OfflineStore(db.dao()),
            files = files,
            server = server,
            fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }, syncEvery = 256, reserve = 0),
            session = { who },
            network = { network },
            wifiOnly = { false },
            scope = scope,
            clock = { clock },
            elapsed = { clock },
            boot = { bootCount },
        )
    }

    @Before
    fun setUp() {
        db = OfflineDatabase.inMemory(ApplicationProvider.getApplicationContext())
        files = OfflineFiles(temp.newFolder("offline"))
        web = MockWebServer().apply { start() }
        server.base = web.url("/")
    }

    @After
    fun tearDown() {
        scope.cancel()
        db.close()
        web.close()
    }

    private fun serveFile() {
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(body)).build())
    }

    @Test
    fun `a ready download is fetched, gets its playlist and is reported complete at the next sync`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        assertEquals(Contact.Online, engine.sync())
        serveFile()
        assertEquals(FetchRun.Done, engine.runFetches())
        val stored = engine.download(D1)!!
        assertTrue(stored.complete)
        assertEquals(1010, stored.bytesWritten)
        assertTrue(files.playlist(D1).readText().contains("#EXT-X-BYTERANGE:100@10"))
        engine.sync()
        assertEquals(true, server.syncs.last().held.single { it.id == D1 }.complete)
    }

    @Test
    fun `revoked downloads lose their files at the next contact`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        engine.sync()
        serveFile()
        engine.runFetches()
        assertTrue(files.media(D1).exists())
        server.syncAnswer = ApiResult.Ok(answer(revoked = listOf(D1)))
        engine.sync()
        assertNull(engine.download(D1))
        assertFalse(files.folder(D1).exists())
    }

    @Test
    fun `without a session the device asks the revocation list (an account taken away)`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1), state(D2))))
        engine.sync()
        files.folder(D1).mkdirs()
        who = null
        server.revoked = ApiResult.Ok(RevocationList(listOf(D1, D2)))
        engine.sync()
        assertEquals(listOf(D1, D2), server.revocationChecks.last().sorted())
        assertTrue(engine.allDownloads().isEmpty())
        assertFalse(files.folder(D1).exists())
    }

    @Test
    fun `a delete without an answer waits and goes out at the next contact`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        engine.sync()
        server.deleteAnswer = ApiResult.Offline
        engine.delete(listOf(D1))
        assertTrue(engine.download(D1)!!.pendingDelete)
        server.deleteAnswer = ApiResult.Ok(Unit)
        // The answer in flight still lists it; the recent delete keeps it out.
        engine.sync()
        assertNull(engine.download(D1))
        assertEquals(listOf(D1, D1), server.deletes)
    }

    @Test
    fun `an interrupted fetch resumes at the durable length`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        engine.sync()
        // The connection breaks after 600 bytes.
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(body.copyOf(600))).addHeader("Content-Length", "1010").build())
        engine.runFetches()
        val broken = engine.download(D1)!!
        assertFalse(broken.complete)
        assertEquals(FetchError.NETWORK, broken.fetchError)
        assertTrue(broken.bytesWritten in 256L..600L)
        clock += FETCH_RETRY_MS
        web.enqueue(MockResponse.Builder().code(206).addHeader("Content-Range", "bytes ${broken.bytesWritten}-1009/1010").addHeader("ETag", "\"v1\"").body(Buffer().write(body.copyOfRange(broken.bytesWritten.toInt(), 1010))).build())
        engine.runFetches()
        assertTrue(engine.download(D1)!!.complete)
        assertTrue(body.contentEquals(files.media(D1).readBytes()))
        web.takeRequest()
        assertEquals("bytes=${broken.bytesWritten}-", web.takeRequest().headers["Range"])
    }

    @Test
    fun `without network the contact is offline`() = runBlocking {
        network = NetworkKind.None
        assertEquals(Contact.Offline, engine.sync())
        assertEquals(Contact.Offline, engine.contact.value)
        server.syncAnswer = ApiResult.Offline
        network = NetworkKind.Wifi
        assertEquals(Contact.Offline, engine.sync())
    }

    @Test
    fun `the offline player plays only what the server allowed, queues progress and counts watch time`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1, allowed = listOf(1)))))
        server.manifests[D1] = manifest(subtitles = true)
        engine.sync()
        serveFile()
        web.enqueue(MockResponse.Builder().code(200).body("WEBVTT\n").build())
        engine.runFetches()
        assertTrue(engine.download(D1)!!.complete)
        val other = OfflinePlaybackServer(engine, { ACCOUNT }, profileId = 2, clock = { clock })
        assertEquals(ApiResult.NotFound, other.start("i1", PlaybackRequest()))
        val player = OfflinePlaybackServer(engine, { ACCOUNT }, profileId = 1, clock = { clock })
        val info = (player.start("i1", PlaybackRequest(subtitleTrack = 7)) as ApiResult.Ok).value
        assertTrue(info.streamUrl.startsWith("file://") && info.streamUrl.endsWith("media.m3u8"))
        assertNotNull(info.subtitleUrl)
        assertEquals(info.streamUrl, player.absolute(info.streamUrl))
        assertNull(player.absolute("/api/v1/items/i1"))
        player.progress("i1", ProgressEvent(positionMs = 15_000, durationMs = 20_000, playedMs = 30_000, seq = 1, state = ProgressEvent.State.PAUSED, clientTime = at(clock), playbackSessionId = info.playbackSessionId))
        assertEquals(15_000L, engine.download(D1)!!.positions["1"]?.positionMs)
        engine.sync()
        val uploaded = server.uploads.single().events.single()
        assertEquals(D1, uploaded.downloadId)
        assertEquals(1L, uploaded.profileId)
        // The next start resumes where offline playback stopped.
        assertEquals(15_000L, (player.start("i1", PlaybackRequest()) as ApiResult.Ok).value.resumePositionMs)
    }

    @Test
    fun `the PIN brake survives a new sync and locks after the third failure`() = runBlocking {
        val salt = "00112233445566778899aabbccddeeff"
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1)), profiles = listOf(profile(1, hasPin = true, salt = salt).copy(pinIterations = 1000))))
        engine.sync()
        engine.pinEntered(1, "4711".toCharArray())
        assertNotNull(engine.profile(ACCOUNT, 1)!!.pinVerifier)
        assertEquals(PinCheck.Ok, engine.checkPin(1, "4711".toCharArray()))
        assertEquals(PinCheck.Wrong, engine.checkPin(1, "0000".toCharArray()))
        assertEquals(PinCheck.Wrong, engine.checkPin(1, "0000".toCharArray()))
        engine.sync()
        assertTrue(engine.checkPin(1, "0000".toCharArray()) is PinCheck.Locked)
        assertTrue(engine.checkPin(1, "4711".toCharArray()) is PinCheck.Locked)
        clock += 31_000
        assertEquals(PinCheck.Ok, engine.checkPin(1, "4711".toCharArray()))
    }

    @Test
    fun `sign-out with delete removes the downloads of the account in the server and on the device`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1), state(D2, value = DownloadStateValue.QUEUED))))
        engine.sync()
        files.folder(D2).mkdirs()
        engine.beforeSignOut(deleteAll = true)
        assertTrue(engine.allDownloads().isEmpty())
        assertEquals(setOf(D1, D2), server.deletes.toSet())
        assertFalse(files.folder(D2).exists())
    }

    /** A big file the server sends slowly: a fetch that is still running when the test acts. */
    private fun serveSlowly(size: Int = 4 * 1024 * 1024) {
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(ByteArray(size))).throttleBody(64 * 1024, 100, java.util.concurrent.TimeUnit.MILLISECONDS).build())
    }

    private suspend fun readyForSlowFetch(size: Long = 4L * 1024 * 1024) {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest(size = size + 10).copy(sizeBytes = size, segments = listOf(com.slouchcat.core.api.generated.models.DownloadSegment(0, 1000, 10, size - 10)))
        engine.sync()
        serveSlowly(size.toInt())
    }

    private suspend fun awaitBytesFlowing() {
        withTimeout(10_000) { while ((engine.progress.value?.bytes ?: 0) == 0L) delay(20) }
    }

    @Test
    fun `a cancelled caller stops its fetch and the next run is the only writer`() = runBlocking {
        readyForSlowFetch()
        val run = launch(Dispatchers.Default) { engine.runFetches() }
        awaitBytesFlowing()
        // WorkManager or the system stops the run: the fetch ends with it (code review B-1).
        withTimeout(10_000) { run.cancelAndJoin() }
        val stopped = files.media(D1).length()
        // Nothing writes on in the background.
        val later = System.nanoTime() + 300_000_000
        while (System.nanoTime() < later) yield()
        assertEquals(stopped, files.media(D1).length())
        assertNull(engine.progress.value)
    }

    @Test
    fun `deleting a download while it comes in stops the fetch and it does not start again`() = runBlocking {
        readyForSlowFetch()
        val run = launch(Dispatchers.Default) { engine.runFetches() }
        awaitBytesFlowing()
        engine.delete(listOf(D1))
        withTimeout(10_000) { run.join() }
        assertNull(engine.download(D1))
        assertFalse(files.folder(D1).exists())
        // One request for the file, no second one after the delete (code review M-1).
        assertEquals(1, web.requestCount)
    }

    @Test
    fun `a cold process waits for the stored session before it fetches`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        engine.sync()
        // The process is new: the session answers only after its stores are read (code review B-2).
        val stores = kotlinx.coroutines.CompletableDeferred<Unit>()
        val cold = OfflineEngine(
            store = OfflineStore(db.dao()),
            files = files,
            server = server,
            fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }, syncEvery = 256, reserve = 0),
            session = { stores.await(); identity(1) },
            network = { network },
            wifiOnly = { false },
            scope = scope,
            clock = { clock },
        )
        serveFile()
        val run = async(Dispatchers.Default) { cold.runFetches() }
        delay(100)
        stores.complete(Unit)
        assertEquals(FetchRun.Done, withTimeout(10_000) { run.await() })
        assertTrue(cold.download(D1)!!.complete)
    }

    @Test
    fun `sign-out with delete and no server keeps tombstones that go out at the next sign-in`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        engine.sync()
        server.deleteAnswer = ApiResult.Offline
        engine.beforeSignOut(deleteAll = true)
        assertTrue(engine.allDownloads().isEmpty())
        // The same account signs in again; the server still lists the download until it hears the delete.
        server.deleteAnswer = ApiResult.Ok(Unit)
        server.deletes.clear()
        engine.sync()
        assertEquals(listOf(D1), server.deletes)
        assertTrue("the download came back", engine.allDownloads().isEmpty())
    }

    @Test
    fun `another prepared copy (200 to a resume) fetches the manifest again`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        engine.sync()
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(body.copyOf(600))).addHeader("Content-Length", "1010").build())
        engine.runFetches()
        clock += FETCH_RETRY_MS
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v2\"").body(Buffer().write(body)).build())
        serveFile()
        engine.runFetches()
        assertTrue(engine.download(D1)!!.complete)
        web.takeRequest()
        assertNotNull(web.takeRequest().headers["Range"])
        // After «changed» the file starts over from byte 0, with the manifest fetched again.
        assertNull(web.takeRequest().headers["Range"])
        assertEquals(2, server.manifestCalls)
    }

    @Test
    fun `one failed sync is not yet offline, two are`() = runBlocking {
        server.syncAnswer = ApiResult.Offline
        engine.sync()
        assertEquals(Contact.Unknown, engine.contact.value)
        engine.sync()
        assertEquals(Contact.Offline, engine.contact.value)
        server.syncAnswer = ApiResult.Ok(answer())
        engine.sync()
        assertEquals(Contact.Online, engine.contact.value)
    }

    @Test
    fun `a restart of the phone with the date set forward does not end a pause`() = runBlocking {
        val salt = "00112233445566778899aabbccddeeff"
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1)), profiles = listOf(profile(1, hasPin = true, salt = salt).copy(pinIterations = 1000))))
        engine.sync()
        engine.pinEntered(1, "4711".toCharArray())
        repeat(2) { engine.checkPin(1, "0000".toCharArray()) }
        assertTrue(engine.checkPin(1, "0000".toCharArray()) is PinCheck.Locked)
        // Restart: a new boot, the clock since boot starts near 0, the date a day ahead.
        bootCount = 2
        clock = 1_000
        val locked = engine.checkPin(1, "4711".toCharArray())
        assertTrue("$locked", locked is PinCheck.Locked && locked.remainingMs > 25_000)
        clock += 31_000
        assertEquals(PinCheck.Ok, engine.checkPin(1, "4711".toCharArray()))
    }

    @Test
    fun `downloads print neither token nor title`() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        engine.sync()
        val text = engine.download(D1)!!.copy(manifest = manifest()).toString()
        assertFalse(text.contains("dt="))
        assertFalse(text.contains("Morgenrot"))
    }
}
