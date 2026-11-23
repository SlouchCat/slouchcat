package com.slouchcat.core.api

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.ServerProof
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.infrastructure.ServerException
import com.slouchcat.core.api.generated.models.ServerProofRequest
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.serialization.SerializationException
import okhttp3.Call
import okhttp3.ConnectionPool
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.MediaType
import okhttp3.Response
import okhttp3.ResponseBody
import okio.Buffer
import okio.BufferedSource
import okio.ForwardingSource
import okio.buffer
import java.io.IOException
import java.io.InterruptedIOException
import java.net.SocketTimeoutException
import java.time.Duration
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import java.util.concurrent.atomic.AtomicInteger

/** The one server the app is set up with, and the key it remembered at the setup (F-38). */
interface TrustedServer {
    val server: HttpUrl?

    /** `getHealth.serverKey` as the setup stored it (base64url); `null` before the setup. */
    val serverKey: String?
}

/** The network the device uses; [generation] changes with every change of the networks the device uses. */
fun interface NetworkState {
    fun generation(): Long
}

/** What `proveServer` answered. */
data class ProofAnswer(val signature: String, val serverKey: String)

/** Asks the server to sign a nonce; over a client without the gate and without credentials. */
fun interface ServerProver {
    /** The answer, `null` when the server refused (4xx, 5xx, unreadable); throws [IOException] without answer. */
    fun prove(server: HttpUrl, nonce: ByteArray): ProofAnswer?
}

/** `proveServer` through the generated client over [client] (which must not carry the gate or credentials). */
class ApiServerProver(private val client: Call.Factory) : ServerProver {
    override fun prove(server: HttpUrl, nonce: ByteArray): ProofAnswer? = try {
        val proof = SlouchcatApi.create(::SystemApi, server, client).proveServer(ServerProofRequest(Base64Url.encode(nonce)))
        proof?.let { ProofAnswer(it.signature, it.serverKey) }
    } catch (e: ClientException) {
        Log.i(TAG, "proof refused", "status" to e.statusCode)
        null
    } catch (e: ServerException) {
        Log.i(TAG, "proof failed", "status" to e.statusCode)
        null
    } catch (e: SerializationException) {
        Log.w(TAG, "proof unreadable", error = e)
        null
    } catch (e: UnsupportedOperationException) {
        Log.w(TAG, "proof answered unexpectedly", error = e)
        null
    }

    private companion object {
        const val TAG = "identity"
    }
}

/** The gate held a request back: the server did not prove itself in this network. */
open class ServerNotProvenException(message: String) : IOException(message)

/** The server proved a key other than the remembered one (a new secret, or another machine). */
class ServerKeyChangedException(val newKey: String) : ServerNotProvenException("the server presents a new key")

/**
 * «Server weist sich aus» (concept 27 section 6, F-38, ADR 0040): for a server over plain HTTP,
 * every request of the shared client to it (API, images, later player and background fetch) waits
 * until the server has signed a fresh nonce with the key remembered at the setup, once per network
 * ([NetworkState]) and at most [maxAge] long. Until then nothing leaves the device: no token, no
 * device id, no password. A wrong signature or no answer fails the request as «not reachable» (an
 * [IOException]). A valid signature under another key fails it as well and sets [newKey]; the app
 * shows that quietly and never offers to sign in by itself (ADR 0040, decision of the build).
 * Over HTTPS the gate does nothing.
 *
 * Two places in the shared client: as the first application interceptor it waits for the proof;
 * [hop], a network interceptor before the credentials, checks every hop to the server again (a
 * retry of OkHttp, a redirect). A hop without a proof fails when the network changed since the
 * request left; in the same network it proves again first, until the request's own connection
 * broke a second time.
 *
 * A connection that breaks forgets the proof, at the hop where it breaks: OkHttp's own retry of
 * that request is the next hop and waits for the new proof like every later request. That holds
 * while the answer is read as well (a segment of the player, a download). The idle connections go
 * with the proof. A request the app itself ended (a cancelled call, an interrupted
 * thread) forgets nothing, the network had no part in it. It puts the proof in doubt: new requests
 * renew a proof in doubt once it is older than [renewAfter], the requests on their way go on with
 * it meanwhile, and a renewal that fails takes it away.
 *
 * Concurrent requests share one proof; a proof that began before the network changed or before a
 * connection broke counts for nothing. After a failure the gate answers at once for [backoff] (the
 * server allows 30 proofs a minute per address). A proof the app aborted is no failure.
 */
