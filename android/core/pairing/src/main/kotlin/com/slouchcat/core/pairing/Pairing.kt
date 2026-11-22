package com.slouchcat.core.pairing

import com.google.crypto.tink.InsecureSecretKeyAccess
import com.google.crypto.tink.hybrid.HpkeParameters
import com.google.crypto.tink.hybrid.HpkePrivateKey
import com.google.crypto.tink.hybrid.HpkePublicKey
import com.google.crypto.tink.hybrid.internal.HpkeDecrypt
import com.google.crypto.tink.hybrid.internal.HpkeEncrypt
import com.google.crypto.tink.subtle.X25519
import com.google.crypto.tink.util.Bytes
import com.google.crypto.tink.util.SecretBytes
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.crypto.Ed25519
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import java.security.GeneralSecurityException
import java.security.MessageDigest
import java.security.SecureRandom
import java.time.Duration

/**
 * The fixed values of the pairing (build log «Protokolle», concept 27 7.1 and 7.2, ADR 0040 and
 * 0042). Every binary value travels as base64url without padding.
 */
object PairingProtocol {
    /** Version of the QR code and of the packet. */
    const val VERSION = 1

    /** `info` of the packet: this prefix and the meeting point id as text. */
    const val PACKET_INFO_PREFIX = "slouchcat-pair-v1:"

    /** `info` of the sealed token in the redeem answer. */
    const val REDEEM_INFO = "slouchcat-redeem-v1"

    /** What the server signs with its key: this prefix, then the bytes of `sealedToken`. */
    const val REDEEM_SIGNATURE_PREFIX = "slouchcat-redeem-signature-v1:"

    const val ID_BYTES = 16
    const val SECRET_BYTES = 32
    const val KEY_BYTES = 32

    /** The relay takes packets up to 4 KB (K-4). */
    const val MAX_PACKET_BYTES = 4096

    /** The relay built into the apps (F-37); changeable in the advanced settings (F-21). */
    const val DEFAULT_RELAY = "https://pair.slouchcat.com"

    /** How often the TV asks the relay for the packet (build decision, K27 7.1). */
    val POLL_INTERVAL: Duration = Duration.ofSeconds(2)

    /** The path of the help page on the relay that the QR code points to. */
    const val HELP_PATH = "tv"

    internal val json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
        encodeDefaults = true
    }

    internal val random = SecureRandom()

    internal fun randomBytes(size: Int): ByteArray = ByteArray(size).also(random::nextBytes)

    /** Whether [text] is base64url without padding of exactly [bytes] bytes. */
    fun isKey(text: String, bytes: Int = KEY_BYTES): Boolean =
        text.length == (bytes * 4 + 2) / 3 && text.all { it.isLetterOrDigit() && it.code < 128 || it == '-' || it == '_' } &&
            Base64Url.decode(text)?.size == bytes
}

/**
 * The one-time X25519 key of a TV for one pairing (concept 26, 4.3 step 1). It lives only in
 * memory, in the object that runs this one pairing; it is never stored, logged or shown.
 */
class TvKeyPair private constructor(private val privateKey: ByteArray, val publicKey: ByteArray) {
    /** The public key as the QR code and the redeem carry it. */
    val publicText: String get() = Base64Url.encode(publicKey)

    /** Opens something HPKE-sealed for this key ([PairingProtocol.REDEEM_INFO] or a packet's info). */
    fun open(info: ByteArray, sealed: ByteArray): ByteArray = Hpke.open(privateKey, publicKey, info, sealed)

    /** Overwrites the private key; the pair is useless afterwards. */
    fun destroy() = privateKey.fill(0)

    override fun toString(): String = "TvKeyPair(public=$publicText, private=[redacted])"

    companion object {
        fun generate(): TvKeyPair {
            val private = X25519.generatePrivateKey()
            return TvKeyPair(private, X25519.publicFromPrivate(private))
        }

        /** From a known private key (test vectors). */
        fun of(privateKey: ByteArray): TvKeyPair = TvKeyPair(privateKey.copyOf(), X25519.publicFromPrivate(privateKey))
    }
}

