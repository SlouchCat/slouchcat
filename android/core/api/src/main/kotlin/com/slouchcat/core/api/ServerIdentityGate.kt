package com.slouchcat.core.api

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.ServerProof
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.infrastructure.ClientException
import com.slouchcat.core.api.generated.infrastructure.ServerException
import com.slouchcat.core.api.generated.models.ServerProofRequest
import com.slouchcat.core.api.log.Log
import kotlinx.serialization.SerializationException
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.Response
import java.io.IOException
import java.time.Duration
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException

/** The one server the app is set up with, and the key it remembered at the setup (F-38). */
interface TrustedServer {
    val server: HttpUrl?

    /** `getHealth.serverKey` as the setup stored it (base64url); `null` before the setup. */
    val serverKey: String?
}

/** The network the device uses; [generation] changes with every change of the default network. */
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
 * until the server has signed a fresh nonce with the key remembered at the setup, once per
 * network ([NetworkState]). Until then nothing leaves the device: no token, no device id, no
 * password. A wrong signature or no answer fails the request as «not reachable» (an [IOException]);
 * a valid signature under another key reports [SessionEvent.ServerKeyChanged] once per network.
 * Over HTTPS the gate does nothing.
 *
 * Install it as the first application interceptor; the credentials are added later, per network
 * hop. Concurrent requests share one proof; a proof that finishes after the network changed counts
 * for nothing and is made again. After a failure the gate answers at once for [backoff] instead of
 * asking again (the server allows 30 proofs a minute per address).
 */
class ServerIdentityGate(
    private val trusted: TrustedServer,
    private val network: NetworkState,
    private val prover: ServerProver,
    private val events: SessionEvents,
    private val timeout: Duration = Duration.ofSeconds(15),
    private val backoff: Duration = Duration.ofSeconds(5),
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

    private val lock = Any()
    private var proven: Pair<Long, String>? = null
    private var inFlight: Attempt? = null
    private var failure: Triple<Long, String, Outcome>? = null
    private var failedAt = 0L
    private var reportedChange: Pair<Long, String>? = null

    /**
     * A new key the user is confirming by signing in again (concept 27 section 6): while set, the
     * gate checks the proof against it instead of the remembered key. The caller suppresses the
     * credentials meanwhile; only the new sign-in travels.
     */
    @Volatile
    var candidateKey: String? = null

    /** The key the server presented when it last failed with a new key. */
    @Volatile
    var presentedKey: String? = null
        private set

    /** Forget every proof (a new server, a confirmed key). */
    fun reset() = synchronized(lock) {
        proven = null
        failure = null
        reportedChange = null
    }

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val server = trusted.server
        if (server != null && !server.isHttps && SlouchcatHttp.sameOrigin(server, request.url)) {
            awaitProof(server)
        }
        return chain.proceed(request)
    }

    private fun awaitProof(server: HttpUrl) {
        // A proof that finished in a network that is gone by now counts for nothing; a few rounds
        // cover a network that changes while the proof runs.
        repeat(ROUNDS) {
            val generation = network.generation()
            val key = candidateKey ?: trusted.serverKey ?: throw ServerNotProvenException("no key remembered")
            var mine = false
            val attempt = synchronized(lock) {
                if (proven == Pair(generation, key)) return
                val failed = failure
                if (failed != null && failed.first == generation && failed.second == key && clock() - failedAt < backoff.toNanos()) {
                    throw failureOf(failed.third)
                }
                inFlight?.takeIf { it.generation == generation && it.key == key } ?: Attempt(generation, key).also {
                    inFlight = it
                    mine = true
                }
            }
            val outcome = if (mine) run(server, attempt) else wait(attempt)
            synchronized(lock) {
                if (inFlight === attempt) inFlight = null
                if (network.generation() != generation) return@repeat
                when (outcome) {
                    Outcome.Proven -> {
                        proven = Pair(generation, key)
                        failure = null
                        return
                    }
                    else -> {
                        failure = Triple(generation, key, outcome)
                        failedAt = clock()
                    }
                }
            }
            if (outcome is Outcome.KeyChanged) report(generation, outcome.key)
            throw failureOf(outcome)
        }
        throw ServerNotProvenException("the network kept changing")
    }

    private fun run(server: HttpUrl, attempt: Attempt): Outcome {
        val outcome = try {
            val nonce = ServerProof.nonce()
            val answer = prover.prove(server, nonce)
            when {
                answer == null -> Outcome.Failed
                ServerProof.valid(attempt.key, nonce, answer.signature) -> Outcome.Proven
                answer.serverKey != attempt.key && ServerProof.valid(answer.serverKey, nonce, answer.signature) -> Outcome.KeyChanged(answer.serverKey)
                else -> Outcome.Failed
            }
        } catch (e: IOException) {
            Log.i(TAG, "no proof", error = e)
            Outcome.Failed
        } catch (e: RuntimeException) {
            Log.w(TAG, "proof broke", error = e)
            Outcome.Failed
        }
        if (outcome != Outcome.Proven) Log.w(TAG, "server not proven", "outcome" to outcome::class.simpleName)
        attempt.result.complete(outcome)
        return outcome
    }

    private fun wait(attempt: Attempt): Outcome = try {
        attempt.result.get(timeout.toMillis(), TimeUnit.MILLISECONDS)
    } catch (e: TimeoutException) {
        Outcome.Failed
    } catch (e: ExecutionException) {
        Outcome.Failed
    } catch (e: InterruptedException) {
        Thread.currentThread().interrupt()
        throw IOException("interrupted while the server proved itself")
    }

    private fun report(generation: Long, key: String) {
        presentedKey = key
        val first = synchronized(lock) {
            (reportedChange != Pair(generation, key)).also { reportedChange = Pair(generation, key) }
        }
        if (first) events.report(SessionEvent.ServerKeyChanged)
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
