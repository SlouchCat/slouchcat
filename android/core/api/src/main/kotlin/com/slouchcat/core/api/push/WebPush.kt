package com.slouchcat.core.api.push

import com.slouchcat.core.api.crypto.Base64Url
import java.math.BigInteger
import java.nio.ByteBuffer
import java.security.AlgorithmParameters
import java.security.GeneralSecurityException
import java.security.KeyFactory
import java.security.KeyPairGenerator
import java.security.SecureRandom
import java.security.interfaces.ECPrivateKey
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec
import java.security.spec.ECParameterSpec
import java.security.spec.ECPoint
import java.security.spec.ECPrivateKeySpec
import java.security.spec.ECPublicKeySpec
import javax.crypto.Cipher
import javax.crypto.KeyAgreement
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * The keys of this device's push subscription (RFC 8291): the P-256 key pair of the user agent
 * (private scalar 32 bytes, public point 65 bytes uncompressed) and the 16-byte auth secret. The
 * server gets [publicKey] (`p256dh`) and [auth]; the private key never leaves the device.
 */
class PushKeys(val privateKey: ByteArray, val publicKey: ByteArray, val auth: ByteArray) {
    init {
        require(privateKey.size == 32 && publicKey.size == 65 && publicKey[0].toInt() == 4 && auth.size == 16) { "push keys of the wrong shape" }
    }

    /** No key material in logs or exceptions. */
    override fun toString(): String = "PushKeys(…)"
}

/** A push body that does not decrypt: wrong key, cut short, too large, tampered with. */
class WebPushException(message: String) : GeneralSecurityException(message)

/**
 * Web-Push decryption of `aes128gcm` (RFC 8188, RFC 8291) on the device (concept 27, 7.3; stage 0
 * decision 3: own code over JCA, no Tink apps-webpush). The push relay hands the server's body on
 * unchanged as `data.wp`; the record size comes from the body's header (2700 for the relay, 4096
 * for others), so any size up to [MAX_BODY] decrypts.
 */
object WebPush {
    /** The largest body the app accepts: Google's limit for a whole message (4096 bytes) bounds it anyway. */
    const val MAX_BODY = 4096

    private const val SALT = 16
    private const val HEADER = SALT + 4 + 1
    private const val TAG = 16
    private val random = SecureRandom()

    private val params: ECParameterSpec by lazy {
        AlgorithmParameters.getInstance("EC").apply { init(ECGenParameterSpec("secp256r1")) }.getParameterSpec(ECParameterSpec::class.java)
    }

    /** A fresh key pair and auth secret for a new subscription. */
    fun generate(): PushKeys {
        val pair = KeyPairGenerator.getInstance("EC").apply { initialize(ECGenParameterSpec("secp256r1"), random) }.generateKeyPair()
        val pub = pair.public as ECPublicKey
        val priv = pair.private as ECPrivateKey
        return PushKeys(fixed(priv.s, 32), uncompressed(pub.w), ByteArray(16).also(random::nextBytes))
    }

    /** `p256dh` and `auth` as `PushSubscription.toJSON()` writes them (base64url without padding). */
    fun p256dh(keys: PushKeys): String = Base64Url.encode(keys.publicKey)

    fun auth(keys: PushKeys): String = Base64Url.encode(keys.auth)

