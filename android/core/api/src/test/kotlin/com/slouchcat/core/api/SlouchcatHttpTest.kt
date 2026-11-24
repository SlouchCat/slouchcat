package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.SystemApi
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.time.Duration
import java.util.concurrent.TimeUnit

class SlouchcatHttpTest {
    private val server = MockWebServer()
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "1.2.3")

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    private fun credentials(for_: HttpUrl) = object : SessionCredentials {
        override val server = for_
        override val bearerToken = "secret-token"
        override val deviceId = "device-1"
    }

    @Test
    fun idleConnectionsGoBeforeTheServerClosesThemAndAfterTheProofLapsed() {
        // The server closes an idle connection after two minutes (`IdleTimeout` in
        // server/cmd/slouchcat/main.go): one that stayed in the pool longer breaks under the next
        // request. And while a proof still counts without a new one, the app must still hold its
        // old connections, because where the network changed unseen it is their break that tells.
        assertTrue(SlouchcatHttp.KEEP_ALIVE < Duration.ofMinutes(2))
        assertTrue(SlouchcatHttp.PROOF_IDLE < SlouchcatHttp.KEEP_ALIVE)
        val http = SlouchcatHttp.client(identity, keepAlive = Duration.ofMillis(200))
        server.enqueue(MockResponse(body = "{}"))
        // Counted while the answer is open: once it is idle, a slow machine may find it gone already.
        // And read to its end, so that the connection goes back to the pool and is not just closed.
        http.newCall(Request.Builder().url(server.url("/")).build()).execute().use {
            assertEquals(1, http.connectionPool.connectionCount())
            assertEquals("{}", it.body.string())
        }
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (http.connectionPool.connectionCount() > 0 && System.nanoTime() < deadline) Thread.sleep(20)
        assertEquals("the pool kept an idle connection past its time", 0, http.connectionPool.connectionCount())
    }

    @Test
    fun everyRequestNamesTheAppAndOnlyWritesCarryTheCsrfHeader() {
        val http = SlouchcatHttp.client(identity)
        server.enqueue(MockResponse(body = "{}"))
        server.enqueue(MockResponse(code = 204))
        http.newCall(Request.Builder().url(server.url("/api/v1/healthz")).build()).execute().close()
        http.newCall(
            Request.Builder().url(server.url("/api/v1/auth/logout")).post("".toRequestBody("application/json".toMediaType())).build(),
        ).execute().close()

        val get = server.takeRequest()
        assertEquals("mobile/1.2.3", get.headers[SlouchcatHttp.HEADER_CLIENT])
        assertNull(get.headers[SlouchcatHttp.HEADER_REQUESTED_WITH])
        assertNull(get.headers["Authorization"])
        val post = server.takeRequest()
        assertEquals("Slouchcat", post.headers[SlouchcatHttp.HEADER_REQUESTED_WITH])
    }

    @Test
    fun credentialsGoOnlyToTheirServer() {
        val own = server.url("/")
        val http = SlouchcatHttp.client(identity, credentials(own))
        server.enqueue(MockResponse(body = "{}"))
        http.newCall(Request.Builder().url(server.url("/api/v1/auth/session")).build()).execute().close()
        val sent = server.takeRequest()
        assertEquals("Bearer secret-token", sent.headers["Authorization"])
        assertEquals("device-1", sent.headers[SlouchcatHttp.HEADER_DEVICE])

        // Same host, another port: another origin.
        val other = MockWebServer().apply { start() }
        other.use {
            val foreign = SlouchcatHttp.client(identity, credentials(own))
            other.enqueue(MockResponse(body = "{}"))
            foreign.newCall(Request.Builder().url(other.url("/image.webp")).build()).execute().close()
            val leaked = other.takeRequest()
            assertNull(leaked.headers["Authorization"])
            assertNull(leaked.headers[SlouchcatHttp.HEADER_DEVICE])
            assertEquals("mobile/1.2.3", leaked.headers[SlouchcatHttp.HEADER_CLIENT])
        }
    }

    @Test
    fun aRedirectToAnotherServerCarriesNoCredentials() {
        val other = MockWebServer().apply { start() }
        other.use {
            val http = SlouchcatHttp.client(identity, credentials(server.url("/")))
            server.enqueue(MockResponse(code = 302, headers = okhttp3.Headers.headersOf("Location", other.url("/elsewhere").toString())))
            other.enqueue(MockResponse(body = "{}"))
            http.newCall(Request.Builder().url(server.url("/api/v1/items/1/image")).build()).execute().use {
                assertEquals(200, it.code)
            }
            val first = server.takeRequest()
            assertEquals("Bearer secret-token", first.headers["Authorization"])
            val second = other.takeRequest()
            assertEquals("/elsewhere", second.url.encodedPath)
            assertNull(second.headers["Authorization"])
            assertNull(second.headers[SlouchcatHttp.HEADER_DEVICE])
            assertEquals("mobile/1.2.3", second.headers[SlouchcatHttp.HEADER_CLIENT])
        }
    }

    @Test
    fun aRedirectWithinTheServerKeepsThem() {
        val http = SlouchcatHttp.client(identity, credentials(server.url("/")))
        server.enqueue(MockResponse(code = 302, headers = okhttp3.Headers.headersOf("Location", "/api/v1/next")))
        server.enqueue(MockResponse(body = "{}"))
        http.newCall(Request.Builder().url(server.url("/api/v1/first")).build()).execute().close()
        server.takeRequest()
        val second = server.takeRequest()
        assertEquals("/api/v1/next", second.url.encodedPath)
        assertEquals("device-1", second.headers[SlouchcatHttp.HEADER_DEVICE])
    }

    @Test
    fun httpsDoesNotFollowRedirectsToHttp() {
        assertEquals(false, SlouchcatHttp.client(identity).followSslRedirects)
    }

    @Test
    fun theBasePathFollowsTheServerAddress() {
        assertEquals("https://media.example.test/api/v1", SlouchcatApi.basePath("https://media.example.test/".toHttpUrl()))
        assertEquals("https://media.example.test/api/v1", SlouchcatApi.basePath("https://media.example.test".toHttpUrl()))
        assertEquals("http://192.0.2.10:8096/sub/api/v1", SlouchcatApi.basePath("http://192.0.2.10:8096/sub".toHttpUrl()))
    }

    @Test
    fun theGeneratedClientRunsOverTheSharedClient() {
        server.enqueue(MockResponse(body = """{"status":"ok","version":"1.42.0","contractVersion":"1.42.0","serverKey":"k","uptimeSeconds":12}"""))
        val api = SlouchcatApi.create(::SystemApi, server.url("/"), SlouchcatHttp.client(identity))
        assertEquals("1.42.0", api.getHealth()?.version)
        assertEquals("/api/v1/healthz", server.takeRequest().url.encodedPath)
    }
}
