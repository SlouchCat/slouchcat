package com.slouchcat.core.api

import com.slouchcat.core.api.log.Log
import okhttp3.ConnectionPool
import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Response
import java.io.IOException
import java.time.Duration
import java.util.concurrent.TimeUnit

/** Which app sends a request, as the server reads it from `X-Slouchcat-Client` (`mobile/1.0.0`). */
data class ClientIdentity(val kind: Kind, val version: String) {
    enum class Kind(val header: String) { MOBILE("mobile"), TV("tv") }

    val header: String get() = "${kind.header}/$version"
}

/**
 * What the shared client sends along for the one server the app is set up with, filled from the
 * token store after the sign-in (core/session, `SessionManager`); [None] sends nothing.
 *
 * The values go only to [server] (same scheme, host and port): a redirect or an address of another
 * host never sees the token or the device id.
 */
interface SessionCredentials {
    val server: HttpUrl?
    val bearerToken: String?
    val deviceId: String?

    object None : SessionCredentials {
        override val server: HttpUrl? = null
        override val bearerToken: String? = null
        override val deviceId: String? = null
    }
}

/**
 * The one OkHttp client of an app (concept 27, 5.2): API, images and player go through it, so the
 * headers and the rules for plain HTTP live in one place.
 *
 * - `X-Slouchcat-Client` on every request (the server can turn an app away that is too old, 426).
 * - `X-Requested-With: Slouchcat` on every request that changes something (the CSRF header of the
 *   server for everything but GET, HEAD and OPTIONS).
 * - `Authorization: Bearer` and `X-Slouchcat-Device` from [SessionCredentials], only to its server:
 *   a network interceptor decides per hop, so a redirect to another host, port or scheme never
 *   carries them, and redirects from https to http are not followed at all.
 * - Plain HTTP is allowed (E-18); on Android the network security config of the app permits it.
 * - [gate] first: «Server weist sich aus» holds every request to a server over HTTP back until the
 *   server proved itself in this network ([ServerIdentityGate], F-38); its `hop` checks every
 *   network hop again (retries, redirects), before the credentials are added.
 * - With [network], in front of the gate: a read that a network change ended on its way goes once
 *   more, in the new network ([NetworkChangeRetry]).
 * - Idle connections go after [KEEP_ALIVE]: before the server closes them, and not before a proof
 *   of the server has lapsed for want of use ([PROOF_IDLE]). The client gets a pool of its own for
 *   that; a pool on the builder that is passed in is replaced.
 * - No logging interceptor: headers and addresses carry the token and stream tokens (rule 10).
 */
object SlouchcatHttp {
    const val HEADER_CLIENT = "X-Slouchcat-Client"
    const val HEADER_DEVICE = "X-Slouchcat-Device"
    const val HEADER_REQUESTED_WITH = "X-Requested-With"
    const val REQUESTED_WITH = "Slouchcat"

    val CONNECT_TIMEOUT: Duration = Duration.ofSeconds(10)
    val READ_TIMEOUT: Duration = Duration.ofSeconds(30)
    val WRITE_TIMEOUT: Duration = Duration.ofSeconds(30)

    /** How long a proof of the server counts without a request to the server ([ServerIdentityGate]). */
    val PROOF_IDLE: Duration = Duration.ofSeconds(60)

    /**
     * How long an idle connection stays in the pool. Shorter than the server keeps it (two
     * minutes, `IdleTimeout` in `server/cmd/slouchcat/main.go`), so that a pause does not end at a
     * connection the server closed. Longer than [PROOF_IDLE], so that the app still holds its old
     * connections for as long as a proof counts without a new one: where the network changed
     * unseen, it is their break that tells.
     */
    val KEEP_ALIVE: Duration = Duration.ofSeconds(90)
    private const val IDLE_CONNECTIONS = 5

    fun client(
        identity: ClientIdentity,
        credentials: SessionCredentials = SessionCredentials.None,
        builder: OkHttpClient.Builder = OkHttpClient.Builder(),
        gate: ServerIdentityGate? = null,
        network: NetworkState? = null,
        keepAlive: Duration = KEEP_ALIVE,
    ): OkHttpClient = builder
        .connectTimeout(CONNECT_TIMEOUT)
        .readTimeout(READ_TIMEOUT)
        .writeTimeout(WRITE_TIMEOUT)
        .followSslRedirects(false)
        .connectionPool(ConnectionPool(IDLE_CONNECTIONS, keepAlive.toMillis(), TimeUnit.MILLISECONDS))
        .apply { if (network != null) addInterceptor(NetworkChangeRetry(network)) }
        .apply { if (gate != null) addInterceptor(gate) }
        .addInterceptor(Headers(identity))
        .apply { if (gate != null) addNetworkInterceptor(gate.hop) }
        .addNetworkInterceptor(Credentials(credentials))
        .build()

    /**
     * A read that failed while the network changed goes once more, in the new network (ADR 0040,
     * addendum of 24 November 2026). The gate stops a hop when the network changed since the
     * request left, and the connections of the old network break: either way the request says
     * nothing about the server, and a page would show «not reachable» while the new network
     * reaches it. In front of the gate, so that the second try waits for the proof of the new
     * network like any new request; the gate's rules stay as they are.
     *
     * Only GET, HEAD and OPTIONS: a write may have reached the server before its connection broke,
     * and only the page knows whether it may send it again. Not when the app ended the call
     * (cancelled, interrupted), not for another key under the server's address, and not when the
     * network stayed the same: then the failure is news about the server.
     */
    class NetworkChangeRetry(private val network: NetworkState) : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request()
            val left = network.generation()
            return try {
                chain.proceed(request)
            } catch (e: IOException) {
                val again = request.method in SAFE_METHODS && e !is ServerKeyChangedException &&
                    !chain.call().isCanceled() && !Thread.currentThread().isInterrupted && network.generation() != left
                if (!again) throw e
                Log.i(TAG, "network changed on the way, asking again", error = e)
                try {
                    chain.proceed(request)
                } catch (second: IOException) {
                    second.addSuppressed(e)
                    throw second
                }
            }
        }

        private companion object {
            const val TAG = "network"
        }
    }

    /** The headers every request carries, whatever host it reaches. */
    class Headers(private val identity: ClientIdentity) : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request()
            val builder = request.newBuilder().header(HEADER_CLIENT, identity.header)
            if (request.method !in SAFE_METHODS) builder.header(HEADER_REQUESTED_WITH, REQUESTED_WITH)
            return chain.proceed(builder.build())
        }
    }

    /**
     * Bearer and device id, set per network hop and only towards [SessionCredentials.server]. As a
     * network interceptor it sees every redirect on its own; the application request never holds
     * the values, so OkHttp cannot copy them into a redirect.
     */
    class Credentials(private val credentials: SessionCredentials) : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request()
            val server = credentials.server
            if (server == null || !sameOrigin(server, request.url)) return chain.proceed(request)
            val builder = request.newBuilder()
            credentials.bearerToken?.let { builder.header("Authorization", "Bearer $it") }
            credentials.deviceId?.let { builder.header(HEADER_DEVICE, it) }
            return chain.proceed(builder.build())
        }
    }

    private val SAFE_METHODS = setOf("GET", "HEAD", "OPTIONS")

    fun sameOrigin(a: HttpUrl, b: HttpUrl): Boolean = a.scheme == b.scheme && a.host == b.host && a.port == b.port
}