    /** The plaintext of [body] for [keys]; throws [WebPushException] for anything that is not a valid message to them. */
    fun decrypt(body: ByteArray, keys: PushKeys): ByteArray {
        if (body.size > MAX_BODY) throw WebPushException("body too large")
        if (body.size < HEADER) throw WebPushException("body too short")
        val buf = ByteBuffer.wrap(body)
        val salt = ByteArray(SALT).also { buf.get(it) }
        val rs = buf.int.toLong() and 0xffffffffL
        if (rs < TAG + 2) throw WebPushException("record size")
        val idLen = buf.get().toInt() and 0xff
        if (idLen != 65 || buf.remaining() < idLen) throw WebPushException("sender key")
        val senderKey = ByteArray(idLen).also { buf.get(it) }
        if (senderKey[0].toInt() != 4) throw WebPushException("sender key")
        val ciphertext = ByteArray(buf.remaining()).also { buf.get(it) }
        if (ciphertext.size <= TAG) throw WebPushException("body too short")

        val shared = try {
            val factory = KeyFactory.getInstance("EC")
            val priv = factory.generatePrivate(ECPrivateKeySpec(BigInteger(1, keys.privateKey), params))
            val point = ECPoint(BigInteger(1, senderKey.copyOfRange(1, 33)), BigInteger(1, senderKey.copyOfRange(33, 65)))
            val pub = factory.generatePublic(ECPublicKeySpec(point, params))
            KeyAgreement.getInstance("ECDH").run {
                init(priv)
                doPhase(pub, true)
                generateSecret()
            }
        } catch (e: GeneralSecurityException) {
            throw WebPushException("key agreement")
        } catch (e: IllegalArgumentException) {
            throw WebPushException("key agreement")
        }
        val ikm = hkdf(keys.auth, shared, "WebPush: info\u0000".toByteArray() + keys.publicKey + senderKey, 32)
        val cek = hkdf(salt, ikm, "Content-Encoding: aes128gcm\u0000".toByteArray(), 16)
        val nonce = hkdf(salt, ikm, "Content-Encoding: nonce\u0000".toByteArray(), 12)

        val out = java.io.ByteArrayOutputStream()
        val size = rs.coerceAtMost(MAX_BODY.toLong()).toInt()
        val records = (ciphertext.indices step size).map { start -> ciphertext.copyOfRange(start, minOf(start + size, ciphertext.size)) }
        records.forEachIndexed { index, record ->
            val plain = try {
                Cipher.getInstance("AES/GCM/NoPadding").run {
                    init(Cipher.DECRYPT_MODE, SecretKeySpec(cek, "AES"), GCMParameterSpec(128, nonceFor(nonce, index)))
                    doFinal(record)
                }
            } catch (e: GeneralSecurityException) {
                throw WebPushException("record does not decrypt")
            }
            var end = plain.size - 1
            while (end >= 0 && plain[end].toInt() == 0) end--
            val last = index == records.lastIndex
            val delimiter = if (last) 2 else 1
            if (end < 0 || plain[end].toInt() != delimiter) throw WebPushException("record delimiter")
            out.write(plain, 0, end)
        }
        return out.toByteArray()
    }

    private fun nonceFor(base: ByteArray, sequence: Int): ByteArray {
        val nonce = base.copyOf()
        for (i in 0 until 4) nonce[nonce.size - 1 - i] = (nonce[nonce.size - 1 - i].toInt() xor ((sequence ushr (8 * i)) and 0xff)).toByte()
        return nonce
    }

    private fun hkdf(salt: ByteArray, ikm: ByteArray, info: ByteArray, length: Int): ByteArray {
        val prk = Mac.getInstance("HmacSHA256").run {
            init(SecretKeySpec(salt, "HmacSHA256"))
            doFinal(ikm)
        }
        return Mac.getInstance("HmacSHA256").run {
            init(SecretKeySpec(prk, "HmacSHA256"))
            doFinal(info + byteArrayOf(1))
        }.copyOf(length)
    }

    private fun fixed(value: BigInteger, size: Int): ByteArray {
        val bytes = value.toByteArray()
        return when {
            bytes.size == size -> bytes
            bytes.size > size -> bytes.copyOfRange(bytes.size - size, bytes.size)
            else -> ByteArray(size - bytes.size) + bytes
        }
    }

    private fun uncompressed(point: ECPoint): ByteArray = byteArrayOf(4) + fixed(point.affineX, 32) + fixed(point.affineY, 32)
}
