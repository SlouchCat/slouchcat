package com.slouchcat.core.offline

import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.launch
import kotlinx.coroutines.async
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.util.concurrent.CountDownLatch

/** The fetch with Range and If-Range (9.2 point 3, stage 0 point 6): resume, start over, errors, full storage. */
class FileFetcherTest {
    @get:Rule val temp = TemporaryFolder()
    private lateinit var server: MockWebServer
    private val body = ByteArray(512 * 1024) { (it % 251).toByte() }

    @Before
    fun start() {
        server = MockWebServer().apply { start() }
    }

    @After
    fun stop() = server.close()

    private fun fetcher(free: Long = Long.MAX_VALUE) = FileFetcher(OkHttpClient(), { free }, syncEvery = 16 * 1024, reserve = 100)

    @Test
    fun `a whole file comes in and every durable length is reported`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v1\"").body(Buffer().write(body)).build())
        val target = temp.newFile()
        val durable = mutableListOf<Long>()
        val outcome = fetcher().fetch(server.url("/f"), target, 0, body.size.toLong(), null, { length, _ -> durable += length; true })
        assertEquals(FetchOutcome.Done("\"v1\""), outcome)
        assertArrayEquals(body, target.readBytes())
        // Several durable points on the way, rising, the last one the whole file.
        assertTrue(durable.size > 2)
        assertEquals(durable.sorted(), durable)
        assertEquals(body.size.toLong(), durable.last())
        assertNull(server.takeRequest().headers["Range"])
    }

    @Test
    fun `a resume asks for the rest with If-Range and appends`() = runBlocking {
        val target = temp.newFile().apply { writeBytes(body.copyOf(1000)) }
        server.enqueue(MockResponse.Builder().code(206).addHeader("Content-Range", "bytes 1000-${body.size - 1}/${body.size}").addHeader("ETag", "\"v1\"").body(Buffer().write(body.copyOfRange(1000, body.size))).build())
        val outcome = fetcher().fetch(server.url("/f"), target, 1000, body.size.toLong(), "\"v1\"", { _, _ -> true })
        assertEquals(FetchOutcome.Done("\"v1\""), outcome)
        assertArrayEquals(body, target.readBytes())
        val request = server.takeRequest()
        assertEquals("bytes=1000-", request.headers["Range"])
        assertEquals("\"v1\"", request.headers["If-Range"])
    }

    @Test
    fun `a 200 to a resume is another prepared file and says changed`() = runBlocking {
        val target = temp.newFile().apply { writeBytes(ByteArray(1000) { 9 }) }
        server.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v2\"").body(Buffer().write(body)).build())
        val outcome = fetcher().fetch(server.url("/f"), target, 1000, body.size.toLong(), "\"v1\"", { _, _ -> true })
        assertEquals(FetchOutcome.Failed(FetchError.CHANGED, 200), outcome)
        // The old bytes stay untouched; the engine fetches the manifest again and starts over.
        assertEquals(1000L, target.length())
    }

    @Test
    fun `a download no longer wanted stops at the next durable point`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val target = temp.newFile()
        val outcome = fetcher().fetch(server.url("/f"), target, 0, body.size.toLong(), null, { _, _ -> false })
        assertEquals(FetchOutcome.Stopped, outcome)
        assertTrue(target.length() in 16L * 1024 until body.size.toLong())
    }

    @Test
    fun `the deadline stops inside a file with the bytes durable`() = runBlocking {
        var now = 0L
        val fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }, syncEvery = 16 * 1024, reserve = 100, clock = { now.also { now += 1_000 } })
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val durable = mutableListOf<Long>()
        val outcome = fetcher.fetch(server.url("/f"), temp.newFile(), 0, body.size.toLong(), null, { length, _ -> durable += length; true }, deadline = 5_000)
        assertEquals(FetchOutcome.Stopped, outcome)
        assertTrue(durable.last() < body.size)
    }

    @Test
    fun `progress comes at most every half second`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val reports = mutableListOf<Long>()
        val fetcher = FileFetcher(OkHttpClient(), { Long.MAX_VALUE }, clock = { 0L })
        fetcher.fetch(server.url("/f"), temp.newFile(), 0, body.size.toLong(), null, { _, _ -> true }, onProgress = { reports += it })
        // The clock stands still: no report on the way, only the last one.
        assertEquals(listOf(body.size.toLong()), reports)
    }

    @Test
    fun `cancelling a fetch whose connection hangs ends at once, not after the read timeout`() = runBlocking {
        // The head and 1 KiB, then silence for a minute.
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).throttleBody(1024, 60, java.util.concurrent.TimeUnit.SECONDS).build())
        val client = OkHttpClient.Builder().readTimeout(java.time.Duration.ofSeconds(60)).build()
        val job = launch(Dispatchers.Default) { FileFetcher(client, { Long.MAX_VALUE }).fetch(server.url("/f"), temp.newFile(), 0, body.size.toLong(), null, { _, _ -> true }) }
        server.takeRequest()
        val started = System.nanoTime()
        withTimeout(10_000) { job.cancelAndJoin() }
        assertTrue("took ${(System.nanoTime() - started) / 1_000_000} ms", System.nanoTime() - started < 5_000_000_000L)
    }

    @Test
    fun `the server's answers become stable codes`() = runBlocking {
        for ((status, code) in listOf(410 to FetchError.EVICTED, 404 to FetchError.GONE, 401 to FetchError.UNAUTHORIZED, 503 to FetchError.NETWORK)) {
            server.enqueue(MockResponse.Builder().code(status).build())
            assertEquals(FetchOutcome.Failed(code, status), fetcher().fetch(server.url("/f"), temp.newFile(), 0, 10, null, { _, _ -> true }))
        }
        // A range that starts elsewhere than asked is no resume.
        server.enqueue(MockResponse.Builder().code(206).addHeader("Content-Range", "bytes 0-${body.size - 1}/${body.size}").body(Buffer().write(body)).build())
        val outcome = fetcher().fetch(server.url("/f"), temp.newFile().apply { writeBytes(ByteArray(10)) }, 10, body.size.toLong(), "\"v1\"", { _, _ -> true })
        assertEquals(FetchError.RANGE, (outcome as FetchOutcome.Failed).code)
    }

    @Test
    fun `a full device stops with quota and keeps the durable part`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val durable = mutableListOf<Long>()
        val outcome = fetcher(free = 50).fetch(server.url("/f"), temp.newFile(), 0, body.size.toLong(), null, { length, _ -> durable += length; true })
        assertEquals(FetchOutcome.Failed(FetchError.QUOTA), outcome)
        assertEquals(1, durable.size)
        assertTrue(durable.single() in 16L * 1024 until body.size.toLong())
    }

    @Test
    fun `a file longer than the manifest says fails with size`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val outcome = fetcher().fetch(server.url("/f"), temp.newFile(), 0, 100, null, { _, _ -> true })
        assertEquals(FetchError.SIZE, (outcome as FetchOutcome.Failed).code)
    }

    @Test
    fun `cancelling stops the fetch and keeps what was durable`() = runBlocking {
        val gate = CountDownLatch(1)
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).throttleBody(16 * 1024, 200, java.util.concurrent.TimeUnit.MILLISECONDS).build())
        val target = temp.newFile()
        val job = async(start = CoroutineStart.UNDISPATCHED) {
            fetcher().fetch(server.url("/f"), target, 0, body.size.toLong(), null, { length, _ -> if (length >= 16 * 1024) gate.countDown(); true })
        }
        withTimeout(10_000) { while (gate.count > 0) kotlinx.coroutines.yield() }
        job.cancel()
        runCatching { job.await() }
        assertTrue(job.isCancelled)
        assertTrue(target.length() in 16L * 1024 until body.size.toLong())
    }
}
