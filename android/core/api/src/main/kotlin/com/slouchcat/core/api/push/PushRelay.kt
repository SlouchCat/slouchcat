package com.slouchcat.core.api.push

import com.slouchcat.core.api.SlouchcatApi
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.util.Base64

/** What the push relay said to `POST /v1/register`. */
sealed interface RelayResult {
    /** The push address to give the server (`createPushSubscription.endpoint`). */
    data class Registered(val endpoint: String) : RelayResult {
        /** The address holds the sealed Google token: never in a log (rule 10: `sealedtoken`). */
        override fun toString(): String = "Registered(…)"
    }

    /** The relay refused the token or the key (400), or is rate limited (429), or down (503 or no answer). */
    data class Failed(val status: Int?) : RelayResult
}

/**
 * The client of the push relay (concept 27, 7.3; ADR 0044; `relay/cmd/push`): `POST /v1/register`
 * with the Google token and the server's VAPID key, back comes the sealed push address. The relay
 * keeps nothing; the address holds both sealed. [base]: `https://push.slouchcat.com` (a build of
 * one's own with its own Firebase project names its own relay, see the hand-over).
 */
class PushRelay(private val base: HttpUrl, private val http: Call.Factory) {
    @Serializable
    private data class Register(val fcmToken: String, val vapidKey: String)

    fun register(fcmToken: String, vapidKey: String): RelayResult {
        val body = SlouchcatApi.json.encodeToString(Register.serializer(), Register(fcmToken, vapidKey)).toRequestBody(JSON)
        val request = Request.Builder().url(base.newBuilder().addPathSegments("v1/register").build()).post(body).build()
        return try {
            http.newCall(request).execute().use { response ->
                if (!response.isSuccessful) return RelayResult.Failed(response.code)
                // A bounded answer: the address is short (security review L-2).
                val source = response.body.source()
                if (source.request(MAX_ANSWER + 1L)) return RelayResult.Failed(null)
                val answer = SlouchcatApi.json.parseToJsonElement(source.buffer.readUtf8()) as? JsonObject
                val endpoint = (answer?.get("endpoint") as? JsonPrimitive)?.takeIf { it.isString }?.content
                if (endpoint != null && isOwnAddress(endpoint)) RelayResult.Registered(endpoint) else RelayResult.Failed(null)
            }
        } catch (e: IOException) {
            RelayResult.Failed(null)
        } catch (e: kotlinx.serialization.SerializationException) {
            RelayResult.Failed(null)
        }
    }

    /**
     * The address must be one of the relay asked (scheme, host and port), of the form
     * `/v1/push/<version>.<sealed>` (relay/internal/push), no query, no fragment, and fit the server's
     * limit of 2048 characters; anything else is refused (security review L-2, code review L-2).
     */
    internal fun isOwnAddress(endpoint: String): Boolean {
        if (endpoint.length > MAX_ENDPOINT) return false
        val url = endpoint.toHttpUrlOrNull() ?: return false
        return url.scheme == base.scheme && url.host == base.host && url.port == base.port &&
            url.querySize == 0 && url.fragment == null && PATH.matches(url.encodedPath) &&
            runCatching { java.net.URI(endpoint) }.isSuccess
    }

    private companion object {
        val JSON = "application/json".toMediaType()
        const val MAX_ANSWER = 8192
        const val MAX_ENDPOINT = 2048
        val PATH = Regex("/v1/push/[a-z0-9]{1,16}\\.[A-Za-z0-9_-]{16,2000}")
    }
}

/** A push message as the server sends it (`{title, body, url}`); the profile name is already in [title] (rule 12). */
data class PushMessage(val title: String, val body: String, val url: String?) {
    companion object {
        private const val MAX_TEXT = 4000

        /** Parses the decrypted payload; `null` for anything that is not one. */
        fun parse(plain: ByteArray): PushMessage? {
            val obj = runCatching { SlouchcatApi.json.parseToJsonElement(plain.toString(Charsets.UTF_8)) as? JsonObject }.getOrNull() ?: return null
            fun text(key: String) = (obj[key] as? JsonPrimitive)?.takeIf { it.isString }?.content
            val title = text("title")?.take(MAX_TEXT) ?: return null
            return PushMessage(title, text("body").orEmpty().take(MAX_TEXT), text("url"))
        }

        /**
         * The FCM data field `wp` back to the encrypted body: standard base64 with padding as the
         * relay writes it, base64url accepted as well; `null` when it is neither or too large.
         */
        fun body(wp: String): ByteArray? {
            if (wp.length > WebPush.MAX_BODY * 4 / 3 + 4) return null
            val bytes = runCatching { Base64.getDecoder().decode(wp) }.getOrNull()
                ?: runCatching { Base64.getUrlDecoder().decode(wp) }.getOrNull()
            return bytes?.takeIf { it.size <= WebPush.MAX_BODY }
        }

        /**
         * The in-app address of [url] (concept 27, 7.3 point 6): only the path (and query) of the
         * own server's address or of a bare path; a foreign host is never opened (`null`).
         * `/admin…` stays as it is; the app opens it in the browser under the own server.
         */
        fun path(url: String?, server: HttpUrl?): String? {
            if (url.isNullOrBlank()) return null
            if (url.startsWith("/")) return url.takeIf { safe(it) }
            val parsed = url.toHttpUrlOrNull() ?: return null
            if (server == null || parsed.scheme != server.scheme || parsed.host != server.host || parsed.port != server.port) return null
            val path = parsed.encodedPath
            val prefix = server.encodedPath.trimEnd('/')
            // The server's own sub path, cut at a «/» only (code review L-1).
            val own = when {
                prefix.isEmpty() -> path
                path == prefix -> "/"
                path.startsWith("$prefix/") -> path.removePrefix(prefix)
                else -> return null
            }
            return (own + (parsed.encodedQuery?.let { "?$it" } ?: "")).takeIf { safe(it) }
        }

        /**
         * A path of the own server only: one leading «/», no second one, no backslash, no control
         * character and no encoded slash or backslash at its start (security review I-1).
         */
        private fun safe(path: String): Boolean {
            if (!path.startsWith("/") || path.startsWith("//")) return false
            if (path.any { it == '\\' || it.isISOControl() }) return false
            val head = path.drop(1).take(3).lowercase()
            return !head.startsWith("%2f") && !head.startsWith("%5c")
        }
    }
}
