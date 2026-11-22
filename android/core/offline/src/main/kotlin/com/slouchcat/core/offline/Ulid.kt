package com.slouchcat.core.offline

import java.security.SecureRandom

/** ULIDs for the ids the client creates (`OfflineEvent.playbackId`): 48 bits of time, 80 bits of chance, Crockford base 32. */
object Ulid {
    private const val ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
    private val random = SecureRandom()

    fun next(now: Long = System.currentTimeMillis()): String {
        val chars = CharArray(26)
        var time = now
        for (i in 9 downTo 0) {
            chars[i] = ALPHABET[(time and 31).toInt()]
            time = time shr 5
        }
        val bytes = ByteArray(10).also(random::nextBytes)
        var bits = 0
        var value = 0
        var at = 10
        for (b in bytes) {
            value = (value shl 8) or (b.toInt() and 0xff)
            bits += 8
            while (bits >= 5) {
                bits -= 5
                chars[at++] = ALPHABET[(value shr bits) and 31]
            }
        }
        return String(chars)
    }
}
