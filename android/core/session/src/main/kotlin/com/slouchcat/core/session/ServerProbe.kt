package com.slouchcat.core.session

import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import okhttp3.HttpUrl
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.IOException
import java.time.Duration

/** What a server told about itself in `getHealth`. */
data class ServerInfo(
    val address: HttpUrl,
    /** The server's release, `Health.version` (`1.42.0`, `1.42.0+abc1234`). */
    val version: String,
    /** The contract the server speaks, `Health.contractVersion`. */
    val contractVersion: String,
    /**
     * The certificate of an https server is trusted only through an authority the user installed
     * on the device (a household's own CA). The page says so instead of plain «encrypted»: whoever
     * can install such an authority on the device can read along (ADR 0039).
     */
    val userCertificate: Boolean = false,
) {
    val encrypted: Boolean get() = address.isHttps
}

sealed interface ProbeResult {
    data class Found(val info: ServerInfo) : ProbeResult

    /** No answer at all: wrong address, wrong network, server down, TLS failed. */
    data object Unreachable : ProbeResult

    /** An answer, but not the health of a Slouchcat server. */
    data object NotSlouchcat : ProbeResult

    /** A Slouchcat server whose contract is older than the app needs (11.5). */
    data class TooOld(val serverVersion: String, val required: String) : ProbeResult

    /** The http address redirects to https of the same host: offer that address. */
    data class RequiresHttps(val address: HttpUrl) : ProbeResult
}

/** Whether a server's certificate chain is trusted only through a CA the user installed; a seam for tests. */
fun interface TrustInspector {
    fun usesUserCertificate(chain: List<java.security.cert.Certificate>, host: String): Boolean

    companion object {
        val None = TrustInspector { _, _ -> false }
    }
}

fun interface ServerProbe {
    suspend fun probe(address: HttpUrl): ProbeResult
}

/** The contract versions the apps need (concept 27, 11.5). */
object Compatibility {
    /** Contract 1.42.0 brings the app sessions (K27 stage 1); older servers cannot sign an app in. */
    const val MIN_CONTRACT = "1.42.0"

    /** Whether [contract] is at least [MIN_CONTRACT]; a build suffix (`+abc`, `-rc1`) is ignored. */
    fun supports(contract: String): Boolean {
        val have = numbers(contract) ?: return false
        val need = numbers(MIN_CONTRACT)!!
        for (i in 0 until 3) {
            if (have[i] != need[i]) return have[i] > need[i]
        }
        return true
    }

    private fun numbers(version: String): List<Int>? {
        val parts = version.trim().removePrefix("v").split('+', '-').first().split('.')
        if (parts.size != 3) return null
        return parts.map { it.toIntOrNull() ?: return null }
    }
}

/**
 * Asks `getHealth` (`GET /api/v1/healthz`) over the shared client. Read as plain JSON rather than
 * through the generated `Health`: an older server lacks `contractVersion`, which the app must
 * report as «server too old» and not as an answer it cannot read. Redirects are not followed, so
 * the address the user confirms is the one that answered.
 */
class HealthServerProbe(client: OkHttpClient, private val trust: TrustInspector = TrustInspector.None) : ServerProbe {
    private val http = client.newBuilder()
        .followRedirects(false)
        .followSslRedirects(false)
        .callTimeout(Duration.ofSeconds(15))
        .build()

    override suspend fun probe(address: HttpUrl): ProbeResult = withContext(Dispatchers.IO) {
        val url = "${SlouchcatApi.basePath(address)}/healthz"
        var userCertificate = false
        val body = try {
            http.newCall(Request.Builder().url(url).build()).execute().use { response ->
                if (response.code in REDIRECTS) {
                    val target = response.header("Location")?.let { response.request.url.resolve(it) }
                    if (target != null && target.isHttps && target.host == address.host) {
                        return@withContext ProbeResult.RequiresHttps(address.newBuilder().scheme("https").port(target.port).build())
                    }
                }
                if (response.code != 200) {
                    Log.i(TAG, "health answered", "status" to response.code)
                    return@withContext ProbeResult.NotSlouchcat
                }
                val chain = response.handshake?.peerCertificates.orEmpty()
                userCertificate = chain.isNotEmpty() && trust.usesUserCertificate(chain, address.host)
                response.body.string()
            }
        } catch (e: IOException) {
            Log.i(TAG, "server unreachable", error = e)
            return@withContext ProbeResult.Unreachable
        }
        when (val result = read(address, body)) {
            is ProbeResult.Found -> result.copy(info = result.info.copy(userCertificate = userCertificate))
            else -> result
        }
    }

    companion object {
        private const val TAG = "probe"
        private val REDIRECTS = setOf(301, 302, 303, 307, 308)

        /** The decision on a 200 answer of `getHealth`; separate for tests. */
        fun read(address: HttpUrl, body: String): ProbeResult {
            val health = try {
                SlouchcatApi.json.parseToJsonElement(body) as? JsonObject
            } catch (e: IllegalArgumentException) {
                null
            } ?: return ProbeResult.NotSlouchcat
            val status = (health["status"] as? JsonPrimitive)?.takeIf { it.isString }?.content
            val version = (health["version"] as? JsonPrimitive)?.takeIf { it.isString }?.content
            if (status != "ok" || version.isNullOrBlank()) return ProbeResult.NotSlouchcat
            val contract = (health["contractVersion"] as? JsonPrimitive)?.takeIf { it.isString }?.content
            if (contract == null || !Compatibility.supports(contract)) {
                return ProbeResult.TooOld(serverVersion = version, required = Compatibility.MIN_CONTRACT)
            }
            return ProbeResult.Found(ServerInfo(address, version, contract))
        }
    }
}
