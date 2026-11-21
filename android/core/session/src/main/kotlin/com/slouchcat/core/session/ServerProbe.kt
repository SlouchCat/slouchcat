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
class HealthServerProbe(client: OkHttpClient) : ServerProbe {
    private val http = client.newBuilder()
        .followRedirects(false)
        .followSslRedirects(false)
        .callTimeout(Duration.ofSeconds(15))
        .build()

    override suspend fun probe(address: HttpUrl): ProbeResult = withContext(Dispatchers.IO) {
        val url = "${SlouchcatApi.basePath(address)}/healthz"
        val body = try {
            http.newCall(Request.Builder().url(url).build()).execute().use { response ->
                if (response.code != 200) {
                    Log.i(TAG, "health answered", "status" to response.code)
                    return@withContext ProbeResult.NotSlouchcat
                }
                response.body.string()
            }
        } catch (e: IOException) {
            Log.i(TAG, "server unreachable", error = e)
            return@withContext ProbeResult.Unreachable
        }
        read(address, body)
    }

    companion object {
        private const val TAG = "probe"

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
