package com.slouchcat.core.api

import com.google.crypto.tink.subtle.Ed25519Sign
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.ServerProof
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import mockwebserver3.Dispatcher
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import mockwebserver3.RecordedRequest
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import java.io.IOException
import java.time.Duration
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong

/** «Server weist sich aus» (concept 27 section 6, F-38): the gate of the shared client. */
class ServerIdentityGateTest {
    private val server = MockWebServer()
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "1.0.0")
    private val serverKeys = Ed25519Sign.KeyPair.newKeyPair()
    private val serverKey = Base64Url.encode(serverKeys.publicKey)
    private val otherKeys = Ed25519Sign.KeyPair.newKeyPair()
    private val otherKey = Base64Url.encode(otherKeys.publicKey)
    private val generation = AtomicLong(1)
    private val events = SessionEvents()
    private var now = 0L

    @Before
    fun start() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest) = MockResponse(body = "{}")
        }
        server.start()
    }

    @After
    fun stop() = server.close()

    private fun sign(keys: Ed25519Sign.KeyPair, nonce: ByteArray): String =
        Base64Url.encode(Ed25519Sign(keys.privateKey).sign(ServerProof.message(nonce)))

    /** A prover that signs like the real server with [keys] and counts its calls. */
    private inner class Signer(var keys: Ed25519Sign.KeyPair = serverKeys) : ServerProver {
        val calls = AtomicInteger()

        override fun prove(server: HttpUrl, nonce: ByteArray): ProofAnswer {
            calls.incrementAndGet()
            return ProofAnswer(sign(keys, nonce), Base64Url.encode(keys.publicKey))
        }
    }

    private fun trusted(url: HttpUrl = server.url("/"), key: String? = serverKey) = object : TrustedServer {
        override val server = url
        override val serverKey = key
    }

    private val credentials get() = object : SessionCredentials {
        override val server = this@ServerIdentityGateTest.server.url("/")
        override val bearerToken = "app-token"
        override val deviceId = "0123456789abcdef0123456789abcdef"
    }

    private fun gate(prover: ServerProver, trusted: TrustedServer = trusted(), timeout: Duration = Duration.ofSeconds(5)) =
        ServerIdentityGate(trusted, { generation.get() }, prover, events, timeout = timeout, backoff = Duration.ofSeconds(5), clock = { now })

    private fun client(gate: ServerIdentityGate): OkHttpClient = SlouchcatHttp.client(identity, credentials, gate = gate)

    private fun get(http: OkHttpClient, path: String = "/api/v1/auth/session") =
        http.newCall(Request.Builder().url(server.url(path)).build()).execute().use { it.code }

    private fun signIn(http: OkHttpClient) = http.newCall(
        Request.Builder().url(server.url("/api/v1/auth/app-login"))
            .post("""{"email":"a@example.test","password":"secret-password"}""".toRequestBody("application/json".toMediaType()))
            .build(),
    ).execute().use { it.code }

    private fun assertRefused(block: () -> Unit): IOException {
        try {
            block()
        } catch (e: IOException) {
            return e
        }
        fail("the gate let the request through")
        error("unreachable")
    }

    @Test
    fun aProvenServerGetsItsRequestsAndOneProofCoversTheNetwork() {
        val signer = Signer()
        val http = client(gate(signer))
        repeat(5) { assertEquals(200, get(http)) }
        assertEquals(1, signer.calls.get())
        assertEquals(5, server.requestCount)
        assertEquals("Bearer app-token", server.takeRequest().headers["Authorization"])
    }

    @Test
    fun aWrongSignatureSendsNothingNeitherTokenNorDeviceIdNorPassword() {
        val impostor = ServerProver { _, nonce -> ProofAnswer(sign(otherKeys, nonce), serverKey) }
        val http = client(gate(impostor))
        val refused = assertRefused { signIn(http) }
        assertTrue(refused is ServerNotProvenException)
        assertFalse(refused is ServerKeyChangedException)
        assertRefused { get(http) }
        assertEquals("nothing reached the server", 0, server.requestCount)
    }

    @Test
    fun noAnswerToTheProofIsNotReachable() {
        val http = client(gate({ _, _ -> throw IOException("no route") }))
        assertRefused { get(http) }
        val refusing = client(gate({ _, _ -> null }))
        assertRefused { get(refusing) }
        assertEquals(0, server.requestCount)
    }

    @Test
    fun aNewKeyIsReportedOnceAndSendsNothingUntilTheUserConfirmsIt() = runTest(UnconfinedTestDispatcher()) {
        val signer = Signer(otherKeys)
        val gate = gate(signer)
        val http = client(gate)
        val reported = async { events.events.first() }
        val refused = assertRefused { get(http) }
        assertTrue(refused is ServerKeyChangedException)
        assertEquals(otherKey, (refused as ServerKeyChangedException).newKey)
        assertEquals(otherKey, gate.presentedKey)
        assertEquals(SessionEvent.ServerKeyChanged, reported.await())
        assertEquals(0, server.requestCount)

        // While the user signs in again to confirm it, the gate checks against the new key.
        gate.candidateKey = otherKey
        assertEquals(200, signIn(http))
        assertEquals(1, server.requestCount)
    }

    @Test
    fun aNewNetworkNeedsANewProof() {
        val signer = Signer()
        val http = client(gate(signer))
        assertEquals(200, get(http))
        generation.incrementAndGet()
        assertEquals(200, get(http))
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
    }

    @Test
    fun concurrentRequestsShareOneProof() {
        val release = CountDownLatch(1)
        val calls = AtomicInteger()
        val slow = ServerProver { _, nonce ->
            calls.incrementAndGet()
            release.await(5, TimeUnit.SECONDS)
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val http = client(gate(slow))
        val pool = Executors.newFixedThreadPool(8)
        val results = Collections.synchronizedList(mutableListOf<Int>())
        val done = CountDownLatch(8)
        repeat(8) {
            pool.execute {
                results += runCatching { get(http) }.getOrDefault(-1)
                done.countDown()
            }
        }
        // Nothing passes while the proof runs.
        assertEquals(0, server.requestCount)
        release.countDown()
        assertTrue(done.await(10, TimeUnit.SECONDS))
        pool.shutdown()
        assertEquals(List(8) { 200 }, results)
        assertEquals(1, calls.get())
    }

    @Test
    fun aProofFromANetworkThatIsGoneCountsForNothing() {
        val nonces = mutableListOf<Long>()
        val calls = AtomicInteger()
        val changing = ServerProver { _, nonce ->
            nonces += generation.get()
            // The network changes while the first proof is on its way.
            if (calls.incrementAndGet() == 1) generation.incrementAndGet()
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val http = client(gate(changing))
        assertEquals(200, get(http))
        assertEquals(2, calls.get())
        assertEquals(listOf(1L, 2L), nonces)
        // The second proof holds for the new network.
        assertEquals(200, get(http))
        assertEquals(2, calls.get())
    }

    @Test
    fun aRequestWaitingForAProofGivesUpAfterTheTimeout() {
        val release = CountDownLatch(1)
        val started = CountDownLatch(1)
        val stuck = ServerProver { _, nonce ->
            started.countDown()
            release.await(10, TimeUnit.SECONDS)
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val http = client(gate(stuck, timeout = Duration.ofMillis(200)))
        val owner = Executors.newSingleThreadExecutor()
        val first = owner.submit<Int> { get(http) }
        assertTrue(started.await(5, TimeUnit.SECONDS))
        assertRefused { get(http) }
        assertEquals(0, server.requestCount)
        release.countDown()
        assertEquals(200, first.get(5, TimeUnit.SECONDS))
        owner.shutdown()
    }

    @Test
    fun afterAFailureTheGateAnswersAtOnceUntilTheBackoffIsOver() {
        val calls = AtomicInteger()
        var answer: (ByteArray) -> ProofAnswer? = { null }
        val prover = ServerProver { _, nonce -> calls.incrementAndGet(); answer(nonce) }
        val http = client(gate(prover))
        assertRefused { get(http) }
        assertRefused { get(http) }
        assertEquals(1, calls.get())
        now += Duration.ofSeconds(6).toNanos()
        answer = { nonce -> ProofAnswer(sign(serverKeys, nonce), serverKey) }
        assertEquals(200, get(http))
        assertEquals(2, calls.get())
    }

    @Test
    fun httpsAndOtherHostsNeedNoProof() {
        val never = ServerProver { _, _ -> error("no proof expected") }
        val https = client(gate(never, trusted(HttpUrl.Builder().scheme("https").host("media.example.test").build())))
        assertEquals(200, get(https))
        val other = client(gate(never, trusted(server.url("/").newBuilder().port(server.port + 1).build())))
        assertEquals(200, get(other))
    }

    @Test
    fun withoutARememberedKeyNothingIsSent() {
        val http = client(gate(Signer(), trusted(key = null)))
        assertRefused { get(http) }
        assertEquals(0, server.requestCount)
    }

    @Test
    fun theProofTravelsWithoutCredentialsAndTheRealServerPasses() {
        // The real exchange: proveServer over a bare client, then the request with the token.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath != "/api/v1/server/prove") return MockResponse(body = "{}")
                val body = Json.parseToJsonElement(request.body!!.utf8()).jsonObject
                val nonce = Base64Url.decode(body["nonce"]!!.jsonPrimitive.content)!!
                return MockResponse(
                    headers = okhttp3.Headers.headersOf("Content-Type", "application/json"),
                    body = """{"signature":"${sign(serverKeys, nonce)}","serverKey":"$serverKey"}""",
                )
            }
        }
        val bare = SlouchcatHttp.client(identity)
        val http = client(gate(ApiServerProver(bare)))
        assertEquals(200, signIn(http))
        val proof = server.takeRequest()
        assertEquals("/api/v1/server/prove", proof.url.encodedPath)
        assertNull(proof.headers["Authorization"])
        assertNull(proof.headers[SlouchcatHttp.HEADER_DEVICE])
        assertFalse(proof.body!!.utf8().contains("secret-password"))
        val login = server.takeRequest()
        assertEquals("/api/v1/auth/app-login", login.url.encodedPath)
        assertEquals("Bearer app-token", login.headers["Authorization"])
    }
}
