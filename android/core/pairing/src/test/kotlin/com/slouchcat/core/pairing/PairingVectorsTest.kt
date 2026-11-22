package com.slouchcat.core.pairing

import com.google.crypto.tink.subtle.Ed25519Sign
import com.google.crypto.tink.subtle.X25519
import com.slouchcat.core.api.crypto.Base64Url
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.security.GeneralSecurityException

/**
 * The shared vectors of Go and Kotlin (testdata/crypto/hpke.json and pickup.json, the same files
 * the server and the relay test against) and round trips of what the phone seals and the TV opens.
 */
class PairingVectorsTest {
    private fun cases(name: String): List<JsonObject> {
        val dir = System.getProperty("slouchcat.vectors").orEmpty()
        val root = Json.parseToJsonElement(File(dir, name).readText()).jsonObject
        assertEquals("slouchcat-crypto-vectors-v1", root["format"]!!.jsonPrimitive.content)
        return root["cases"]!!.jsonArray.map { it.jsonObject }
    }

    private fun JsonObject.text(key: String): String = get(key)!!.jsonPrimitive.content

    private fun JsonObject.bytes(key: String): ByteArray = Base64Url.decode(text(key))!!

    @Test
    fun hpkeVectorsOfTheSuiteOpenAsInGo() {
        // The suite of the build log only: AES-256-GCM, empty aad. The case of RFC 9180 A.1.1 uses
        // AES-128-GCM and an aad, which the one-shot API of the apps never needs.
        val ours = cases("hpke.json").filter { it["input"]!!.jsonObject.let { i -> i["aead"]!!.jsonPrimitive.int == 2 && i.text("aad").isEmpty() } }
        assertEquals(3, ours.size)
        for (case in ours) {
            val input = case["input"]!!.jsonObject
            val name = case.text("name")
            assertEquals(name, 32, input["kem"]!!.jsonPrimitive.int)
            val keys = TvKeyPair.of(input.bytes("recipientPrivateKey"))
            assertArrayEquals(name, input.bytes("recipientPublicKey"), keys.publicKey)
            val plain = keys.open(input.bytes("info"), input.bytes("sealed"))
            assertArrayEquals(name, case["expected"]!!.jsonObject.bytes("plaintext"), plain)
        }
    }

    @Test
    fun thePacketAndTheSealedTokenOfTheVectorsReadAsTheirTypes() {
        val all = cases("hpke.json")
        val packetCase = all.first { it.text("name").contains("slouchcat-pair-v1") }["input"]!!.jsonObject
        val info = packetCase.bytes("info").decodeToString()
        assertTrue(info.startsWith(PairingProtocol.PACKET_INFO_PREFIX))
        val id = info.removePrefix(PairingProtocol.PACKET_INFO_PREFIX)
        val keys = TvKeyPair.of(packetCase.bytes("recipientPrivateKey"))
        val packet = PairingPacket.open(keys, id, packetCase.bytes("sealed"))
        assertNotNull(packet)
        assertEquals(1, packet!!.v)
        assertEquals("http://192.0.2.10:8096", packet.server)
        assertEquals(43, packet.voucher.length)
        assertTrue(PairingProtocol.isKey(packet.serverKey!!))
        // Another meeting point does not open the same packet (the id is bound in the info).
        assertNull(PairingPacket.open(keys, "A".repeat(22), packetCase.bytes("sealed")))

        val tokenCase = all.first { it.text("name").contains("slouchcat-redeem-v1") }["input"]!!.jsonObject
        val answer = RedeemAnswer.open(TvKeyPair.of(tokenCase.bytes("recipientPrivateKey")), tokenCase.bytes("sealed"))
        assertEquals("owner@example.test", answer!!.accountEmail)
        assertEquals("dev_example", answer.deviceId)
        assertFalse(answer.toString().contains(answer.token))
        assertFalse(answer.toString().contains(answer.deviceId))
    }

    @Test
    fun thePickupHashIsTheOneOfGo() {
        for (case in cases("pickup.json")) {
            val secret = PickupSecret.of(case["input"]!!.jsonObject.text("secret"))!!
            assertEquals(case.text("name"), case["expected"]!!.jsonObject.text("hash"), secret.hash)
            assertEquals("PickupSecret([redacted])", secret.toString())
        }
        assertNull(PickupSecret.of("AAAA"))
        assertEquals(43, PickupSecret.generate().hash.length)
    }

