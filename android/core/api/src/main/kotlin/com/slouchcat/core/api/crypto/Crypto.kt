package com.slouchcat.core.api.crypto

import com.google.crypto.tink.subtle.Ed25519Verify
import java.security.GeneralSecurityException
import java.security.SecureRandom
import java.util.Base64

/**
 * The cryptography the apps share with the server (protocols of the build log, test vectors in
 * `testdata/crypto/`): Ed25519 through Tink (stage 0, point 7). «Server weist sich aus» uses it
 * today; the pairing of stage 9 adds HPKE next to it and reuses [Base64Url] and [ServerProof].
 */
object Base64Url {
    private val encoder = Base64.getUrlEncoder().withoutPadding()
    private val decoder = Base64.getUrlDecoder()

    fun encode(bytes: ByteArray): String = encoder.encodeToString(bytes)

    /** Decodes base64url with or without padding; `null` for anything else. */
    fun decode(text: String): ByteArray? = try {
        decoder.decode(text)
    } catch (e: IllegalArgumentException) {
        null
    }
}

object Ed25519 {
    const val PUBLIC_KEY_BYTES = 32
    const val SIGNATURE_BYTES = 64

    /** Whether [signature] is a valid Ed25519 signature of [message] under [publicKey]; never throws. */
    fun verify(publicKey: ByteArray, message: ByteArray, signature: ByteArray): Boolean {
        if (publicKey.size != PUBLIC_KEY_BYTES || signature.size != SIGNATURE_BYTES) return false
        return try {
            Ed25519Verify(publicKey).verify(signature, message)
            true
        } catch (e: GeneralSecurityException) {
            false
        } catch (e: IllegalArgumentException) {
            false
        }
    }
}

/**
 * The proof of `proveServer` (concept 27 section 6, F-38; ADR 0040): the server signs the UTF-8
 * prefix `slouchcat-server-proof-v1:` followed by the 32 bytes of the app's nonce.
 */
object ServerProof {
    const val PREFIX = "slouchcat-server-proof-v1:"
    const val NONCE_BYTES = 32

    private val random = SecureRandom()

    /** A fresh nonce of 32 random bytes. */
    fun nonce(): ByteArray = ByteArray(NONCE_BYTES).also(random::nextBytes)

    fun message(nonce: ByteArray): ByteArray = PREFIX.toByteArray(Charsets.UTF_8) + nonce

    /**
     * What a person compares: the first 16 bytes of SHA-256 over the key, in groups of four hex
     * digits (`3f2a 91c0 …`); `null` for a key that is not base64url.
     */
    fun fingerprint(serverKey: String): String? {
        val key = Base64Url.decode(serverKey) ?: return null
        val digest = java.security.MessageDigest.getInstance("SHA-256").digest(key).copyOf(FINGERPRINT_BYTES)
        return digest.joinToString("") { "%02x".format(it) }.chunked(4).joinToString(" ")
    }

    private const val FINGERPRINT_BYTES = 16

    /** Whether [signature] (base64url) signs [nonce] under [serverKey] (base64url). */
    fun valid(serverKey: String, nonce: ByteArray, signature: String): Boolean {
        val key = Base64Url.decode(serverKey) ?: return false
        val sig = Base64Url.decode(signature) ?: return false
        return Ed25519.verify(key, message(nonce), sig)
    }
}
