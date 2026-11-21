package com.slouchcat.core.offline

import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.models.OfflinePinVerifier
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** The offline PIN against the shared vectors (`testdata/crypto/pbkdf2.json`) and its brake (9.5, review L-5). */
class OfflinePinTest {
    private val cases: List<JsonObject> by lazy {
        val dir = System.getProperty("slouchcat.vectors") ?: error("slouchcat.vectors not set")
        Json.parseToJsonElement(File(dir, "pbkdf2.json").readText()).jsonObject["cases"]!!.jsonArray.map { it.jsonObject }
    }

    @Test
    fun `pbkdf2 gives the bytes of the Go vectors`() {
        assertTrue(cases.isNotEmpty())
        for (case in cases) {
            val input = case["input"]!!.jsonObject
            val pin = String(Base64Url.decode(input["password"]!!.jsonPrimitive.content)!!).toCharArray()
            val salt = Base64Url.decode(input["salt"]!!.jsonPrimitive.content)!!
            val hash = OfflinePin.pbkdf2(pin, salt, input["iterations"]!!.jsonPrimitive.int, input["length"]!!.jsonPrimitive.int)
            assertArrayEquals(case["name"]!!.jsonPrimitive.content, Base64Url.decode(case["expected"]!!.jsonObject["hash"]!!.jsonPrimitive.content), hash)
        }
    }

    @Test
    fun `a server verifier of the vectors accepts its PIN and refuses another`() {
        val case = cases.first()
        val input = case["input"]!!.jsonObject
        val verifier = OfflinePinVerifier(
            salt = input["salt"]!!.jsonPrimitive.content,
            iterations = input["iterations"]!!.jsonPrimitive.int,
            hash = case["expected"]!!.jsonObject["hash"]!!.jsonPrimitive.content,
        )
        val pin = String(Base64Url.decode(input["password"]!!.jsonPrimitive.content)!!)
        val profile = StoredProfile(1, ACCOUNT, profile(1, hasPin = true).copy(offlinePin = verifier), 0)
        assertEquals(OfflinePin.Requirement.OfflinePin, OfflinePin.requirement(profile))
        assertTrue(OfflinePin.verify(profile, pin.toCharArray()))
        assertFalse(OfflinePin.verify(profile, "000000".toCharArray()))
    }

    @Test
    fun `the device derives a verifier of the normal PIN with the hex salt`() {
        val salt = "00112233445566778899aabbccddeeff"
        val verifier = OfflinePin.deriveLocal("4711".toCharArray(), salt, iterations = 1000)!!
        val profile = StoredProfile(1, ACCOUNT, profile(1, hasPin = true, salt = salt), 0, pinVerifier = verifier)
        assertEquals(OfflinePin.Requirement.Pin, OfflinePin.requirement(profile))
        assertTrue(OfflinePin.verify(profile, "4711".toCharArray()))
        assertFalse(OfflinePin.verify(profile, "4712".toCharArray()))
        // A PIN without a verifier on this device cannot open offline.
        assertEquals(OfflinePin.Requirement.Unavailable, OfflinePin.requirement(profile.copy(pinVerifier = null)))
        assertEquals(OfflinePin.Requirement.None, OfflinePin.requirement(StoredProfile(2, ACCOUNT, profile(2), 0)))
    }

    @Test
    fun `no pause for two failures, 30 s after the third, doubling up to 15 minutes`() {
        assertEquals(0, OfflinePin.lockDelayMs(2))
        assertEquals(30_000, OfflinePin.lockDelayMs(3))
        assertEquals(60_000, OfflinePin.lockDelayMs(4))
        assertEquals(15 * 60_000L, OfflinePin.lockDelayMs(40))
    }

    @Test
    fun `moving the wall clock forward does not end a pause within the same boot`() {
        var lock = PinLock()
        repeat(3) { lock = OfflinePin.registerFailure(lock, now = 1_000, elapsed = 500, boot = 7) }
        assertEquals(30_000, OfflinePin.remainingMs(lock, now = 1_000, elapsed = 500, boot = 7))
        // Date set a day ahead, clock since boot 1 s later: still locked.
        assertEquals(29_000, OfflinePin.remainingMs(lock, now = 1_000 + 86_400_000, elapsed = 1_500, boot = 7))
        // A date set back does not stretch the pause beyond its length.
        assertEquals(30_000, OfflinePin.remainingMs(lock, now = -1_000_000, elapsed = 500, boot = 9))
        assertEquals(0, OfflinePin.remainingMs(lock, now = 40_000, elapsed = 40_000, boot = 7))
    }

    @Test
    fun `neither a verifier nor a profile prints its PIN material`() {
        val verifier = LocalPinVerifier("00112233445566778899aabbccddeeff", 1000, "secret-hash")
        assertFalse(verifier.toString().contains("secret-hash"))
        assertFalse(verifier.toString().contains("0011"))
        val profile = StoredProfile(1, ACCOUNT, profile(1, hasPin = true, salt = "00112233445566778899aabbccddeeff"), 0, pinVerifier = verifier)
        assertFalse(profile.toString().contains("secret-hash"))
        assertFalse(profile.toString().contains("0011"))
    }
}
