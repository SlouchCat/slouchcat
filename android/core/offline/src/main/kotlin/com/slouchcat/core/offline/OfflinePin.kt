package com.slouchcat.core.offline

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.models.OfflinePinVerifier
import java.security.MessageDigest
import javax.crypto.SecretKeyFactory
import javax.crypto.spec.PBEKeySpec

/**
 * The offline PIN (AN-19, concept 27 9.5; declared exception 1 of plan principle 1): the device
 * compares a PIN with a PBKDF2-SHA256 verifier the server delivered (separate offline PIN) or one it
 * derived itself after a successful online PIN entry (normal PIN, with the server's salt). The aim
 * is a curious child at a shared device, not a manipulated phone: a four-digit PIN falls to a
 * search of the stored verifier (the page says so). Same values as the PWA's pin.ts and
 * `testdata/crypto/pbkdf2.json`. Never on the main thread (over a second on a phone).
 */
object OfflinePin {
    /** Failures that pass without a pause; the third one starts the brake. */
    private const val FREE_FAILURES = 2
    private const val FIRST_LOCK_MS = 30_000L
    private const val MAX_LOCK_MS = 15 * 60_000L
    private const val HASH_BYTES = 32

    /** PBKDF2-SHA256 of the PIN (UTF-8) with the platform's provider. */
    fun pbkdf2(pin: CharArray, salt: ByteArray, iterations: Int, bytes: Int = HASH_BYTES): ByteArray {
        val spec = PBEKeySpec(pin, salt, iterations, bytes * 8)
        try {
            return SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).encoded
        } finally {
            spec.clearPassword()
        }
    }

    /** Constant-time comparison (`MessageDigest.isEqual`). */
    fun same(a: ByteArray, b: ByteArray): Boolean = MessageDigest.isEqual(a, b)

    fun hexToBytes(hex: String): ByteArray? {
        if (hex.length % 2 != 0 || !hex.all { it in '0'..'9' || it in 'a'..'f' || it in 'A'..'F' }) return null
        return ByteArray(hex.length / 2) { hex.substring(it * 2, it * 2 + 2).toInt(16).toByte() }
    }

    /** The verifier of the normal PIN: PBKDF2 over the PIN with the server's hex salt (`OfflineProfile.pinSalt`). */
    fun deriveLocal(pin: CharArray, saltHex: String, iterations: Int = OFFLINE_PIN_ITERATIONS): LocalPinVerifier? {
        val salt = hexToBytes(saltHex) ?: return null
        return LocalPinVerifier(saltHex, iterations, Base64Url.encode(pbkdf2(pin, salt, iterations)))
    }

    /** Whether [pin] matches a verifier of the server (separate offline PIN, base64url salt). */
    fun matchesServer(pin: CharArray, verifier: OfflinePinVerifier): Boolean {
        val expected = Base64Url.decode(verifier.hash) ?: return false
        val salt = Base64Url.decode(verifier.salt) ?: return false
        if (expected.isEmpty() || verifier.iterations < 1) return false
        return same(pbkdf2(pin, salt, verifier.iterations, expected.size), expected)
    }

    /** Whether [pin] matches the device's verifier of the normal PIN. */
    fun matchesLocal(pin: CharArray, verifier: LocalPinVerifier): Boolean {
        val expected = Base64Url.decode(verifier.hash) ?: return false
        val salt = hexToBytes(verifier.salt) ?: return false
        if (expected.isEmpty() || verifier.iterations < 1) return false
        return same(pbkdf2(pin, salt, verifier.iterations, expected.size), expected)
    }

    /** What opening a profile offline takes. */
    enum class Requirement {
        None,
        OfflinePin,
        Pin,

        /** A PIN without a verifier on this device: it has to be entered online once. */
        Unavailable,
    }

    fun requirement(profile: StoredProfile): Requirement {
        val snapshot = profile.snapshot
        if (snapshot.offlinePin != null) return Requirement.OfflinePin
        if (!snapshot.hasPin) return Requirement.None
        val verifier = profile.pinVerifier
        // A new salt means the PIN changed: the old verifier is void.
        if (verifier != null && snapshot.pinSalt != null && verifier.salt == snapshot.pinSalt) return Requirement.Pin
        return Requirement.Unavailable
    }

    /** Checks [pin] against the verifier [profile] requires. */
    fun verify(profile: StoredProfile, pin: CharArray): Boolean = when (requirement(profile)) {
        Requirement.None -> true
        Requirement.OfflinePin -> profile.snapshot.offlinePin?.let { matchesServer(pin, it) } ?: false
        Requirement.Pin -> profile.pinVerifier?.let { matchesLocal(pin, it) } ?: false
        Requirement.Unavailable -> false
    }

    /** Pause after [failures] wrong PINs: none for two, 30 s after the third, doubling, at most 15 min. */
    fun lockDelayMs(failures: Int): Long {
        if (failures <= FREE_FAILURES) return 0
        val steps = (failures - FREE_FAILURES - 1).coerceAtMost(30)
        return (FIRST_LOCK_MS shl steps).coerceAtMost(MAX_LOCK_MS)
    }

    /** The brake after one more failure at device time [now], clock since boot [elapsed] of boot [boot]. */
    fun registerFailure(lock: PinLock, now: Long, elapsed: Long, boot: Int): PinLock {
        val failures = lock.failures + 1
        val delay = lockDelayMs(failures)
        return if (delay > 0) PinLock(failures, now + delay, elapsed + delay, boot) else PinLock(failures, 0, 0, boot)
    }

    /**
     * Milliseconds until the next attempt is accepted (0: now). The longer of the wall clock and,
     * within the same boot, the clock since boot: moving the date forward does not end a pause.
     */
    fun remainingMs(lock: PinLock, now: Long, elapsed: Long, boot: Int): Long {
        val wall = lock.until - now
        val sinceBoot = if (lock.boot == boot && lock.untilElapsed > 0) lock.untilElapsed - elapsed else 0
        // A wall clock set back would stretch the pause beyond its length: cap it.
        val capped = wall.coerceAtMost(lockDelayMs(lock.failures))
        return maxOf(capped, sinceBoot, 0)
    }
}
