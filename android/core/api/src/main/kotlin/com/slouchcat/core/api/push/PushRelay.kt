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
    data class Registered(val endpoint: String) : RelayResult

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
                val answer = SlouchcatApi.json.parseToJsonElement(response.body.string()) as? JsonObject
                val endpoint = (answer?.get("endpoint") as? JsonPrimitive)?.takeIf { it.isString }?.content
                // The address must belong to the relay asked (scheme and host), never somewhere else.
                val url = endpoint?.toHttpUrlOrNull()
                if (url == null || url.host != base.host || url.scheme != base.scheme) RelayResult.Failed(response.code) else RelayResult.Registered(endpoint)
            }
        } catch (e: IOException) {
            RelayResult.Failed(null)
        } catch (e: kotlinx.serialization.SerializationException) {
            RelayResult.Failed(null)
        }
    }

    private companion object {
        val JSON = "application/json".toMediaType()
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
            if (url.startsWith("/") && !url.startsWith("//")) return url
            val parsed = url.toHttpUrlOrNull() ?: return null
            if (server == null || parsed.host != server.host || parsed.port != server.port) return null
            val path = parsed.encodedPath
            val prefix = server.encodedPath.trimEnd('/')
            val own = if (prefix.isNotEmpty() && path.startsWith(prefix)) path.removePrefix(prefix).ifEmpty { "/" } else path
            return own + (parsed.encodedQuery?.let { "?$it" } ?: "")
        }
    }
}