    @Test
    fun whatThePhoneSealsTheTvOpensAndNothingElse() {
        val tv = TvKeyPair.generate()
        val id = Base64Url.encode(ByteArray(16) { it.toByte() })
        val packet = PairingPacket(server = "https://media.example.test", voucher = Base64Url.encode(ByteArray(32)), serverKey = Base64Url.encode(ByteArray(32) { 7 }))
        val sealed = packet.seal(tv.publicKey, id)
        assertEquals(packet, PairingPacket.open(tv, id, sealed))
        assertNull(PairingPacket.open(TvKeyPair.generate(), id, sealed))
        assertNull(PairingPacket.open(tv, Base64Url.encode(ByteArray(16)), sealed))
        assertNull(PairingPacket.open(tv, id, sealed.copyOf(20)))
        val broken = sealed.copyOf().also { it[it.size - 1] = (it[it.size - 1] + 1).toByte() }
        assertNull(PairingPacket.open(tv, id, broken))
        assertFalse(packet.toString().contains(packet.voucher))

        // Another version, or something that is no packet, is no packet either.
        val v2 = Hpke.seal(tv.publicKey, PairingPacket.info(id), """{"v":2,"server":"https://x.test","voucher":"x"}""".encodeToByteArray())
        assertNull(PairingPacket.open(tv, id, v2))
        assertNull(PairingPacket.open(tv, id, Hpke.seal(tv.publicKey, PairingPacket.info(id), "[]".encodeToByteArray())))

        val answer = RedeemAnswer("device-token", "dev_1", "owner@example.test")
        val sealedAnswer = Hpke.seal(tv.publicKey, PairingProtocol.REDEEM_INFO.encodeToByteArray(), Json.encodeToString(RedeemAnswer.serializer(), answer).encodeToByteArray())
        assertEquals(answer, RedeemAnswer.open(tv, sealedAnswer))
        // A packet is no redeem answer: the info differs.
        assertNull(RedeemAnswer.open(tv, sealed))

        assertTrue(tv.toString().contains("[redacted]"))
    }

    @Test(expected = GeneralSecurityException::class)
    fun aKeyOfTheWrongLengthSealsNothing() {
        Hpke.seal(ByteArray(31), ByteArray(0), ByteArray(0))
    }

    @Test
    fun theRedeemSignatureCoversPrefixAndSealedBytes() {
        val pair = Ed25519Sign.KeyPair.newKeyPair()
        val key = Base64Url.encode(pair.publicKey)
        val sealed = ByteArray(64) { it.toByte() }
        val signature = Base64Url.encode(Ed25519Sign(pair.privateKey).sign(RedeemSignature.message(sealed)))
        assertTrue(RedeemSignature.valid(key, sealed, signature))
        assertFalse(RedeemSignature.valid(key, sealed.copyOf().also { it[0] = 9 }, signature))
        assertFalse(RedeemSignature.valid(Base64Url.encode(Ed25519Sign.KeyPair.newKeyPair().publicKey), sealed, signature))
        assertFalse(RedeemSignature.valid("not base64!", sealed, signature))
        // Signed without the prefix: not the server's redeem signature.
        val bare = Base64Url.encode(Ed25519Sign(pair.privateKey).sign(sealed))
        assertFalse(RedeemSignature.valid(key, sealed, bare))
    }

    @Test
    fun theQrCodeReadsBackAndNothingElseReadsAsOne() {
        val key = Base64Url.encode(X25519.publicFromPrivate(X25519.generatePrivateKey()))
        val id = Base64Url.encode(ByteArray(16) { 3 })
        val code = PairingCode("https://pair.slouchcat.com".toHttpUrl(), id, key)
        assertEquals("https://pair.slouchcat.com/tv#v=1&i=$id&k=$key", code.text)
        assertEquals(code, PairingCode.parse(code.text))
        // A relay of a test, with a port and a path below it.
        val local = PairingCode("http://10.0.2.2:8180/relay/".toHttpUrl(), id, key)
        assertEquals("http://10.0.2.2:8180/relay/tv#v=1&i=$id&k=$key", local.text)
        assertEquals(local.copy(relay = "http://10.0.2.2:8180/relay".toHttpUrl()), PairingCode.parse(local.text))

        val bad = listOf(
            "https://pair.slouchcat.com/tv",
            "https://pair.slouchcat.com/tv#v=2&i=$id&k=$key",
            "https://pair.slouchcat.com/help#v=1&i=$id&k=$key",
            "https://pair.slouchcat.com/tv?x=1#v=1&i=$id&k=$key",
            "https://pair.slouchcat.com/tv#v=1&i=$id&k=${key.dropLast(1)}",
            "https://pair.slouchcat.com/tv#v=1&i=${id}A&k=$key",
            "https://pair.slouchcat.com/tv#v=1&i=$id&k=$key&x=1",
            "https://pair.slouchcat.com/tv#v=1&i=$id&k=${key.dropLast(1)}=",
            "ftp://pair.slouchcat.com/tv#v=1&i=$id&k=$key",
            "slouchcat:tv#v=1&i=$id&k=$key",
        )
        for (text in bad) assertNull(text, PairingCode.parse(text))
        assertFalse(code.toString().contains(key))
    }
}
