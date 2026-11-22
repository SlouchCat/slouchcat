package com.slouchcat.core.offline

import android.app.Application
import androidx.test.core.app.ApplicationProvider
import androidx.work.ListenableWorker
import androidx.work.testing.TestListenableWorkerBuilder
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.offline.background.OfflineHost
import com.slouchcat.core.offline.background.OfflineWorker
import com.slouchcat.core.offline.db.OfflineDatabase
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** An application that hands the worker its engine, as the app does. */
class OfflineTestApp : Application(), OfflineHost {
    override lateinit var offlineEngine: OfflineEngine
}

/**
 * The background run of WorkManager (code review B-2, m-9): a cold process (the session answers only
 * after its stores are read) syncs and fetches a pending download; the run ends with success, or asks
 * again when work is left.
 */
@RunWith(RobolectricTestRunner::class)
@Config(application = OfflineTestApp::class)
class OfflineWorkerTest {
    private val app = ApplicationProvider.getApplicationContext<OfflineTestApp>()
    private val web = MockWebServer().apply { start() }
    private val server = FakeOfflineServer(web.url("/"))
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val db = OfflineDatabase.inMemory(app)
    private val files = OfflineFiles(app.cacheDir.resolve("worker-offline").apply { deleteRecursively() })
    private val stores = CompletableDeferred<Unit>()

    init {
        app.offlineEngine = OfflineEngine(
            store = OfflineStore(db.dao()),
            files = files,
            server = server,
            fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }, syncEvery = 256, reserve = 0),
            // A cold process: the session is there only after the stores are read.
            session = { stores.await(); identity(1) },
            network = { NetworkKind.Wifi },
            wifiOnly = { false },
            scope = scope,
        )
    }

    @After
    fun tearDown() {
        db.close()
        web.close()
    }

    @Test
    fun aColdWorkerWaitsForTheSessionAndFetchesThePendingDownload() = runBlocking {
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(ByteArray(1010))).build())
        val worker = TestListenableWorkerBuilder<OfflineWorker>(app).build()
        val result = async(Dispatchers.Default) { worker.doWork() }
        delay(100)
        stores.complete(Unit)
        assertEquals(ListenableWorker.Result.success(), withTimeout(20_000) { result.await() })
        assertTrue(app.offlineEngine.download(D1)!!.complete)
    }

    @Test
    fun workLeftAtTheEndOfARunAsksForAnotherRun() = runBlocking {
        stores.complete(Unit)
        server.syncAnswer = ApiResult.Ok(answer(downloads = listOf(state(D1))))
        server.manifests[D1] = manifest()
        // The server breaks the connection after 600 bytes: a device error, again after its pause.
        web.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(ByteArray(600))).addHeader("Content-Length", "1010").build())
        val worker = TestListenableWorkerBuilder<OfflineWorker>(app).build()
        assertEquals(ListenableWorker.Result.success(), withTimeout(20_000) { worker.doWork() })
        assertEquals(FetchError.NETWORK, app.offlineEngine.download(D1)!!.fetchError)
    }
}
