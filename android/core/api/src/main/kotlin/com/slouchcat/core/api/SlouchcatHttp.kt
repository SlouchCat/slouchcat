package com.slouchcat.core.api

import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Response
import java.time.Duration

/** Which app sends a request, as the server reads it from `X-Slouchcat-Client` (`mobile/1.0.0`). */
data class ClientIdentity(val kind: Kind, val version: String) {
    enum class Kind(val header: String) { MOBILE("mobile"), TV("tv") }

    val header: String get() = "${kind.header}/$version"
}

/**
 * What the shared client sends along for the one server the app is set up with. Part 2 of stage 3
 * fills it from the token store after the sign-in; until then [None] sends nothing.
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

    fun client(
        identity: ClientIdentity,
        credentials: SessionCredentials = SessionCredentials.None,
        builder: OkHttpClient.Builder = OkHttpClient.Builder(),
    ): OkHttpClient = builder
        .connectTimeout(CONNECT_TIMEOUT)
        .readTimeout(READ_TIMEOUT)
        .writeTimeout(WRITE_TIMEOUT)
        .followSslRedirects(false)
        .addInterceptor(Headers(identity))
        .addNetworkInterceptor(Credentials(credentials))
        .build()

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