class ServerIdentityGate(
    private val trusted: TrustedServer,
    private val network: NetworkState,
    private val prover: ServerProver,
    private val timeout: Duration = Duration.ofSeconds(15),
    private val backoff: Duration = Duration.ofSeconds(5),
    private val maxAge: Duration = Duration.ofMinutes(10),
    private val clock: () -> Long = System::nanoTime,
    private val renewAfter: Duration = Duration.ofMinutes(1),
) : Interceptor {
    private sealed interface Outcome {
        data object Proven : Outcome

        data class KeyChanged(val key: String) : Outcome

        data object Failed : Outcome

        /** No verdict on the server: an interruption or an Error ended the attempt. */
        data object Aborted : Outcome
    }

    /** A proof on its way; [breaks] is the number of broken connections when it began. */
    private class Attempt(val generation: Long, val key: String, val breaks: Long) {
        val result = CompletableFuture<Outcome>()
    }

    private class Proof(val generation: Long, val key: String, val at: Long) {
        /** The app ended a request since: it may have given up on a server that is no longer there. */
        var doubted = false
    }

    /**
     * What the gate remembers of one request on its way. A call runs its interceptors on one
     * thread; [noted] and [broken] are also written by whoever reads the answer, on a thread of
     * its own, when the request itself is over.
     */
    private class Trip {
        /** The network the request left in. */
        var generation = 0L

        /** How often a connection of this request broke. */
        @Volatile var broken = 0

        /** The error [hop] has taken note of, so that [intercept] does not count it again. */
        @Volatile var noted: IOException? = null
    }

    private val lock = Any()
    private var proven: Proof? = null
    private var inFlight: Attempt? = null
    private var failure: Triple<Long, String, Outcome>? = null
    private var failedAt = 0L

    /** Broken connections so far; a proof that began before the latest of them counts for nothing. */
    private var breaks = 0L

    /** Requests waiting for another request's proof (tests wait for them). */
    internal val waiting = AtomicInteger()

    /**
     * A new key the user is confirming by signing in again (section 6): while set, the gate checks
     * the proof against it instead of the remembered key. The caller suppresses the credentials meanwhile.
     */
    @Volatile
    var candidateKey: String? = null

    private val _newKey = MutableStateFlow<String?>(null)

    /**
     * The key another key-holder under the server's address proved, while the remembered one did
     * not; `null` once the remembered key proves itself again. The app shows «Server nicht
     * bestätigt» and offers the confirmation only on the user's own way (Mehr › Server).
     */
    val newKey: StateFlow<String?> = _newKey.asStateFlow()

    /** Forget every proof and the new key (a new server, a confirmed key); a proof on its way counts for nothing. */
    fun reset() {
        synchronized(lock) {
            proven = null
            failure = null
            breaks++
        }
        _newKey.value = null
    }

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val server = trusted.server
        if (server == null || server.isHttps || !SlouchcatHttp.sameOrigin(server, request.url)) return chain.proceed(request)
        val trip = chain.call().tag(Trip::class) { Trip() }
        trip.generation = awaitProof(server)
        return try {
            chain.proceed(request)
        } catch (e: IOException) {
            // What no hop saw: the connection never came about, or OkHttp ended the call.
            note(chain.call(), chain.connectionPool, e)
            throw e
        }
    }

    /**
     * The network half: every hop to the server over HTTP needs a proof of the current network,
     * and takes note of how it ended. Here and not only in [intercept], because OkHttp sends a
     * request again by itself when its connection broke, below [intercept]: that retry is the next
     * hop, and by then the proof has to be gone (security review M-1).
     */
    val hop: Interceptor = Interceptor { chain ->
        val request = chain.request()
        val server = trusted.server
        if (server == null || server.isHttps || !SlouchcatHttp.sameOrigin(server, request.url)) return@Interceptor chain.proceed(request)
        val call = chain.call()
        val pool = chain.connectionPool
        if (!isProvenNow()) proveOnTheWay(server, call.tag(Trip::class))
        val response = try {
            chain.proceed(request)
        } catch (e: IOException) {
            note(call, pool, e)
            throw e
        }
        // The hop is not over with the head of the answer: whoever reads the body may meet the break.
        response.newBuilder().body(Watched(response.body, call, pool)).build()
    }

    /**
     * The body of an answer, which is read after its hop returned. A connection that breaks in it
     * ends no request the gate could see: OkHttp does not retry, the reader (the player in a
     * segment, a download) asks again with a new request, and that one must not find the old proof.
     * A body without a length that is not chunked ends where its connection ends, so there a
     * connection closed too early looks like the end; the server sends no such answers.
     */
    private inner class Watched(private val body: ResponseBody, private val call: Call, private val pool: ConnectionPool) : ResponseBody() {
        /** One break per body: a reader that tries again meets the same broken connection, not a new one. */
        @Volatile private var broken = false

        private val source = object : ForwardingSource(body.source()) {
            override fun read(sink: Buffer, byteCount: Long): Long = try {
                super.read(sink, byteCount)
            } catch (e: IOException) {
                if (!broken) {
                    broken = true
                    note(call, pool, e)
                }
                throw e
            }
        }.buffer()

        override fun contentType(): MediaType? = body.contentType()

        override fun contentLength(): Long = body.contentLength()

        override fun source(): BufferedSource = source
    }

    /**
     * A hop without a proof. As a rule the request had one when it left ([intercept]) and it is gone: a
     * connection broke (the request's own, and this hop is OkHttp's retry; or another request's),
     * the proof ran out, or its renewal failed. After a network change the request fails; what
     * becomes of it in the new network is for the app to decide. In the network it left in, the hop
     * waits for a new proof, unless the request's own connection has broken twice: a machine that
     * signs and then breaks every connection costs a request one proof on its way and not one per
     * retry of OkHttp. A proof lost to the break of another request does not count against that:
     * when several connections from before break one after the other, a request loses the proof
     * to another's break first and to its own next.
     */
    private fun proveOnTheWay(server: HttpUrl, trip: Trip?) {
        // No trip: the request came to the server by a redirect from elsewhere and never waited for a proof.
        if (trip == null) throw ServerNotProvenException("no proof for a request that did not set out for the server")
        if (trip.generation != network.generation()) throw ServerNotProvenException("the network changed before the request left")
        if (trip.broken >= OWN_BREAKS) throw ServerNotProvenException("the connection broke again after a new proof")
        // The proof may be one of a newer network by now.
        if (awaitProof(server) != trip.generation) throw ServerNotProvenException("the network changed before the request left")
    }

    /**
     * Takes note of how a request ended, once per error. The app's own doing puts the proof in
     * doubt. Anything else is a connection that broke, and maybe the network went with it: the
     * proof is forgotten, and so are the idle connections, which are from before the break as well
     * (OkHttp's retry would take the next of them from the pool and break again; they go for every
     * host of the shared client, the pool has no narrower way).
     *
     * Not for a request that left in a network that is gone: after a network change its
     * connections break, often several at once, and that is no news about the network of now,
     * which has its own proof or gets one.
     */
    private fun note(call: Call, pool: ConnectionPool, e: IOException) {
        if (e is ServerNotProvenException) return
        val trip = call.tag(Trip::class)
        if (trip != null && trip.generation != network.generation()) return
        if (trip?.noted === e) return
        trip?.noted = e
        if (endedByTheApp(call, e)) {
            doubt()
        } else {
            if (trip != null) trip.broken++
            if (forget()) Log.i(TAG, "connection broke, proof forgotten", error = e)
            pool.evictAll()
        }
    }

    /**
     * Whether the app ended the request and not the network: the call was cancelled (a page left
     * cancels its images, the player a segment when it seeks), or its thread was interrupted (a
     * cancelled coroutine around a blocking call; once the call has its connection, Okio throws
     * [InterruptedIOException] and leaves the thread interrupted; while it still connects, OkHttp
     * lets an `InterruptedException` through, which never reaches the catch). A timeout is no such
     * thing: [SocketTimeoutException] is an [InterruptedIOException] as well and counts as a broken
     * connection, interrupted or not. And OkHttp ends a call that ran into its call timeout by
     * cancelling it: where a call has such a limit (the shared client sets none), a cancelled call
     * may be a server that stopped answering and counts as broken too.
     *
     * What this cannot tell apart: a page that was left, and an app that gave up on a server that
     * hangs (a coroutine timeout around a call interrupts the thread just the same). Both keep the
     * proof and put it in doubt (ADR 0040, addenda of 23 November 2026). A limit that should count
     * as a broken connection belongs to OkHttp (`readTimeout`, `callTimeout`), not around the call.
     */
    private fun endedByTheApp(call: Call, e: IOException): Boolean {
        val limit = call.timeout()
        val cancelled = call.isCanceled() && limit.timeoutNanos() == 0L && !limit.hasDeadline()
        return cancelled || interrupted(e)
    }

    /** Whether [e] is the report of an interrupted thread and not a timeout. */
    private fun interrupted(e: IOException): Boolean = e is InterruptedIOException && e !is SocketTimeoutException && Thread.currentThread().isInterrupted

    /** Whether the current network has a valid proof of the key in use. */
    fun isProvenNow(): Boolean = synchronized(lock) { validProof() != null }

    /** The proof of the key in use for the current network, under [lock]. */
    private fun validProof(): Proof? {
        val key = candidateKey ?: trusted.serverKey ?: return null
        return proven?.takeIf { it.generation == network.generation() && it.key == key && clock() - it.at < maxAge.toNanos() }
    }

    /** Forgets the proof after a broken connection; whether there was one. */
    private fun forget(): Boolean = synchronized(lock) {
        breaks++
        (proven != null).also { proven = null }
    }

    private fun doubt() = synchronized(lock) { proven?.doubted = true }

    /** Returns with a valid proof of the server, and the generation of the network it is for. */
    private fun awaitProof(server: HttpUrl): Long {
        // A proof counts for nothing when the network changed or a connection broke while it ran,
        // and it is no verdict when its owner was interrupted; a few rounds cover that.
        repeat(ROUNDS) {
            val generation = network.generation()
            val key = candidateKey ?: trusted.serverKey ?: throw ServerNotProvenException("no key remembered")
            var mine = false
            val attempt = synchronized(lock) {
                val proof = validProof()
                // A proof in doubt goes on counting for the requests on their way ([hop]); a new
                // request renews it once it is old enough.
                if (proof != null && !(proof.doubted && clock() - proof.at >= renewAfter.toNanos())) return proof.generation
                val failed = failure
                if (failed != null && failed.first == generation && failed.second == key && clock() - failedAt < backoff.toNanos()) {
                    throw failureOf(failed.third)
                }
                inFlight?.takeIf { it.generation == generation && it.key == key && it.breaks == breaks } ?: Attempt(generation, key, breaks).also {
                    inFlight = it
                    mine = true
                }
            }
            val outcome = if (mine) own(server, attempt) else wait(attempt) ?: throw ServerNotProvenException("the proof took too long")
            val counts = synchronized(lock) { network.generation() == generation && breaks == attempt.breaks }
            if (outcome == Outcome.Aborted || !counts) return@repeat
            when (outcome) {
                Outcome.Proven -> {
                    if (key == trusted.serverKey) _newKey.value = null
                    return generation
                }
                is Outcome.KeyChanged -> _newKey.value = outcome.key
                else -> Unit
            }
            throw failureOf(outcome)
        }
        throw ServerNotProvenException("the proof kept starting over")
    }

    /**
     * Runs [attempt] on the thread of the request that began it and records what it found, before
     * any request waiting for it goes on. What aborts the attempt is thrown on to that request.
     */
    private fun own(server: HttpUrl, attempt: Attempt): Outcome {
        var outcome: Outcome = Outcome.Aborted
        try {
            outcome = run(server, attempt.key)
            synchronized(lock) {
                // Not when the network changed or a connection broke meanwhile: the server signed before that.
                if (network.generation() == attempt.generation && breaks == attempt.breaks) {
                    if (outcome == Outcome.Proven) {
                        proven = Proof(attempt.generation, attempt.key, clock())
                        failure = null
                        Log.i(TAG, "server proven")
                    } else {
                        // Also a proof in doubt that was to be renewed: whoever answers now did not
                        // prove to be the server, and a foreign machine answers just like a server
                        // that is briefly unwell. Not the proof of another key (the remembered one,
                        // while a new key the user is confirming fails).
                        if (proven?.key == attempt.key) proven = null
                        failure = Triple(attempt.generation, attempt.key, outcome)
                        failedAt = clock()
                    }
                }
            }
        } finally {
            // Also after an Error: the next request starts a new proof, and nobody may wait for
            // one that never ends (security review L-5).
            synchronized(lock) { if (inFlight === attempt) inFlight = null }
            attempt.result.complete(outcome)
        }
        return outcome
    }

    private fun run(server: HttpUrl, key: String): Outcome {
        val outcome = try {
            val nonce = ServerProof.nonce()
            val answer = prover.prove(server, nonce)
            when {
                answer == null -> Outcome.Failed
                ServerProof.valid(key, nonce, answer.signature) -> Outcome.Proven
                answer.serverKey != key && ServerProof.valid(answer.serverKey, nonce, answer.signature) -> Outcome.KeyChanged(answer.serverKey)
                else -> Outcome.Failed
            }
        } catch (e: IOException) {
            // The app gave the request up while it proved (a cancelled coroutine interrupts the
            // thread): that says nothing about the server, so no failure and no backoff. While the
            // proof still connects, OkHttp lets an InterruptedException through instead; it passes
            // here as it is and counts the same.
            if (interrupted(e)) {
                Log.i(TAG, "proof aborted")
                throw e
            }
            Log.i(TAG, "no proof", error = e)
            Outcome.Failed
        } catch (e: RuntimeException) {
            Log.w(TAG, "proof broke", error = e)
            Outcome.Failed
        }
        if (outcome != Outcome.Proven) Log.w(TAG, "server not proven", "outcome" to outcome::class.simpleName)
        return outcome
    }

    /** The owner's outcome, or `null` after [timeout]; a waiter records no failure, only the owner does. */
    private fun wait(attempt: Attempt): Outcome? {
        waiting.incrementAndGet()
        return try {
            attempt.result.get(timeout.toMillis(), TimeUnit.MILLISECONDS)
        } catch (e: TimeoutException) {
            null
        } catch (e: ExecutionException) {
            // Never today: the owner completes the future with an outcome, also after an Error.
            Outcome.Failed
        } catch (e: InterruptedException) {
            // As Okio reports it, so that it counts as the app's own doing where a hop waited.
            Thread.currentThread().interrupt()
            throw InterruptedIOException("interrupted while the server proved itself")
        } finally {
            waiting.decrementAndGet()
        }
    }

    private fun failureOf(outcome: Outcome): IOException = when (outcome) {
        is Outcome.KeyChanged -> ServerKeyChangedException(outcome.key)
        else -> ServerNotProvenException("the server did not prove itself")
    }

    private companion object {
        const val TAG = "identity"
        const val ROUNDS = 3

        /** The break of its own connection that ends a request; before it, a hop proves again. */
        const val OWN_BREAKS = 2
    }
}
