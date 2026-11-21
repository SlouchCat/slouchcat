package com.slouchcat.core.pairing

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.log.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runInterruptible
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import okhttp3.Call
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.time.Instant
import java.time.format.DateTimeParseException

/**
 * The client of the relay (concept 27 7.1; ADR 0042; `relay/cmd/pair`). The relay is no part of the
 * server's contract: plain JSON, errors as `{"error":"<code>"}`. [http] must be a client without the
 * credentials of the user's server (it never sees token or device id).
 *
 * The relay is HTTPS in operation; a local relay of a test (`http://10.0.2.2:8180`) works as well.
 */
class RelayClient(private val http: Call.Factory, val base: HttpUrl) {
    /** `POST /v1/pair`. */
    sealed interface Open {
        data class Ok(val id: String, val expiresAt: Instant) : Open

        /** 429 or 503: too many open meeting points (of this caller or in total). */
        data object Busy : Open

        /** No answer: no network, wrong address, the relay is down. */
        data object Unreachable : Open

        data class Failed(val status: Int?) : Open
    }

    /** `GET /v1/pair/{id}` with the pickup secret. */
    sealed interface Poll {
        /** 200: the sealed packet (enc followed by the ciphertext); the meeting point is used up. */
        class Packet(val sealed: ByteArray) : Poll {
            override fun toString(): String = "Packet(${sealed.size} bytes)"
        }

        /** 204: nothing there yet. */
        data object Waiting : Poll

        /** 404 or 410: expired, unknown or already picked up. */
        data object Gone : Poll

        /** 401 or 403: the relay does not take this secret. */
        data object Refused : Poll

        data object Unreachable : Poll

        data class Failed(val status: Int?) : Poll
    }

    /** `PUT /v1/pair/{id}`, the phone's side (stage 9 of concept 27). */
    enum class Put {
        Delivered,

        /** 409: a packet is lying there already (maybe one somebody else put there, K26 4.6). */
        Occupied,

        /** 410: a packet was picked up already. */
        Consumed,

        /** 404: unknown or expired. */
        Unknown,
        Unreachable,
        Failed,
    }

    @Serializable
    private data class OpenRequest(val pickupHash: String)

    @Serializable
    private data class Opened(val id: String, val expiresAt: String)

    @Serializable
    private data class PacketBody(val packet: String)

    suspend fun open(secret: PickupSecret): Open {
        val body = PairingProtocol.json.encodeToString(OpenRequest.serializer(), OpenRequest(secret.hash))
        val request = Request.Builder().url(pairUrl()).post(body.toRequestBody(JSON)).build()
        return when (val answer = send(request)) {
            null -> Open.Unreachable
            else -> when (answer.status) {
                201 -> {
                    val opened = decode(answer.body) { PairingProtocol.json.decodeFromString(Opened.serializer(), it) }
                    val at = opened?.let { runCatching { Instant.parse(it.expiresAt) }.getOrNull() }
                    if (opened == null || at == null || !PairingProtocol.isKey(opened.id, PairingProtocol.ID_BYTES)) Open.Failed(answer.status) else Open.Ok(opened.id, at)
                }
                429, 503 -> Open.Busy
                else -> Open.Failed(answer.status)
            }
        }
    }

    suspend fun poll(id: String, secret: PickupSecret): Poll {
        val request = Request.Builder().url(pairUrl(id)).header("Authorization", secret.authorization).get().build()
        val answer = send(request) ?: return Poll.Unreachable
        return when (answer.status) {
            200 -> {
                val packet = decode(answer.body) { PairingProtocol.json.decodeFromString(PacketBody.serializer(), it) }
                val sealed = packet?.let { Base64Url.decode(it.packet) }
                if (sealed == null || sealed.size > PairingProtocol.MAX_PACKET_BYTES) Poll.Failed(answer.status) else Poll.Packet(sealed)
            }
            204 -> Poll.Waiting
            404, 410 -> Poll.Gone
            401, 403 -> Poll.Refused
            else -> Poll.Failed(answer.status)
        }
    }

    suspend fun put(id: String, sealed: ByteArray): Put {
        val body = PairingProtocol.json.encodeToString(PacketBody.serializer(), PacketBody(Base64Url.encode(sealed)))
        val request = Request.Builder().url(pairUrl(id)).put(body.toRequestBody(JSON)).build()
        val answer = send(request) ?: return Put.Unreachable
        return when (answer.status) {
            204 -> Put.Delivered
            409 -> Put.Occupied
            410 -> Put.Consumed
            404 -> Put.Unknown
            else -> Put.Failed
        }
    }

    /** `DELETE /v1/pair/{id}`: the TV gives the meeting point up; `true` when the relay confirmed. */
    suspend fun cancel(id: String, secret: PickupSecret): Boolean {
        val request = Request.Builder().url(pairUrl(id)).header("Authorization", secret.authorization).delete().build()
        return send(request)?.status == 204
    }

    private fun pairUrl(id: String? = null): HttpUrl = base.newBuilder()
        .addPathSegments("v1/pair")
        .apply { if (id != null) addPathSegment(id) }
        .build()

    private class Answer(val status: Int, val body: String)

    /** The answer, `null` without one. Logs only the status (an answer may carry the packet). */
    private suspend fun send(request: Request): Answer? = try {
        runInterruptible(Dispatchers.IO) {
            http.newCall(request).execute().use { response ->
                // At most MAX_ANSWER bytes are read: the relay is set by the user (review N-6).
                Answer(response.code, response.peekBody(MAX_ANSWER.toLong()).string())
            }
        }
    } catch (e: IOException) {
        Log.i(TAG, "relay unreachable", "method" to request.method, error = e)
        null
    }.also { answer -> if (answer != null && answer.status >= 400) Log.i(TAG, "relay refused", "method" to request.method, "status" to answer.status) }

    private inline fun <T> decode(body: String, read: (String) -> T): T? = try {
        read(body)
    } catch (e: SerializationException) {
        null
    } catch (e: IllegalArgumentException) {
        null
    } catch (e: DateTimeParseException) {
        null
    }

    private companion object {
        const val TAG = "relay"
        val JSON = "application/json".toMediaType()

        /** More than a packet of 4 KB in base64url and its JSON. */
        const val MAX_ANSWER = 16 * 1024
    }
}
