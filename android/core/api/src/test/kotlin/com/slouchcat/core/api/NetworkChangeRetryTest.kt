package com.slouchcat.core.api

import com.google.crypto.tink.subtle.Ed25519Sign
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.ServerProof
import mockwebserver3.Dispatcher
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import mockwebserver3.RecordedRequest
import mockwebserver3.SocketEffect
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import java.io.IOException
import java.time.Duration
import java.util.Collections
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong

/**
 * A request that a network change ended on its way, in the shared client with the gate (ADR 0040,
 * addendum of 24 November 2026): a read asks once more in the new network, a write does not.
 * The case of CI run 45253038403: right after the sign-in the emulator changed its addresses, the
 * gate stopped `listMyProfiles` at its hop, and the profile picker stayed on «not reachable».
 */
class NetworkChangeRetryTest {
    private val server = MockWebServer()
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "1.0.0")
    private val keys = Ed25519Sign.KeyPair.newKeyPair()
    private val generation = AtomicLong(1)
    private val proofs = AtomicInteger()

    /** The paths that reached the server, in order. */
    private val seen = Collections.synchronizedList(mutableListOf<String>())

    /** What the server does with a request; the default answers. */
    @Volatile private var answer: (RecordedRequest) -> MockResponse? = { MockResponse(body = "{}") }

    @Before
    fun start() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                seen += request.url.encodedPath
                return answer(request) ?: MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            }
        }
        server.start()
    }

    @After
    fun stop() = server.close()

    private val prover = ServerProver { _, nonce ->
        proofs.incrementAndGet()
        ProofAnswer(Base64Url.encode(Ed25519Sign(keys.privateKey).sign(ServerProof.message(nonce))), Base64Url.encode(keys.publicKey))
    }

    private val trusted = object : TrustedServer {
        override val server: HttpUrl get() = this@NetworkChangeRetryTest.server.url("/")
        override val serverKey: String = Base64Url.encode(keys.publicKey)
    }

    private val credentials = object : SessionCredentials {
        override val server: HttpUrl get() = this@NetworkChangeRetryTest.server.url("/")
        override val bearerToken = "app-token"
        override val deviceId = "0123456789abcdef0123456789abcdef"
    }

    /**
     * The shared client as the apps build it. [changesBeforeHop] network changes come between the
     * proof and the hop of a request, one per hop, like the addresses of the emulator after the sign-in.
     */
    private fun client(changesBeforeHop: Int = 0, prover: ServerProver = this.prover): OkHttpClient {
        val left = AtomicInteger(changesBeforeHop)
        val builder = OkHttpClient.Builder().addNetworkInterceptor { chain ->
            // Before the gate's hop (the network interceptors of the builder come first).
            if (left.getAndDecrement() > 0) generation.incrementAndGet()
            chain.proceed(chain.request())
        }
        val gate = ServerIdentityGate(trusted, { generation.get() }, prover, timeout = Duration.ofSeconds(5))
        return SlouchcatHttp.client(identity, credentials, builder, gate, network = { generation.get() })
    }

    private fun get(http: OkHttpClient, path: String = "/api/v1/me/profiles"): Int =
        http.newCall(Request.Builder().url(server.url(path)).build()).execute().use { it.code }

    private fun post(http: OkHttpClient, path: String = "/api/v1/me/profiles/1/select"): Int = http.newCall(
        Request.Builder().url(server.url(path)).post("{}".toRequestBody("application/json".toMediaType())).build(),
    ).execute().use { it.code }

    private fun refused(block: () -> Unit): IOException {
        try {
            block()
        } catch (e: IOException) {
            return e
        }
        fail("the request got an answer")
        error("unreachable")
    }

    @Test
    fun aReadTheGateStoppedForANetworkChangeAsksAgainInTheNewNetwork() {
        val http = client(changesBeforeHop = 1)
        assertEquals(200, get(http))
        assertEquals("one proof per network", 2, proofs.get())
        assertEquals("nothing left in the old network", listOf("/api/v1/me/profiles"), seen.toList())
    }

    @Test
    fun aReadWhoseConnectionBrokeWithTheOldNetworkAsksAgain() {
        val first = AtomicInteger()
        answer = { if (first.getAndIncrement() == 0) null.also { generation.incrementAndGet() } else MockResponse(body = "{}") }
        val http = client()
        assertEquals(200, get(http))
        assertEquals(2, proofs.get())
        assertEquals(listOf("/api/v1/me/profiles", "/api/v1/me/profiles"), seen.toList())
    }

    @Test
    fun aWriteIsNotSentAgain() {
        // The hop may have reached the server before it broke; the app does not know.
        val stopped = client(changesBeforeHop = 1)
        assertTrue(refused { post(stopped) } is ServerNotProvenException)
        assertEquals(emptyList<String>(), seen.toList())

        val first = AtomicInteger()
        answer = { if (first.getAndIncrement() == 0) null.also { generation.incrementAndGet() } else MockResponse(body = "{}") }
        refused { post(client()) }
        assertEquals("sent once", listOf("/api/v1/me/profiles/1/select"), seen.toList())
    }

    @Test
    fun aReadAsksAgainOnlyOnce() {
        val http = client(changesBeforeHop = 2)
        assertTrue(refused { get(http) } is ServerNotProvenException)
        assertEquals(emptyList<String>(), seen.toList())
        assertEquals(2, proofs.get())
    }

    @Test
    fun aFailureInTheSameNetworkIsNotRepeated() {
        // The server refuses to prove itself: no network change, so no second try (and no second proof).
        val calls = AtomicInteger()
        val http = client(prover = { _, _ -> null.also { calls.incrementAndGet() } })
        refused { get(http) }
        assertEquals(1, calls.get())
        assertEquals(emptyList<String>(), seen.toList())
    }
}
