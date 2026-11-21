package com.slouchcat.core.offline

import kotlinx.coroutines.CoroutineStart
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
        val outcome = fetcher().fetch(server.url("/f"), target, 0, body.size.toLong(), null, { length, _ -> durable += length })
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
        val outcome = fetcher().fetch(server.url("/f"), target, 1000, body.size.toLong(), "\"v1\"", { _, _ -> })
        assertEquals(FetchOutcome.Done("\"v1\""), outcome)
        assertArrayEquals(body, target.readBytes())
        val request = server.takeRequest()
        assertEquals("bytes=1000-", request.headers["Range"])
        assertEquals("\"v1\"", request.headers["If-Range"])
    }

    @Test
    fun `a 200 to a resume (another prepared file) starts the file over`() = runBlocking {
        val target = temp.newFile().apply { writeBytes(ByteArray(1000) { 9 }) }
        server.enqueue(MockResponse.Builder().code(200).addHeader("ETag", "\"v2\"").body(Buffer().write(body)).build())
        val outcome = fetcher().fetch(server.url("/f"), target, 1000, body.size.toLong(), "\"v1\"", { _, _ -> })
        assertEquals(FetchOutcome.Done("\"v2\""), outcome)
        assertArrayEquals(body, target.readBytes())
    }

    @Test
    fun `the server's answers become stable codes`() = runBlocking {
        for ((status, code) in listOf(410 to FetchError.EVICTED, 404 to FetchError.GONE, 401 to FetchError.UNAUTHORIZED, 503 to FetchError.NETWORK)) {
            server.enqueue(MockResponse.Builder().code(status).build())
            assertEquals(FetchOutcome.Failed(code, status), fetcher().fetch(server.url("/f"), temp.newFile(), 0, 10, null, { _, _ -> }))
        }
        // A range that starts elsewhere than asked is no resume.
        server.enqueue(MockResponse.Builder().code(206).addHeader("Content-Range", "bytes 0-${body.size - 1}/${body.size}").body(Buffer().write(body)).build())
        val outcome = fetcher().fetch(server.url("/f"), temp.newFile().apply { writeBytes(ByteArray(10)) }, 10, body.size.toLong(), "\"v1\"", { _, _ -> })
        assertEquals(FetchError.RANGE, (outcome as FetchOutcome.Failed).code)
    }

    @Test
    fun `a full device stops with quota and keeps the durable part`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val durable = mutableListOf<Long>()
        val outcome = fetcher(free = 50).fetch(server.url("/f"), temp.newFile(), 0, body.size.toLong(), null, { length, _ -> durable += length })
        assertEquals(FetchOutcome.Failed(FetchError.QUOTA), outcome)
        assertEquals(1, durable.size)
        assertTrue(durable.single() in 16L * 1024 until body.size.toLong())
    }

    @Test
    fun `a file longer than the manifest says fails with size`() = runBlocking {
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).build())
        val outcome = fetcher().fetch(server.url("/f"), temp.newFile(), 0, 100, null, { _, _ -> })
        assertEquals(FetchError.SIZE, (outcome as FetchOutcome.Failed).code)
    }

    @Test
    fun `cancelling stops the fetch and keeps what was durable`() = runBlocking {
        val gate = CountDownLatch(1)
        server.enqueue(MockResponse.Builder().code(200).body(Buffer().write(body)).throttleBody(16 * 1024, 200, java.util.concurrent.TimeUnit.MILLISECONDS).build())
        val target = temp.newFile()
        val job = async(start = CoroutineStart.UNDISPATCHED) {
            fetcher().fetch(server.url("/f"), target, 0, body.size.toLong(), null, { length, _ -> if (length >= 16 * 1024) gate.countDown() })
        }
        withTimeout(10_000) { while (gate.count > 0) kotlinx.coroutines.yield() }
        job.cancel()
        runCatching { job.await() }
        assertTrue(job.isCancelled)
        assertTrue(target.length() in 16L * 1024 until body.size.toLong())
    }
}
