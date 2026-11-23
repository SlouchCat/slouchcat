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
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.Response
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
 * [hop], a network interceptor before the credentials, fails every hop to the server (a retry of
 * OkHttp, a redirect) when the network changed after the proof. A request that loses its
 * connection forgets the proof; a request the app itself ended (a cancelled call, an interrupted
 * thread) does not, the network had no part in it. Concurrent requests share one proof; a proof
 * that finishes after the network changed counts for nothing. After a failure the gate answers at
 * once for [backoff] (the server allows 30 proofs a minute per address).
 */
class ServerIdentityGate(
    private val trusted: TrustedServer,
    private val network: NetworkState,
    private val prover: ServerProver,
    private val timeout: Duration = Duration.ofSeconds(15),
    private val backoff: Duration = Duration.ofSeconds(5),
    private val maxAge: Duration = Duration.ofMinutes(10),
    private val clock: () -> Long = System::nanoTime,
) : Interceptor {
    private sealed interface Outcome {
        data object Proven : Outcome

        data class KeyChanged(val key: String) : Outcome

        data object Failed : Outcome
    }

    private class Attempt(val generation: Long, val key: String) {
        val result = CompletableFuture<Outcome>()
    }

    private data class Proof(val generation: Long, val key: String, val at: Long)

    private val lock = Any()
    private var proven: Proof? = null
    private var inFlight: Attempt? = null
    private var failure: Triple<Long, String, Outcome>? = null
    private var failedAt = 0L

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

    /** Forget every proof and the new key (a new server, a confirmed key). */
    fun reset() {
        synchronized(lock) {
            proven = null
            failure = null
        }
        _newKey.value = null
    }

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val server = trusted.server
        if (server == null || server.isHttps || !SlouchcatHttp.sameOrigin(server, request.url)) return chain.proceed(request)
        awaitProof(server)
        return try {
            chain.proceed(request)
        } catch (e: IOException) {
            // The connection broke: maybe the network went with it. The next request proves again.
            // Not when the app ended the request itself: forgetting then fails the requests on
            // their way to [hop], in a network that never changed.
            if (e !is ServerNotProvenException && !endedByTheApp(chain.call(), e)) forget()
            throw e
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
     * proof (ADR 0040, addendum of 23 November 2026). A limit that should count as a broken
     * connection belongs to OkHttp (`readTimeout`, `callTimeout`), not around the call.
     */
    private fun endedByTheApp(call: Call, e: IOException): Boolean {
        val limit = call.timeout()
        val cancelled = call.isCanceled() && limit.timeoutNanos() == 0L && !limit.hasDeadline()
        return cancelled || (e is InterruptedIOException && e !is SocketTimeoutException && Thread.currentThread().isInterrupted)
    }

    /** The network half: every hop to the server over HTTP needs a proof of the current network. */
    val hop: Interceptor = Interceptor { chain ->
        val request = chain.request()
        val server = trusted.server
        if (server != null && !server.isHttps && SlouchcatHttp.sameOrigin(server, request.url) && !isProvenNow()) {
            throw ServerNotProvenException("the network changed before the request left")
        }
        chain.proceed(request)
    }

    /** Whether the current network has a valid proof of the key in use. */
    fun isProvenNow(): Boolean = synchronized(lock) {
        val key = candidateKey ?: trusted.serverKey ?: return false
        val proof = proven ?: return false
        proof.generation == network.generation() && proof.key == key && clock() - proof.at < maxAge.toNanos()
    }

    private fun forget() = synchronized(lock) { proven = null }

    private fun awaitProof(server: HttpUrl) {
        // A proof that finished in a network that is gone by now counts for nothing; a few rounds
        // cover a network that changes while the proof runs.
        repeat(ROUNDS) {
            val generation = network.generation()
            val key = candidateKey ?: trusted.serverKey ?: throw ServerNotProvenException("no key remembered")
            var mine = false
            val attempt = synchronized(lock) {
                if (isProvenNow()) return
                val failed = failure
                if (failed != null && failed.first == generation && failed.second == key && clock() - failedAt < backoff.toNanos()) {
                    throw failureOf(failed.third)
                }
                inFlight?.takeIf { it.generation == generation && it.key == key } ?: Attempt(generation, key).also {
                    inFlight = it
                    mine = true
                }
            }
            val outcome = try {
                if (mine) run(server, attempt) else wait(attempt) ?: throw ServerNotProvenException("the proof took too long")
            } finally {
                // Also when the proof threw an Error: the next request starts a new one.
                if (mine) synchronized(lock) { if (inFlight === attempt) inFlight = null }
            }
            synchronized(lock) {
                if (network.generation() != generation) return@repeat
                if (mine) {
                    if (outcome == Outcome.Proven) {
                        proven = Proof(generation, key, clock())
                        failure = null
                    } else {
                        failure = Triple(generation, key, outcome)
                        failedAt = clock()
                    }
                }
            }
            when (outcome) {
                Outcome.Proven -> {
                    if (key == trusted.serverKey) _newKey.value = null
                    return
                }
                is Outcome.KeyChanged -> _newKey.value = outcome.key
                Outcome.Failed -> Unit
            }
            throw failureOf(outcome)
        }
        throw ServerNotProvenException("the network kept changing")
    }

    private fun run(server: HttpUrl, attempt: Attempt): Outcome {
        var outcome: Outcome = Outcome.Failed
        try {
            val nonce = ServerProof.nonce()
            val answer = prover.prove(server, nonce)
            outcome = when {
                answer == null -> Outcome.Failed
                ServerProof.valid(attempt.key, nonce, answer.signature) -> Outcome.Proven
                answer.serverKey != attempt.key && ServerProof.valid(answer.serverKey, nonce, answer.signature) -> Outcome.KeyChanged(answer.serverKey)
                else -> Outcome.Failed
            }
        } catch (e: IOException) {
            Log.i(TAG, "no proof", error = e)
        } catch (e: RuntimeException) {
            Log.w(TAG, "proof broke", error = e)
        } finally {
            // Also after an Error: nobody may wait for a proof that never ends (security review L-5).
            attempt.result.complete(outcome)
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
            Outcome.Failed
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            throw IOException("interrupted while the server proved itself")
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
    }
}
