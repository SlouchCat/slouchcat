package com.slouchcat.core.api.crypto

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** The shared vectors of Go and Kotlin (testdata/crypto, format slouchcat-crypto-vectors-v1). */
class CryptoVectorsTest {
    private fun cases(name: String): List<JsonObject> {
        val dir = System.getProperty("slouchcat.vectors").orEmpty()
        val root = Json.parseToJsonElement(File(dir, name).readText()).jsonObject
        assertEquals("slouchcat-crypto-vectors-v1", root["format"]!!.jsonPrimitive.content)
        return root["cases"]!!.jsonArray.map { it.jsonObject }
    }

    private fun JsonObject.text(key: String): String = get(key)!!.jsonPrimitive.content

    private fun JsonObject.bytes(key: String): ByteArray = Base64Url.decode(text(key))!!

    @Test
    fun ed25519VectorsOfTheServer() {
        val all = cases("ed25519.json")
        assertTrue(all.size >= 2)
        for (case in all) {
            val input = case["input"]!!.jsonObject
            val expected = case["expected"]!!.jsonObject["valid"]!!.jsonPrimitive.boolean
            val name = case.text("name")
            assertEquals(name, expected, Ed25519.verify(input.bytes("publicKey"), input.bytes("message"), input.bytes("signature")))
            // The message is prefix and nonce; ServerProof builds the same bytes and decides the same.
            if (input.containsKey("nonce")) {
                assertArrayEquals(name, input.bytes("message"), ServerProof.message(input.bytes("nonce")))
                assertEquals(name, expected, ServerProof.valid(input.text("publicKey"), input.bytes("nonce"), input.text("signature")))
            }
        }
    }

    @Test
    fun brokenInputIsNeverValid() {
        assertFalse(Ed25519.verify(ByteArray(31), ByteArray(0), ByteArray(64)))
        assertFalse(Ed25519.verify(ByteArray(32), ByteArray(0), ByteArray(63)))
        assertFalse(ServerProof.valid("not base64!", ByteArray(32), "AAAA"))
        assertEquals(32, ServerProof.nonce().size)
        assertEquals("AQID", Base64Url.encode(byteArrayOf(1, 2, 3)))
        // SHA-256 of 32 zero bytes starts with 66687aadf862bd776c8fc18b8e9f8e20.
        assertEquals("6668 7aad f862 bd77 6c8f c18b 8e9f 8e20", ServerProof.fingerprint(Base64Url.encode(ByteArray(32))))
        assertEquals(null, ServerProof.fingerprint("not base64!"))
    }
}