/**
 * The pickup secret (K-1): 32 random bytes that never leave the TV; the relay knows only their
 * SHA-256 and hands the packet only to whoever sends the secret itself.
 */
class PickupSecret private constructor(private val raw: ByteArray) {
    /** SHA-256 over the 32 raw bytes, base64url: what `POST /v1/pair` sends. */
    val hash: String get() = Base64Url.encode(MessageDigest.getInstance("SHA-256").digest(raw))

    /** The header `Authorization: Pickup <secret>` of `GET` and `DELETE`. */
    internal val authorization: String get() = "Pickup ${Base64Url.encode(raw)}"

    fun destroy() = raw.fill(0)

    override fun toString(): String = "PickupSecret([redacted])"

    companion object {
        fun generate(): PickupSecret = PickupSecret(PairingProtocol.randomBytes(PairingProtocol.SECRET_BYTES))

        /** From base64url (test vectors); `null` unless it is 32 bytes. */
        fun of(text: String): PickupSecret? = Base64Url.decode(text)?.takeIf { it.size == PairingProtocol.SECRET_BYTES }?.let(::PickupSecret)
    }
}

/**
 * HPKE (RFC 9180) base mode, one message, with the suite of the build log: DHKEM(X25519,
 * HKDF-SHA256), HKDF-SHA256, AES-256-GCM, empty `aad`. The sealed form is `enc` (32 bytes)
 * followed by the ciphertext, the same as Go's `hpke.Seal` (K27 stage 0, point 1: Tink, NO_PREFIX).
 */
object Hpke {
    private val parameters: HpkeParameters = HpkeParameters.builder()
        .setVariant(HpkeParameters.Variant.NO_PREFIX)
        .setKemId(HpkeParameters.KemId.DHKEM_X25519_HKDF_SHA256)
        .setKdfId(HpkeParameters.KdfId.HKDF_SHA256)
        .setAeadId(HpkeParameters.AeadId.AES_256_GCM)
        .build()

    /** Seals [plain] for [recipient] (32 bytes); throws [GeneralSecurityException] for a key that is none. */
    fun seal(recipient: ByteArray, info: ByteArray, plain: ByteArray): ByteArray =
        HpkeEncrypt.create(publicKey(recipient)).encrypt(plain, info)

    /** Opens [sealed]; throws [GeneralSecurityException] when it was not sealed for this key and [info]. */
    fun open(privateKey: ByteArray, publicKey: ByteArray, info: ByteArray, sealed: ByteArray): ByteArray {
        if (sealed.size < PairingProtocol.KEY_BYTES + TAG_BYTES) throw GeneralSecurityException("sealed message too short")
        val private = HpkePrivateKey.create(publicKey(publicKey), SecretBytes.copyFrom(privateKey, InsecureSecretKeyAccess.get()))
        return HpkeDecrypt.create(private).decrypt(sealed, info)
    }

    private fun publicKey(bytes: ByteArray): HpkePublicKey {
        if (bytes.size != PairingProtocol.KEY_BYTES) throw GeneralSecurityException("not an X25519 key")
        return HpkePublicKey.create(parameters, Bytes.copyFrom(bytes), null)
    }

    private const val TAG_BYTES = 16
}

/**
 * The plaintext of the packet the phone lays down for the TV: the server address under which the
 * phone reaches its server, the voucher and the server's Ed25519 key (F-38).
 */
@Serializable
data class PairingPacket(
    val v: Int = PairingProtocol.VERSION,
    val server: String,
    val voucher: String,
    val serverKey: String? = null,
) {
    /** Never the voucher (rule 10, K-8). */
    override fun toString(): String = "PairingPacket(v=$v, server=$server, voucher=[redacted], serverKey=$serverKey)"

    /** Seals the packet for the TV's [tvKey] (32 bytes) and the meeting point [id]: the phone's side. */
    fun seal(tvKey: ByteArray, id: String): ByteArray =
        Hpke.seal(tvKey, info(id), PairingProtocol.json.encodeToString(serializer(), this).encodeToByteArray())

    companion object {
        fun info(id: String): ByteArray = (PairingProtocol.PACKET_INFO_PREFIX + id).encodeToByteArray()

        /**
         * Opens a packet with the TV's keys; `null` when it is not sealed for them and this meeting
         * point, is no packet, or has another version.
         */
        fun open(keys: TvKeyPair, id: String, sealed: ByteArray): PairingPacket? {
            val plain = try {
                keys.open(info(id), sealed)
            } catch (e: GeneralSecurityException) {
                return null
            }
            val packet = try {
                PairingProtocol.json.decodeFromString(serializer(), plain.decodeToString())
            } catch (e: SerializationException) {
                return null
            } catch (e: IllegalArgumentException) {
                return null
            }
            return packet.takeIf { it.v == PairingProtocol.VERSION && it.voucher.isNotBlank() && it.server.isNotBlank() }
        }
    }
}

