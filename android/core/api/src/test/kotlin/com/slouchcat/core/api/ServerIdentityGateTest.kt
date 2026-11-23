package com.slouchcat.core.api

import com.google.crypto.tink.subtle.Ed25519Sign
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.ServerProof
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import mockwebserver3.Dispatcher
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import mockwebserver3.RecordedRequest
import mockwebserver3.SocketEffect
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import java.io.IOException
import java.io.InterruptedIOException
import java.net.InetAddress
import java.net.ServerSocket
import java.net.SocketException
import java.net.SocketTimeoutException
import java.time.Duration
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

/** «Server weist sich aus» (concept 27 section 6, F-38): the gate of the shared client. */
class ServerIdentityGateTest {
    private val server = MockWebServer()
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, "1.0.0")
    private val serverKeys = Ed25519Sign.KeyPair.newKeyPair()
    private val serverKey = Base64Url.encode(serverKeys.publicKey)
    private val otherKeys = Ed25519Sign.KeyPair.newKeyPair()
    private val otherKey = Base64Url.encode(otherKeys.publicKey)
    private val generation = AtomicLong(1)

    @Volatile private var now = 0L
    private val threads = Executors.newCachedThreadPool()

    @Before
    fun start() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest) = MockResponse(body = "{}")
        }
        server.start()
    }

    @After
    fun stop() {
        threads.shutdownNow()
        server.close()
    }

    private fun sign(keys: Ed25519Sign.KeyPair, nonce: ByteArray): String =
        Base64Url.encode(Ed25519Sign(keys.privateKey).sign(ServerProof.message(nonce)))

    /** A prover that signs like the real server with [keys] and counts its calls; each one goes to [log] as `proof`. */
    private inner class Signer(var keys: Ed25519Sign.KeyPair = serverKeys, private val log: MutableList<String>? = null) : ServerProver {
        val calls = AtomicInteger()

        override fun prove(server: HttpUrl, nonce: ByteArray): ProofAnswer {
            calls.incrementAndGet()
            log?.add("proof")
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
        ServerIdentityGate(trusted, { generation.get() }, prover, timeout = timeout, backoff = Duration.ofSeconds(5), maxAge = Duration.ofMinutes(10), clock = { now })

    private fun client(gate: ServerIdentityGate, builder: OkHttpClient.Builder = OkHttpClient.Builder()): OkHttpClient =
        SlouchcatHttp.client(identity, credentials, builder = builder, gate = gate)

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

    /** The error a call ended with, `null` when it got an answer. */
    private fun errorOf(call: Call): IOException? = try {
        call.execute().close()
        null
    } catch (e: IOException) {
        e
    }

    /**
     * Holds a request to [path] where a request is after it got its connection: past the gate's
     * wait for the proof, not yet at [ServerIdentityGate.hop] (the network interceptors of the
     * builder run before the ones of [SlouchcatHttp.client]).
     */
    private class Held(private val path: String) : Interceptor {
        val arrived = CountDownLatch(1)
        val release = CountDownLatch(1)

        override fun intercept(chain: Interceptor.Chain): Response {
            if (chain.request().url.encodedPath == path) {
                arrived.countDown()
                release.await(10, TimeUnit.SECONDS)
            }
            return chain.proceed(chain.request())
        }
    }

    /**
     * Interrupts [thread] once it waits for the answer in a socket read: Okio's look at the
     * interruption before the read is behind it, and the read itself ignores an interruption. So
     * the call ends with whatever the connection does next, on an interrupted thread.
     */
    private fun interruptInItsRead(thread: Thread) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (thread.stackTrace.none { it.className.startsWith("java.net.Socket") && it.methodName == "read" }) {
            if (System.nanoTime() > deadline) fail("the call never waited for its answer")
            Thread.sleep(5)
        }
        thread.interrupt()
    }

    /** Runs [call] on a thread of its own, which [caller] names: the error it ended with, and whether the thread was interrupted by then. */
    private fun errorOnItsOwnThread(call: Call, caller: AtomicReference<Thread>): Pair<IOException?, Boolean> {
        val own = Executors.newSingleThreadExecutor()
        try {
            return own.submit<Pair<IOException?, Boolean>> {
                caller.set(Thread.currentThread())
                val error = errorOf(call)
                error to Thread.interrupted()
            }.get(10, TimeUnit.SECONDS)
        } finally {
            own.shutdownNow()
        }
    }

    /** Runs [block] on a thread of its own, which [caller] names: what it ended with. The thread is left uninterrupted. */
    private fun <T> onItsOwnThread(caller: AtomicReference<Thread>? = null, block: () -> T): Future<Result<T>> = threads.submit<Result<T>> {
        caller?.set(Thread.currentThread())
        try {
            runCatching(block)
        } finally {
            Thread.interrupted()
        }
    }

    /**
     * Writes what reaches the server to [log] (path, connection, whether the token came along) and
     * answers with [answer]; `null` closes the connection without an answer, as a server does that
     * went away and as a machine does that never saw the connection.
     */
    private fun recording(log: MutableList<String>, answer: (RecordedRequest) -> MockResponse? = { MockResponse(body = "{}") }) = object : Dispatcher() {
        override fun dispatch(request: RecordedRequest): MockResponse {
            val token = if (request.headers["Authorization"] != null) "with" else "without"
            log += "${request.url.encodedPath} on ${request.connectionIndex} $token token"
            return answer(request) ?: MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
        }
    }

    /** Answers `proveServer` like the real server and counts the proofs that reached it. */
    private fun proving(proofs: AtomicInteger) = object : Dispatcher() {
        override fun dispatch(request: RecordedRequest): MockResponse {
            if (request.url.encodedPath != "/api/v1/server/prove") return MockResponse(body = "{}")
            proofs.incrementAndGet()
            val nonce = Base64Url.decode(Json.parseToJsonElement(request.body!!.utf8()).jsonObject["nonce"]!!.jsonPrimitive.content)!!
            return MockResponse(
                headers = okhttp3.Headers.headersOf("Content-Type", "application/json"),
                body = """{"signature":"${sign(serverKeys, nonce)}","serverKey":"$serverKey"}""",
            )
        }
    }

    /** Interrupts the thread of a call to `/interrupted` on its way, as a cancelled coroutine does it (ApiCaller). */
    private val interrupting = Interceptor { chain ->
        if (chain.request().url.encodedPath == "/interrupted") Thread.currentThread().interrupt()
        chain.proceed(chain.request())
    }

    /**
     * A minute passes while the app goes on using the server: the proof gets a minute older and
     * never lies unused for long (a proof no request used for a minute counts no more).
     */
    private fun aMinutePassesInUse(http: OkHttpClient) {
        now += Duration.ofSeconds(31).toNanos()
        assertEquals(200, get(http))
        now += Duration.ofSeconds(30).toNanos()
    }

    /** The app ends a call of [http], which carries [interrupting]. */
    private fun theAppEndsACall(http: OkHttpClient) {
        val error = onItsOwnThread { get(http, "/interrupted") }.get(5, TimeUnit.SECONDS).exceptionOrNull()
        assertTrue("not the interruption Okio reports: $error", error is InterruptedIOException && error !is SocketTimeoutException)
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
    fun anErrorInTheProofLeavesNoRequestWaiting() {
        val calls = AtomicInteger()
        val gate = gate({ _, _ -> calls.incrementAndGet(); throw NoClassDefFoundError("r8") }, timeout = Duration.ofSeconds(30))
        val http = client(gate)
        repeat(2) {
            val started = System.nanoTime()
            try {
                get(http)
                fail("an Error passed")
            } catch (e: NoClassDefFoundError) {
                // The owner sees the Error; no later request waits for this attempt.
            }
            assertTrue("a request waited for a dead proof", System.nanoTime() - started < TimeUnit.SECONDS.toNanos(10))
        }
        assertEquals(2, calls.get())
        assertEquals(0, server.requestCount)
    }

    @Test
    fun aNewKeyIsHeldQuietlyAndSendsNothingUntilTheUserConfirmsIt() {
        val signer = Signer(otherKeys)
        val gate = gate(signer)
        val http = client(gate)
        val refused = assertRefused { get(http) }
        assertTrue(refused is ServerKeyChangedException)
        assertEquals(otherKey, (refused as ServerKeyChangedException).newKey)
        assertEquals(otherKey, gate.newKey.value)
        assertEquals(0, server.requestCount)

        // Only the user's explicit confirmation makes the gate check against the new key.
        gate.candidateKey = otherKey
        assertEquals(200, signIn(http))
        assertEquals(1, server.requestCount)
    }

    @Test
    fun aNewKeyThatDoesNotProveItselfLeavesTheProofOfTheRememberedKey() {
        // The user opens the confirmation of a new key and gives it up: the server never signed
        // with that key, which says nothing against the proof of the remembered one.
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        gate.candidateKey = otherKey
        assertRefused { signIn(http) }
        assertEquals(2, signer.calls.get())
        gate.candidateKey = null
        assertTrue("the failed proof of another key cost the proof of the remembered one", gate.isProvenNow())
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
    }

    @Test
    fun theRememberedKeyProvingAgainClearsTheNewKey() {
        val signer = Signer(otherKeys)
        val gate = gate(signer)
        val http = client(gate)
        assertRefused { get(http) }
        assertEquals(otherKey, gate.newKey.value)
        // Back in the own network: the remembered key proves itself.
        signer.keys = serverKeys
        generation.incrementAndGet()
        assertEquals(200, get(http))
        assertNull(gate.newKey.value)
        gate.reset()
        assertNull(gate.newKey.value)
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
    fun aProofGetsOld() {
        val signer = Signer()
        val http = client(gate(signer))
        assertEquals(200, get(http))
        now += Duration.ofMinutes(11).toNanos()
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
    }

    @Test
    fun aProofUnusedForAMinuteCountsNoMore() {
        // A change of the network that the device cannot see mostly falls into a pause: the user
        // walks into another network. So after a minute without a request to the server the next
        // one proves first. Requests that follow each other keep the proof for its ten minutes.
        val log = Collections.synchronizedList(mutableListOf<String>())
        server.dispatcher = recording(log)
        val signer = Signer(log = log)
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        repeat(3) {
            now += Duration.ofSeconds(59).toNanos()
            assertEquals(200, get(http))
        }
        assertEquals("a proof in use was renewed", 1, signer.calls.get())
        now += Duration.ofSeconds(59).toNanos()
        assertTrue(gate.isProvenNow())
        now += Duration.ofSeconds(2).toNanos()
        assertFalse("the proof still counts after a minute without a request (a look at it is none)", gate.isProvenNow())
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
        assertEquals(listOf("proof", "/api/v1/auth/session on 0 with token"), log.takeLast(2))
    }

    @Test
    fun aForeignMachineAfterAPauseGetsNothing() {
        // What the pause is for. In aForeignMachineBehindABrokenConnectionGetsNoSecondRequest the
        // first request after the unseen change had left with the token; after a pause none does.
        val log = Collections.synchronizedList(mutableListOf<String>())
        server.dispatcher = recording(log)
        val foreign = AtomicBoolean()
        val prover = ServerProver { _, nonce ->
            log += "proof"
            ProofAnswer(sign(if (foreign.get()) otherKeys else serverKeys, nonce), serverKey)
        }
        val http = client(gate(prover))
        assertEquals(200, get(http))
        now += Duration.ofSeconds(61).toNanos()
        foreign.set(true)
        val refused = assertRefused { get(http) }
        assertTrue("$refused", refused is ServerNotProvenException)
        assertEquals(listOf("proof", "/api/v1/auth/session on 0 with token", "proof"), log.toList())
    }

    @Test
    fun aRequestOnItsWayWhenThePauseIsOverProvesBeforeItLeaves() {
        // The pause is measured where a request leaves, at its hop.
        val held = Held("/second")
        val signer = Signer()
        val http = client(gate(signer), OkHttpClient.Builder().addNetworkInterceptor(held))
        assertEquals(200, get(http))
        try {
            val second = onItsOwnThread { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            now += Duration.ofSeconds(61).toNanos()
            held.release.countDown()
            assertEquals(200, second.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals("the request left with a proof nobody had used for a minute", 2, signer.calls.get())
        } finally {
            held.release.countDown()
        }
    }

    @Test
    fun aBrokenConnectionForgetsTheProof() {
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        assertTrue(gate.isProvenNow())
        server.close()
        assertRefused { get(http) }
        assertFalse("the proof survived a broken connection", gate.isProvenNow())
    }

    /**
     * The app cancels a call that is on the server while a request to `/second` has its connection
     * and has not left yet; [afterwards] runs before that request goes on. Returns what that request
     * ended with.
     */
    private fun aRequestOnItsWayWhileACallIsCancelled(gate: ServerIdentityGate, afterwards: () -> Unit = {}): Result<Int> {
        val reached = CountDownLatch(1)
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath == "/cancelled") {
                    reached.countDown()
                    answer.await(10, TimeUnit.SECONDS)
                }
                return MockResponse(body = "{}")
            }
        }
        val held = Held("/second")
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(held))
        assertEquals(200, get(http))
        val pool = Executors.newFixedThreadPool(2)
        try {
            val second = pool.submit<Result<Int>> { runCatching { get(http, "/second") } }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            val call = http.newCall(Request.Builder().url(server.url("/cancelled")).build())
            val cancelled = pool.submit<IOException?> { errorOf(call) }
            assertTrue("the call to cancel is not on the server", reached.await(5, TimeUnit.SECONDS))
            call.cancel()
            assertTrue("the cancelled call got an answer", cancelled.get(5, TimeUnit.SECONDS) is IOException)
            afterwards()
            held.release.countDown()
            return second.get(5, TimeUnit.SECONDS)
        } finally {
            answer.countDown()
            held.release.countDown()
            pool.shutdownNow()
        }
    }

    @Test
    fun aCallTheAppCancelledKeepsTheProofForTheRequestsOnTheirWay() {
        // A page is left and Coil cancels its images, the player seeks and cancels a segment: the
        // app ended the call, the network did not. A request on its way meanwhile gets its answer.
        val signer = Signer()
        val gate = gate(signer)
        assertEquals(200, aRequestOnItsWayWhileACallIsCancelled(gate).getOrThrow())
        assertTrue("a call the app cancelled cost the proof", gate.isProvenNow())
        assertEquals(1, signer.calls.get())
    }

    @Test
    fun aNetworkChangeAfterACancelledCallStillStopsTheRequestOnItsWay() {
        // Keeping the proof after a cancelled call keeps it for this network only.
        val refused = aRequestOnItsWayWhileACallIsCancelled(gate(Signer())) { generation.incrementAndGet() }.exceptionOrNull()
        assertTrue("the request left into a network without a proof: $refused", refused is ServerNotProvenException)
        assertEquals(listOf("/api/v1/auth/session", "/cancelled"), List(server.requestCount) { server.takeRequest().url.encodedPath })
    }

    @Test
    fun aCallWhoseThreadWasInterruptedKeepsTheProofForTheRequestsOnTheirWay() {
        // A cancelled coroutine interrupts the thread of its blocking call (ApiCaller): the next
        // Okio operation ends the call with InterruptedIOException("interrupted") and leaves the
        // thread interrupted; the call itself is not cancelled.
        val interrupt = Interceptor { chain ->
            if (chain.request().url.encodedPath == "/interrupted") Thread.currentThread().interrupt()
            chain.proceed(chain.request())
        }
        val held = Held("/second")
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(held).addNetworkInterceptor(interrupt))
        assertEquals(200, get(http))
        val pool = Executors.newFixedThreadPool(2)
        try {
            val second = pool.submit<Int> { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            val call = http.newCall(Request.Builder().url(server.url("/interrupted")).build())
            val interrupted = pool.submit<IOException?> {
                try {
                    errorOf(call)
                } finally {
                    Thread.interrupted()
                }
            }
            val error = interrupted.get(5, TimeUnit.SECONDS)
            assertTrue("not the interruption Okio reports: $error", error is InterruptedIOException && error !is SocketTimeoutException)
            assertEquals("interrupted", error?.message)
            assertFalse(call.isCanceled())
            held.release.countDown()
            assertEquals(200, second.get(5, TimeUnit.SECONDS))
            assertTrue("an interrupted call cost the proof", gate.isProvenNow())
            assertEquals(1, signer.calls.get())
        } finally {
            held.release.countDown()
            pool.shutdownNow()
        }
    }

    @Test
    fun aProofInDoubtIsRenewedOnceItIsAMinuteOld() {
        // After a call the app ended the proof stays, the requests on their way need it. But it is
        // in doubt: the app may have given up on a server that hangs since the network changed
        // unseen. New requests prove again once such a proof is a minute old, never on every abort.
        val signer = Signer()
        val http = client(gate(signer), OkHttpClient.Builder().addNetworkInterceptor(interrupting))
        assertEquals(200, get(http))
        aMinutePassesInUse(http)
        assertEquals(200, get(http))
        assertEquals("a proof nobody doubts was renewed before its ten minutes", 1, signer.calls.get())

        theAppEndsACall(http)
        assertEquals(200, get(http))
        assertEquals("a proof in doubt, a minute old, was not renewed", 2, signer.calls.get())

        repeat(3) {
            theAppEndsACall(http)
            assertEquals(200, get(http))
        }
        now += Duration.ofSeconds(30).toNanos()
        theAppEndsACall(http)
        assertEquals(200, get(http))
        assertEquals("a proof younger than a minute was renewed", 2, signer.calls.get())

        now += Duration.ofSeconds(31).toNanos()
        assertEquals(200, get(http))
        assertEquals(3, signer.calls.get())
        assertEquals(200, get(http))
        assertEquals("one doubt renewed the proof twice", 3, signer.calls.get())
    }

    @Test
    fun aCancelledCallPutsTheProofInDoubtToo() {
        val signer = Signer()
        val gate = gate(signer)
        assertEquals(200, aRequestOnItsWayWhileACallIsCancelled(gate).getOrThrow())
        val http = client(gate)
        aMinutePassesInUse(http)
        assertEquals("a proof younger than a minute was renewed", 1, signer.calls.get())
        assertEquals(200, get(http))
        assertEquals("a proof in doubt, a minute old, was not renewed", 2, signer.calls.get())
    }

    @Test
    fun whileAProofInDoubtIsRenewedTheRequestsOnTheirWayPass() {
        val renewing = CountDownLatch(1)
        val release = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) {
                renewing.countDown()
                release.await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val held = Held("/second")
        val http = client(gate(prover), OkHttpClient.Builder().addNetworkInterceptor(held).addNetworkInterceptor(interrupting))
        assertEquals(200, get(http))
        theAppEndsACall(http)
        try {
            val second = onItsOwnThread { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            aMinutePassesInUse(http)
            val third = onItsOwnThread { get(http) }
            assertTrue("the proof in doubt is not renewed", renewing.await(5, TimeUnit.SECONDS))
            held.release.countDown()
            assertEquals(200, second.get(5, TimeUnit.SECONDS).getOrThrow())
            assertFalse("the new request did not wait for the renewal", third.isDone)
            release.countDown()
            assertEquals(200, third.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(2, calls.get())
        } finally {
            held.release.countDown()
            release.countDown()
        }
    }

    @Test
    fun aRenewalThatFailsClearsTheProof() {
        // What the doubt is for: the machine under the server's address is another one by now.
        val log = Collections.synchronizedList(mutableListOf<String>())
        server.dispatcher = recording(log)
        val foreign = AtomicBoolean()
        val prover = ServerProver { _, nonce -> ProofAnswer(sign(if (foreign.get()) otherKeys else serverKeys, nonce), serverKey) }
        val held = Held("/second")
        val gate = gate(prover)
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(held).addNetworkInterceptor(interrupting))
        assertEquals(200, get(http))
        theAppEndsACall(http)
        try {
            val second = onItsOwnThread { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            aMinutePassesInUse(http)
            foreign.set(true)
            val refused = assertRefused { get(http, "/third") }
            assertTrue("$refused", refused is ServerNotProvenException)
            assertFalse("the proof survived a renewal that failed", gate.isProvenNow())
            held.release.countDown()
            val onItsWay = second.get(5, TimeUnit.SECONDS).exceptionOrNull()
            assertTrue("the request on its way left without a proof: $onItsWay", onItsWay is ServerNotProvenException)
            assertTrue("a request left without a proof: $log", log.none { it.startsWith("/second") || it.startsWith("/third") })
        } finally {
            held.release.countDown()
        }
    }

    @Test
    fun aRenewalTheAppAbortedKeepsTheProofAndIsNoFailure() {
        // The renewal runs right when further aborts are likely: the user is leaving pages.
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) {
                Thread.currentThread().interrupt()
                throw InterruptedIOException("interrupted")
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val gate = gate(prover)
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(interrupting))
        assertEquals(200, get(http))
        theAppEndsACall(http)
        aMinutePassesInUse(http)
        val aborted = onItsOwnThread { get(http) }.get(5, TimeUnit.SECONDS).exceptionOrNull()
        assertTrue("not the interruption of the renewal: $aborted", aborted is InterruptedIOException && aborted !is SocketTimeoutException)
        assertEquals(2, calls.get())
        assertTrue("an aborted renewal cost the proof", gate.isProvenNow())
        assertEquals(200, get(http))
        assertEquals("the next request did not renew the proof in doubt", 3, calls.get())
    }

    @Test
    fun aSocketTimeoutForgetsTheProof() {
        // SocketTimeoutException is an InterruptedIOException too, but nobody interrupted anything:
        // the server stopped answering, which is what a network that went away looks like.
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath == "/silent") answer.await(10, TimeUnit.SECONDS)
                return MockResponse(body = "{}")
            }
        }
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        try {
            assertEquals(200, get(http))
            val impatient = http.newBuilder().readTimeout(Duration.ofMillis(200)).build()
            val refused = assertRefused { get(impatient, "/silent") }
            assertTrue("not a timeout: $refused", refused is SocketTimeoutException)
            assertFalse("the proof survived a timeout", gate.isProvenNow())
        } finally {
            answer.countDown()
        }
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
    }

    @Test
    fun aCallTimeoutForgetsTheProof() {
        // OkHttp ends a call that ran into its call timeout by cancelling it. That is a server that
        // stopped answering and not a call the app gave up.
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath == "/silent") answer.await(10, TimeUnit.SECONDS)
                return MockResponse(body = "{}")
            }
        }
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        try {
            assertEquals(200, get(http))
            val call = http.newBuilder().callTimeout(Duration.ofMillis(200)).build()
                .newCall(Request.Builder().url(server.url("/silent")).build())
            assertTrue("the call got an answer", errorOf(call) is IOException)
            assertTrue("OkHttp no longer cancels a call at its call timeout", call.isCanceled())
            assertFalse("the proof survived a call timeout", gate.isProvenNow())
        } finally {
            answer.countDown()
        }
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
    }

    @Test
    fun aSocketTimeoutOfAnInterruptedCallForgetsTheProof() {
        // The interruption does not explain a server that stopped answering.
        val caller = AtomicReference<Thread>()
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath == "/silent") {
                    interruptInItsRead(caller.get())
                    answer.await(10, TimeUnit.SECONDS)
                }
                return MockResponse(body = "{}")
            }
        }
        val gate = gate(Signer())
        val http = client(gate)
        try {
            assertEquals(200, get(http))
            // Long enough for the interruption to land first, also on a busy runner.
            val call = http.newBuilder().readTimeout(Duration.ofSeconds(3)).build()
                .newCall(Request.Builder().url(server.url("/silent")).build())
            val (error, interrupted) = errorOnItsOwnThread(call, caller)
            assertTrue("the thread was not interrupted when the call ended", interrupted)
            assertTrue("not a timeout: $error", error is SocketTimeoutException)
            assertFalse("the proof survived a timeout of an interrupted call", gate.isProvenNow())
        } finally {
            answer.countDown()
        }
    }

    @Test
    fun aConnectionResetUnderAnInterruptedCallForgetsTheProof() {
        // The interruption does not explain a reset either: that is how a machine answers a
        // connection it does not know. (A connection closed in good order is different: on an
        // interrupted thread Okio reads once more and reports the interruption.)
        val caller = AtomicReference<Thread>()
        ServerSocket(0, 1, InetAddress.getLoopbackAddress()).use { listener ->
            val resetting = Executors.newSingleThreadExecutor()
            try {
                resetting.execute {
                    listener.accept().use { socket ->
                        val head = StringBuilder()
                        while (!head.endsWith("\r\n\r\n")) {
                            val byte = socket.getInputStream().read()
                            if (byte < 0) return@use
                            head.append(byte.toChar())
                        }
                        interruptInItsRead(caller.get())
                        socket.setSoLinger(true, 0)
                    }
                }
                val url = HttpUrl.Builder().scheme("http").host(listener.inetAddress.hostAddress!!).port(listener.localPort).build()
                val signer = Signer()
                val gate = gate(signer, trusted(url))
                // Without OkHttp's own retry: it would connect again on the interrupted thread, and
                // that ends with an InterruptedException which no interceptor sees.
                val http = client(gate, OkHttpClient.Builder().retryOnConnectionFailure(false))
                val (error, interrupted) = errorOnItsOwnThread(http.newCall(Request.Builder().url(url).build()), caller)
                assertTrue("the thread was not interrupted when the call ended", interrupted)
                assertTrue("not a reset: $error", error is SocketException)
                assertEquals(1, signer.calls.get())
                assertFalse("the proof survived a reset under an interrupted call", gate.isProvenNow())
            } finally {
                resetting.shutdownNow()
            }
        }
    }

    @Test
    fun aServerThatHangsUpForgetsTheProof() {
        val hangups = AtomicInteger()
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/hangup") {
                hangups.incrementAndGet()
                MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            } else {
                MockResponse(body = "{}")
            }
        }
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        // OkHttp sends the request again by itself. That hop gets one new proof (the server is
        // there and signs); when its connection breaks as well, the request ends.
        val refused = assertRefused { get(http, "/hangup") }
        // What the app sees: the third hop (OkHttp retries once more) is held back, «not reachable».
        assertTrue("$refused", refused is ServerNotProvenException)
        assertEquals("one new proof for a request on its way, no second", 2, signer.calls.get())
        assertEquals("the request left once after its new proof, and not a third time", 2, hangups.get())
        assertFalse("the proof survived a connection the server closed", gate.isProvenNow())
        assertEquals(200, get(http))
        assertEquals(3, signer.calls.get())
    }

    @Test
    fun aRetryOfOkHttpAfterABrokenConnectionProvesAgainBeforeItLeaves() {
        // A connection from the pool breaks and OkHttp sends the request again on a new one, below
        // the application interceptor (M-1). The break may be another machine under the server's
        // address, so the retry waits for a new proof. The real server signs, the user sees nothing.
        val log = Collections.synchronizedList(mutableListOf<String>())
        val dropped = AtomicInteger()
        server.dispatcher = recording(log) { request ->
            if (request.url.encodedPath == "/dropped" && dropped.getAndIncrement() == 0) null else MockResponse(body = "{}")
        }
        val gate = gate(Signer(log = log))
        val http = client(gate)
        assertEquals(200, get(http))
        assertEquals(200, get(http, "/dropped"))
        assertEquals(
            listOf("proof", "/api/v1/auth/session on 0 with token", "/dropped on 0 with token", "proof", "/dropped on 1 with token"),
            log.toList(),
        )
        assertTrue(gate.isProvenNow())
    }

    @Test
    fun aForeignMachineBehindABrokenConnectionGetsNoSecondRequest() {
        // The case M-1 is for: another network with the same addresses, which the device did not
        // see coming. The machine there resets the old connection, accepts a new one and cannot
        // sign. The request on the old connection had left with the token, and whoever holds the
        // address can read it (the log below shows it): the gate cannot take that back. What it
        // stops is everything after the break.
        val log = Collections.synchronizedList(mutableListOf<String>())
        server.dispatcher = recording(log) { request -> if (request.url.encodedPath == "/dropped") null else MockResponse(body = "{}") }
        val foreign = AtomicBoolean()
        val prover = ServerProver { _, nonce ->
            log += "proof"
            ProofAnswer(sign(if (foreign.get()) otherKeys else serverKeys, nonce), serverKey)
        }
        val gate = gate(prover)
        val http = client(gate)
        assertEquals(200, get(http))
        foreign.set(true)
        val refused = assertRefused { get(http, "/dropped") }
        assertTrue("$refused", refused is ServerNotProvenException)
        assertEquals(
            "the retry left without a new proof",
            listOf("proof", "/api/v1/auth/session on 0 with token", "/dropped on 0 with token", "proof"),
            log.toList(),
        )
        assertFalse(gate.isProvenNow())
    }

    @Test
    fun aBrokenConnectionTakesTheIdleConnectionsWithIt() {
        // The server went away and came back (an update), or the network is another one: every
        // connection from before is dead, not only the one that showed it. OkHttp's retry would
        // take the next of them from the pool and break again, after the one new proof of the request.
        val log = Collections.synchronizedList(mutableListOf<String>())
        val together = CountDownLatch(3)
        val release = CountDownLatch(1)
        val gone = AtomicInteger(-1)
        server.dispatcher = recording(log) { request ->
            if (request.url.encodedPath == "/together") {
                together.countDown()
                release.await(10, TimeUnit.SECONDS)
            }
            if (request.connectionIndex <= gone.get()) null else MockResponse(body = "{}")
        }
        val signer = Signer()
        val http = client(gate(signer))
        try {
            val three = List(3) { onItsOwnThread { get(http, "/together") } }
            assertTrue("no three connections at once", together.await(5, TimeUnit.SECONDS))
            release.countDown()
            three.forEach { assertEquals(200, it.get(5, TimeUnit.SECONDS).getOrThrow()) }
        } finally {
            release.countDown()
        }
        assertEquals(3, http.connectionPool.idleConnectionCount())
        assertEquals(1, signer.calls.get())
        gone.set(2)
        log.clear()
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
        assertEquals("a second connection from before was tried: $log", 2, log.size)
        assertEquals("/api/v1/auth/session on 3 with token", log.last())
    }

    @Test
    fun aRequestOnItsWayOutlivesTheBrokenConnectionOfAnother() {
        // A break forgets the proof for every request. One that had its connection by then and had
        // not left yet proves again at its hop and goes on; it used to fail there.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/dropped") {
                MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            } else {
                MockResponse(body = "{}")
            }
        }
        val held = Held("/second")
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(held))
        assertEquals(200, get(http))
        try {
            val second = onItsOwnThread { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            // Without OkHttp's retry the break ends the request, and the gate proves for nobody yet.
            val broken = assertRefused { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/dropped") }
            assertFalse("the gate held the request back: $broken", broken is ServerNotProvenException)
            assertFalse(gate.isProvenNow())
            assertEquals(1, signer.calls.get())
            held.release.countDown()
            assertEquals(200, second.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(2, signer.calls.get())
        } finally {
            held.release.countDown()
        }
    }

    @Test
    fun aProofThatFinishesAfterABreakCountsForNothing() {
        // The same seen from the request that owns the proof on its way, when nobody else proves
        // meanwhile: the answer it gets is from before the break, so it asks again before it goes
        // on its way.
        val reached = CountDownLatch(1)
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath != "/slow") return MockResponse(body = "{}")
                reached.countDown()
                answer.await(10, TimeUnit.SECONDS)
                return MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            }
        }
        val began = CountDownLatch(1)
        val finish = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) {
                began.countDown()
                finish.await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val held = Held("/second")
        val http = client(gate(prover), OkHttpClient.Builder().addNetworkInterceptor(held))
        assertEquals(200, get(http))
        try {
            val broken = onItsOwnThread { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/slow") }
            assertTrue(reached.await(5, TimeUnit.SECONDS))
            now += Duration.ofMinutes(11).toNanos()
            val owner = onItsOwnThread { get(http, "/second") }
            assertTrue("no proof on its way", began.await(5, TimeUnit.SECONDS))
            answer.countDown()
            val error = broken.get(5, TimeUnit.SECONDS).exceptionOrNull()
            assertTrue("no broken connection: $error", error is IOException && error !is ServerNotProvenException)
            finish.countDown()
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            assertEquals("the request went on its way with the proof from before the break", 3, calls.get())
            held.release.countDown()
            assertEquals(200, owner.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(3, calls.get())
        } finally {
            answer.countDown()
            finish.countDown()
            held.release.countDown()
        }
    }

    @Test
    fun aBrokenConnectionIsCountedOnce() {
        // The hop sees the break first, the application interceptor sees the same error again when
        // OkHttp does not retry. A request that proved anew in between keeps its proof: it is from
        // after the break.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/dropped") {
                MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            } else {
                MockResponse(body = "{}")
            }
        }
        val shared = AtomicReference<OkHttpClient>()
        val between = Interceptor { chain ->
            try {
                chain.proceed(chain.request())
            } catch (e: IOException) {
                if (chain.request().url.encodedPath == "/dropped") assertEquals(200, onItsOwnThread { get(shared.get()) }.get(5, TimeUnit.SECONDS).getOrThrow())
                throw e
            }
        }
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(between))
        shared.set(http)
        assertEquals(200, get(http))
        val broken = assertRefused { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/dropped") }
        assertFalse("the gate held the request back: $broken", broken is ServerNotProvenException)
        assertEquals(2, signer.calls.get())
        assertTrue("one break cost two proofs", gate.isProvenNow())
    }

    /**
     * A request leaves, the network changes and proves itself anew, and [later] the connection of
     * that request breaks. Returns the gate, the client and how many proofs the server gave by then.
     */
    private fun aRequestOfTheNetworkBeforeBreaks(later: Duration): Triple<ServerIdentityGate, OkHttpClient, Signer> {
        val reached = CountDownLatch(1)
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath != "/slow") return MockResponse(body = "{}")
                reached.countDown()
                answer.await(10, TimeUnit.SECONDS)
                return MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            }
        }
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        try {
            val old = onItsOwnThread { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/slow") }
            assertTrue(reached.await(5, TimeUnit.SECONDS))
            generation.incrementAndGet()
            assertEquals(200, get(http))
            assertEquals(2, signer.calls.get())
            now += later.toNanos()
            answer.countDown()
            val broken = old.get(5, TimeUnit.SECONDS).exceptionOrNull()
            assertTrue("no broken connection: $broken", broken is IOException && broken !is ServerNotProvenException)
        } finally {
            answer.countDown()
        }
        return Triple(gate, http, signer)
    }

    @Test
    fun aRequestOfAnotherNetworkThatBreaksSaysNothingAboutTheNetworkOfNow() {
        // After a network change the connections of the old network break, often several at once.
        // The new network has its own proof by then, and those breaks are no news about it.
        val (gate, http, signer) = aRequestOfTheNetworkBeforeBreaks(later = Duration.ofSeconds(29))
        assertTrue("a break in the old network cost the proof of the new one", gate.isProvenNow())
        assertEquals(1, http.connectionPool.idleConnectionCount())
        assertEquals(200, get(http))
        assertEquals(2, signer.calls.get())
    }

    @Test
    fun aConnectionThatOutlivedANetworkChangeAndBreaksLaterIsNewsAgain() {
        // Not every change of the networks takes the connections with it (mobile data comes while
        // the Wi-Fi stays). A connection that was still there half a minute later and breaks then
        // breaks for a reason of its own.
        val (gate, http, signer) = aRequestOfTheNetworkBeforeBreaks(later = Duration.ofSeconds(31))
        assertFalse("the proof survived a break long after the network change", gate.isProvenNow())
        assertEquals(0, http.connectionPool.idleConnectionCount())
        assertEquals(200, get(http))
        assertEquals(3, signer.calls.get())
    }

    @Test
    fun breaksThatKeepComingWhileTheServerProvesFailNoRequest() {
        // The server came back from an update and a page loads in waves: connections from before
        // break one after the other, each while a proof runs. Every such proof counts for nothing,
        // and none of them says anything against the server.
        val breaksAtATime = 4
        val reached = CountDownLatch(breaksAtATime)
        val answers = List(breaksAtATime) { CountDownLatch(1) }
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                val slow = request.url.encodedPath.removePrefix("/slow/").toIntOrNull() ?: return MockResponse(body = "{}")
                reached.countDown()
                answers[slow].await(10, TimeUnit.SECONDS)
                return MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            }
        }
        val began = List(breaksAtATime) { CountDownLatch(1) }
        val finish = List(breaksAtATime) { CountDownLatch(1) }
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            val call = calls.incrementAndGet() - 2
            if (call in 0 until breaksAtATime) {
                began[call].countDown()
                finish[call].await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        // The rounds end with the gate's timeout; a slow machine must not get there.
        val http = client(gate(prover, timeout = Duration.ofSeconds(30)))
        assertEquals(200, get(http))
        try {
            val unretried = http.newBuilder().retryOnConnectionFailure(false).build()
            val old = List(breaksAtATime) { onItsOwnThread { get(unretried, "/slow/$it") } }
            assertTrue(reached.await(5, TimeUnit.SECONDS))
            now += Duration.ofMinutes(11).toNanos()
            val request = onItsOwnThread { get(http) }
            repeat(breaksAtATime) {
                assertTrue("no proof on its way in round $it", began[it].await(5, TimeUnit.SECONDS))
                answers[it].countDown()
                assertTrue(old[it].get(5, TimeUnit.SECONDS).exceptionOrNull() is IOException)
                finish[it].countDown()
            }
            assertEquals(200, request.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(breaksAtATime + 2, calls.get())
        } finally {
            answers.forEach { it.countDown() }
            finish.forEach { it.countDown() }
        }
    }

    @Test
    fun breaksThatKeepComingEndARequestWhenItsTimeIsUp() {
        // No request waits for ever for a proof that every break overtakes: the time a request
        // waits for the proof of another is also the time it gives its own rounds.
        val reached = CountDownLatch(1)
        val answer = CountDownLatch(1)
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath != "/slow") return MockResponse(body = "{}")
                reached.countDown()
                answer.await(10, TimeUnit.SECONDS)
                return MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            }
        }
        val began = CountDownLatch(1)
        val finish = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) {
                began.countDown()
                finish.await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val http = client(gate(prover, timeout = Duration.ofMillis(200)))
        assertEquals(200, get(http))
        try {
            val old = onItsOwnThread { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/slow") }
            assertTrue(reached.await(5, TimeUnit.SECONDS))
            now += Duration.ofMinutes(11).toNanos()
            val request = onItsOwnThread { get(http) }
            assertTrue("no proof on its way", began.await(5, TimeUnit.SECONDS))
            answer.countDown()
            assertTrue(old.get(5, TimeUnit.SECONDS).exceptionOrNull() is IOException)
            Thread.sleep(300)
            finish.countDown()
            val refused = request.get(5, TimeUnit.SECONDS).exceptionOrNull()
            assertTrue("$refused", refused is ServerNotProvenException)
            assertEquals("the proof ran out of time", refused?.message)
            assertEquals("a round began after the time was up", 2, calls.get())
        } finally {
            answer.countDown()
            finish.countDown()
        }
    }

    @Test
    fun manyConnectionsThatBreakAtOnceFailNoRequest() {
        // The server went away and came back while the app held five connections, and a page asks
        // for five things at once. Every break counts for itself (a proof from before any of them
        // is worth nothing), so this may cost a proof per break, but no request fails.
        val log = Collections.synchronizedList(mutableListOf<String>())
        val together = CountDownLatch(5)
        val release = CountDownLatch(1)
        val gone = AtomicInteger(-1)
        server.dispatcher = recording(log) { request ->
            if (request.url.encodedPath == "/together") {
                together.countDown()
                release.await(10, TimeUnit.SECONDS)
            }
            if (request.connectionIndex <= gone.get()) null else MockResponse(body = "{}")
        }
        val signer = Signer()
        val http = client(gate(signer))
        try {
            val five = List(5) { onItsOwnThread { get(http, "/together") } }
            assertTrue("no five connections at once", together.await(5, TimeUnit.SECONDS))
            release.countDown()
            five.forEach { assertEquals(200, it.get(5, TimeUnit.SECONDS).getOrThrow()) }
        } finally {
            release.countDown()
        }
        assertEquals(5, http.connectionPool.idleConnectionCount())
        gone.set(4)
        val start = CountDownLatch(1)
        val again = List(5) {
            onItsOwnThread {
                start.await(5, TimeUnit.SECONDS)
                get(http)
            }
        }
        start.countDown()
        again.forEach { assertEquals(200, it.get(10, TimeUnit.SECONDS).getOrThrow()) }
        assertTrue("more proofs than broken connections: ${signer.calls.get()}", signer.calls.get() in 2..6)
    }

    @Test
    fun aProofThatBeganBeforeAResetCountsForNothing() {
        // «Server wechseln» while a proof is on its way: that proof is of the server before.
        val began = CountDownLatch(1)
        val finish = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 1) {
                began.countDown()
                finish.await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val gate = gate(prover)
        val http = client(gate)
        try {
            val first = onItsOwnThread { get(http) }
            assertTrue(began.await(5, TimeUnit.SECONDS))
            gate.reset()
            finish.countDown()
            assertEquals(200, first.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals("the proof from before the reset counted", 2, calls.get())
        } finally {
            finish.countDown()
        }
    }

    @Test
    fun aNetworkChangeWhileAHopProvesAgainStopsTheRequest() {
        // The new proof of a hop is for the network the request left in, or for none.
        val log = Collections.synchronizedList(mutableListOf<String>())
        server.dispatcher = recording(log) { request -> if (request.url.encodedPath == "/dropped") null else MockResponse(body = "{}") }
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) generation.incrementAndGet()
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val http = client(gate(prover))
        assertEquals(200, get(http))
        val refused = assertRefused { get(http, "/dropped") }
        assertTrue("$refused", refused is ServerNotProvenException)
        assertEquals("the server signed in both networks", 3, calls.get())
        assertEquals(listOf("/api/v1/auth/session on 0 with token", "/dropped on 0 with token"), log.toList())
    }

    @Test
    fun aHopInterruptedWhileItWaitsForAProofIsNoBrokenConnection() {
        // A request on its way waits at its hop for the proof of another and its coroutine is
        // cancelled: the app ended it. That must not cost the other its proof.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/dropped") {
                MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            } else {
                MockResponse(body = "{}")
            }
        }
        val began = CountDownLatch(1)
        val finish = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) {
                began.countDown()
                finish.await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val held = Held("/second")
        val gate = gate(prover)
        val http = client(gate, OkHttpClient.Builder().addNetworkInterceptor(held))
        assertEquals(200, get(http))
        try {
            val thread = AtomicReference<Thread>()
            val second = onItsOwnThread(thread) { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            assertRefused { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/dropped") }
            val third = onItsOwnThread { get(http) }
            assertTrue("no proof on its way", began.await(5, TimeUnit.SECONDS))
            held.release.countDown()
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
            while (gate.waiting.get() < 1 && System.nanoTime() < deadline) Thread.onSpinWait()
            assertEquals("the hop does not wait for the proof on its way", 1, gate.waiting.get())
            thread.get().interrupt()
            val interrupted = second.get(5, TimeUnit.SECONDS).exceptionOrNull()
            assertTrue("$interrupted", interrupted is InterruptedIOException && interrupted !is SocketTimeoutException)
            finish.countDown()
            assertEquals(200, third.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals("the interrupted hop counted as a broken connection", 2, calls.get())
        } finally {
            held.release.countDown()
            finish.countDown()
        }
    }

    @Test
    fun aRequestThatLostTheProofBeforeItLeftSurvivesItsOwnBrokenConnectionToo() {
        // Several connections from before break one after the other (the server came back from an
        // update): a request loses the proof to the break of another before it leaves, proves
        // again, and then finds its own connection dead. That is its first own break, not its second.
        val log = Collections.synchronizedList(mutableListOf<String>())
        val seconds = AtomicInteger()
        server.dispatcher = recording(log) { request ->
            when (request.url.encodedPath) {
                "/dropped" -> null
                "/second" -> if (seconds.getAndIncrement() == 0) null else MockResponse(body = "{}")
                else -> MockResponse(body = "{}")
            }
        }
        val held = Held("/second")
        val signer = Signer(log = log)
        val http = client(gate(signer), OkHttpClient.Builder().addNetworkInterceptor(held))
        assertEquals(200, get(http))
        try {
            val second = onItsOwnThread { get(http, "/second") }
            assertTrue(held.arrived.await(5, TimeUnit.SECONDS))
            assertRefused { get(http.newBuilder().retryOnConnectionFailure(false).build(), "/dropped") }
            held.release.countDown()
            assertEquals(200, second.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(
                listOf(
                    "proof", "/api/v1/auth/session on 0 with token", "/dropped on 1 with token",
                    "proof", "/second on 0 with token", "proof", "/second on 2 with token",
                ),
                log.toList(),
            )
        } finally {
            held.release.countDown()
        }
    }

    @Test
    fun aProofThatBeganBeforeABreakCountsForNothing() {
        // A proof is on its way (the old one ran out) when a connection breaks. Whatever that proof
        // ends with, the server signed before the break: the request whose connection broke asks again.
        val reached = CountDownLatch(1)
        val answer = CountDownLatch(1)
        val slow = AtomicInteger()
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse {
                if (request.url.encodedPath != "/slow" || slow.getAndIncrement() > 0) return MockResponse(body = "{}")
                reached.countDown()
                answer.await(10, TimeUnit.SECONDS)
                return MockResponse.Builder().onResponseStart(SocketEffect.CloseSocket()).build()
            }
        }
        val began = CountDownLatch(1)
        val finish = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 2) {
                began.countDown()
                finish.await(10, TimeUnit.SECONDS)
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        // A request that waited for the proof on its way would wait longer than this test does.
        val http = client(gate(prover, timeout = Duration.ofSeconds(30)))
        assertEquals(200, get(http))
        try {
            val broken = onItsOwnThread { get(http, "/slow") }
            assertTrue(reached.await(5, TimeUnit.SECONDS))
            now += Duration.ofMinutes(11).toNanos()
            val other = onItsOwnThread { get(http) }
            assertTrue("no proof on its way", began.await(5, TimeUnit.SECONDS))
            answer.countDown()
            assertEquals("the request waited for the proof from before its break", 200, broken.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(3, calls.get())
            finish.countDown()
            assertEquals(200, other.get(5, TimeUnit.SECONDS).getOrThrow())
            assertEquals(3, calls.get())
        } finally {
            answer.countDown()
            finish.countDown()
        }
    }

    @Test
    fun aConnectionThatBreaksWhileTheAnswerIsReadForgetsTheProof() {
        // The player in a segment, a download in its file: the answer began and the connection
        // breaks in its body. OkHttp does not retry that; whoever reads asks again with a new request.
        val log = Collections.synchronizedList(mutableListOf<String>())
        server.dispatcher = recording(log) { request ->
            if (request.url.encodedPath == "/cut") {
                MockResponse.Builder().body("x".repeat(256 * 1024)).onResponseBody(SocketEffect.CloseSocket()).build()
            } else {
                MockResponse(body = "{}")
            }
        }
        val signer = Signer(log = log)
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        http.newCall(Request.Builder().url(server.url("/cut")).build()).execute().use { answer ->
            assertEquals(200, answer.code)
            val body = answer.body.source()
            val cut = assertRefused { body.readByteString() }
            assertFalse("the gate held the request back: $cut", cut is ServerNotProvenException)
            assertFalse("the proof survived a connection that broke in the answer", gate.isProvenNow())
            assertEquals(200, get(http))
            assertEquals(
                listOf("proof", "/api/v1/auth/session on 0 with token", "/cut on 0 with token", "proof", "/api/v1/auth/session on 1 with token"),
                log.toList(),
            )
            // A reader that tries the broken answer again meets the same break, not a new one.
            assertRefused { body.readByteString() }
            assertTrue("one break in an answer cost two proofs", gate.isProvenNow())
        }
    }

    @Test
    fun anAnswerTheAppStopsReadingKeepsTheProofAndPutsItInDoubt() {
        // The player seeks and cancels the segment it is reading, a page is left while its image
        // loads: the app ended the answer, the network did not.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/long") {
                MockResponse.Builder().body("x".repeat(256 * 1024)).throttleBody(64 * 1024, 2, TimeUnit.SECONDS).build()
            } else {
                MockResponse(body = "{}")
            }
        }
        val signer = Signer()
        val gate = gate(signer)
        val http = client(gate)
        assertEquals(200, get(http))
        val call = http.newCall(Request.Builder().url(server.url("/long")).build())
        call.execute().use { answer ->
            assertEquals(200, answer.code)
            val reader = AtomicReference<Thread>()
            val reading = onItsOwnThread(reader) { answer.body.string() }
            // In the middle of the answer: the reader waits in the socket for the part the server holds back.
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
            while (reader.get()?.stackTrace?.any { it.className.startsWith("java.net.Socket") && it.methodName == "read" } != true) {
                if (System.nanoTime() > deadline) fail("the answer was never read")
                Thread.sleep(5)
            }
            call.cancel()
            assertTrue("the cancelled answer was read to its end", reading.get(5, TimeUnit.SECONDS).exceptionOrNull() is IOException)
        }
        assertTrue("an answer the app stopped reading cost the proof", gate.isProvenNow())
        assertEquals(1, signer.calls.get())
        aMinutePassesInUse(http)
        assertEquals("a proof younger than a minute was renewed", 1, signer.calls.get())
        assertEquals(200, get(http))
        assertEquals("a proof in doubt, a minute old, was not renewed", 2, signer.calls.get())
    }

    @Test
    fun aHopAfterANetworkChangeFailsBeforeTheCredentialsAreAdded() {
        // The network changes while the first hop is on the server: OkHttp follows the redirect
        // (as it would retry) below the application interceptor; the hop check stops it.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/first") {
                generation.incrementAndGet()
                MockResponse(code = 302, headers = okhttp3.Headers.headersOf("Location", "/second"))
            } else {
                MockResponse(body = "{}")
            }
        }
        val http = client(gate(Signer()))
        val refused = assertRefused { get(http, "/first") }
        assertTrue(refused is ServerNotProvenException)
        assertEquals("the redirect did not leave", 1, server.requestCount)
    }

    @Test
    fun aHopAfterANetworkChangeAsksForNoProof() {
        // The request fails at once, without waiting for a proof it cannot use; the next request
        // proves in the new network.
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = if (request.url.encodedPath == "/first") {
                generation.incrementAndGet()
                MockResponse(code = 302, headers = okhttp3.Headers.headersOf("Location", "/second"))
            } else {
                MockResponse(body = "{}")
            }
        }
        val signer = Signer()
        val http = client(gate(signer))
        assertRefused { get(http, "/first") }
        assertEquals(1, signer.calls.get())
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
        val gate = gate(slow)
        val http = client(gate)
        val pool = Executors.newFixedThreadPool(8)
        val results = Collections.synchronizedList(mutableListOf<Int>())
        val done = CountDownLatch(8)
        repeat(8) {
            pool.execute {
                results += runCatching { get(http) }.getOrDefault(-1)
                done.countDown()
            }
        }
        // Wait until one owns the proof and seven wait for it; nothing has passed meanwhile.
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (gate.waiting.get() < 7 && System.nanoTime() < deadline) Thread.onSpinWait()
        assertEquals(7, gate.waiting.get())
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
        assertEquals(200, get(http))
        assertEquals(2, calls.get())
    }

    @Test
    fun aWaiterThatGivesUpRecordsNoFailure() {
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
        // The owner's proof counts: the next request passes at once.
        assertEquals(200, get(http))
        owner.shutdown()
    }

    @Test
    fun aProofTheAppAbortedIsNoFailure() {
        // The first request of a network proves on its own thread. Its coroutine is cancelled
        // meanwhile (ApiCaller interrupts the thread), and Okio ends the proof with an
        // InterruptedIOException. That says nothing about the server.
        val proofs = AtomicInteger()
        server.dispatcher = proving(proofs)
        val http = client(gate(ApiServerProver(SlouchcatHttp.client(identity))))
        assertEquals(200, get(http))
        generation.incrementAndGet()
        val owner = onItsOwnThread {
            Thread.currentThread().interrupt()
            get(http)
        }.get(5, TimeUnit.SECONDS).exceptionOrNull()
        assertTrue("not the interruption Okio reports: $owner", owner is InterruptedIOException && owner !is SocketTimeoutException)
        // No failure and no backoff: the next request asks the server and passes.
        val asked = proofs.get()
        assertEquals(200, get(http))
        assertEquals(asked + 1, proofs.get())
    }

    /**
     * A request waits for the proof of another, whose thread is interrupted meanwhile; [abort] is
     * what the prover does with that. The waiter must pass with a proof of its own; returns what
     * the interrupted request ended with.
     */
    private fun theOwnerOfAProofIsInterruptedWhileAnotherWaits(abort: (InterruptedException) -> Nothing): Throwable? {
        val began = CountDownLatch(1)
        val calls = AtomicInteger()
        val prover = ServerProver { _, nonce ->
            if (calls.incrementAndGet() == 1) {
                began.countDown()
                try {
                    CountDownLatch(1).await(10, TimeUnit.SECONDS)
                } catch (e: InterruptedException) {
                    abort(e)
                }
            }
            ProofAnswer(sign(serverKeys, nonce), serverKey)
        }
        val gate = gate(prover)
        val http = client(gate)
        val thread = AtomicReference<Thread>()
        val owner = onItsOwnThread(thread) { get(http) }
        assertTrue(began.await(5, TimeUnit.SECONDS))
        val waiter = onItsOwnThread { get(http) }
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (gate.waiting.get() < 1 && System.nanoTime() < deadline) Thread.onSpinWait()
        assertEquals(1, gate.waiting.get())
        thread.get().interrupt()
        assertEquals(200, waiter.get(5, TimeUnit.SECONDS).getOrThrow())
        assertEquals(2, calls.get())
        return owner.get(5, TimeUnit.SECONDS).exceptionOrNull()
    }

    @Test
    fun waitersOfAProofInterruptedWhileItConnectedStartANewRound() {
        // While the proof still connects, OkHttp lets the InterruptedException through as it is.
        val owner = theOwnerOfAProofIsInterruptedWhileAnotherWaits { throw it }
        assertTrue("$owner", owner is InterruptedException)
    }

    @Test
    fun waitersOfAProofInterruptedOnItsConnectionStartANewRound() {
        // Once the proof has its connection, Okio reports the interruption and leaves the thread interrupted.
        val owner = theOwnerOfAProofIsInterruptedWhileAnotherWaits {
            Thread.currentThread().interrupt()
            throw InterruptedIOException("interrupted")
        }
        assertTrue("$owner", owner is InterruptedIOException)
    }

    @Test
    fun aProofThatTimedOutOnAnInterruptedThreadIsAFailure() {
        // The interruption does not explain a server that does not answer the proof.
        val calls = AtomicInteger()
        val http = client(
            gate({ _, _ ->
                calls.incrementAndGet()
                Thread.currentThread().interrupt()
                throw SocketTimeoutException("timeout")
            }),
        )
        val owner = onItsOwnThread { get(http) }.get(5, TimeUnit.SECONDS).exceptionOrNull()
        assertTrue("$owner", owner is ServerNotProvenException)
        try {
            assertRefused { get(http) }
        } finally {
            Thread.interrupted()
        }
        assertEquals("the backoff did not hold", 1, calls.get())
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