/** The plaintext of `sealedToken`: the device token, its device id and the account (K-9). */
@Serializable
data class RedeemAnswer(val token: String, val deviceId: String, val accountEmail: String) {
    /** Never the token or the device id (rule 10). */
    override fun toString(): String = "RedeemAnswer(token=[redacted], deviceId=[redacted], accountEmail=$accountEmail)"

    companion object {
        /** Opens `sealedToken` with the TV's keys; `null` when that fails or it is no answer. */
        fun open(keys: TvKeyPair, sealed: ByteArray): RedeemAnswer? {
            val plain = try {
                keys.open(PairingProtocol.REDEEM_INFO.encodeToByteArray(), sealed)
            } catch (e: GeneralSecurityException) {
                return null
            }
            val answer = try {
                PairingProtocol.json.decodeFromString(serializer(), plain.decodeToString())
            } catch (e: SerializationException) {
                return null
            } catch (e: IllegalArgumentException) {
                return null
            }
            return answer.takeIf { it.token.isNotBlank() && it.deviceId.isNotBlank() }
        }
    }
}

/** The server's signature over the redeem answer (ADR 0040): checked before the token is opened. */
object RedeemSignature {
    fun message(sealed: ByteArray): ByteArray = PairingProtocol.REDEEM_SIGNATURE_PREFIX.encodeToByteArray() + sealed

    /** Whether [signature] (base64url) signs [sealed] under [serverKey] (base64url); never throws. */
    fun valid(serverKey: String, sealed: ByteArray, signature: String): Boolean {
        val key = Base64Url.decode(serverKey) ?: return false
        val sig = Base64Url.decode(signature) ?: return false
        return Ed25519.verify(key, message(sealed), sig)
    }
}

/**
 * The content of the QR code (concept 26 4.3, K27 7.2): `https://<relay>/tv#v=1&i=<id>&k=<key>`.
 * The part after `#` never reaches a server; a camera app lands on the help page of the relay.
 */
data class PairingCode(val relay: HttpUrl, val id: String, val tvKey: String) {
    /** The text the QR code carries. */
    val text: String
        get() = "${relay.newBuilder().addPathSegment(PairingProtocol.HELP_PATH).build()}#v=${PairingProtocol.VERSION}&i=$id&k=$tvKey"

    override fun toString(): String = "PairingCode(relay=$relay)"

    companion object {
        /** Reads a scanned code; `null` for anything that is not exactly such a code of version 1. */
        fun parse(text: String): PairingCode? {
            val hash = text.indexOf('#')
            if (hash < 0) return null
            val address = text.substring(0, hash).toHttpUrlOrNull() ?: return null
            val segments = address.pathSegments.filter { it.isNotEmpty() }
            if (segments.lastOrNull() != PairingProtocol.HELP_PATH || address.query != null) return null
            val relay = address.newBuilder().encodedPath("/").apply { segments.dropLast(1).forEach { addPathSegment(it) } }.build()
            val fields = text.substring(hash + 1).split('&').associate { part ->
                val eq = part.indexOf('=')
                if (eq < 0) part to "" else part.substring(0, eq) to part.substring(eq + 1)
            }
            if (fields.size != 3 || fields["v"] != PairingProtocol.VERSION.toString()) return null
            val id = fields["i"] ?: return null
            val key = fields["k"] ?: return null
            if (!PairingProtocol.isKey(id, PairingProtocol.ID_BYTES) || !PairingProtocol.isKey(key)) return null
            return PairingCode(relay, id, key)
        }
    }
}
