package com.slouchcat.core.api.push

import com.slouchcat.core.api.crypto.Base64Url
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.io.File
import java.util.Base64

/** Web-Push decryption against the shared vectors (testdata/crypto/webpush.json) and its refusals. */
class WebPushTest {
    private fun cases(): List<JsonObject> {
        val dir = System.getProperty("slouchcat.vectors").orEmpty()
        val root = Json.parseToJsonElement(File(dir, "webpush.json").readText()).jsonObject
        assertEquals("slouchcat-crypto-vectors-v1", root["format"]!!.jsonPrimitive.content)
        return root["cases"]!!.jsonArray.map { it.jsonObject }
    }

    private fun JsonObject.bytes(key: String): ByteArray = Base64Url.decode(get(key)!!.jsonPrimitive.content)!!

    private fun keys(input: JsonObject) = PushKeys(input.bytes("uaPrivateKey"), input.bytes("uaPublicKey"), input.bytes("authSecret"))

    private fun refused(body: ByteArray, keys: PushKeys) {
        try {
            WebPush.decrypt(body, keys)
            fail("decrypted what it must not")
        } catch (e: WebPushException) {
            // expected
        }
    }

    @Test
    fun everyVectorDecrypts() {
        val all = cases()
        assertEquals(3, all.size)
        for (case in all) {
            val input = case["input"]!!.jsonObject
            val plain = WebPush.decrypt(input.bytes("body"), keys(input))
            assertArrayEquals(case["name"]!!.jsonPrimitive.content, case["expected"]!!.jsonObject.bytes("plaintext"), plain)
        }
        // The 2700 vector is what the push relay carries: the body is exactly the record size.
        val relay = all.single { it["input"]!!.jsonObject["recordSize"]!!.jsonPrimitive.int == 2700 }["input"]!!.jsonObject
        assertEquals(2700, relay.bytes("body").size)
    }

    @Test
    fun aWrongKeyACutMessageAndATooLargeOneAreRefused() {
        val input = cases().first()["input"]!!.jsonObject
        val body = input.bytes("body")
        val right = keys(input)
        val other = WebPush.generate()
        refused(body, PushKeys(other.privateKey, right.publicKey, right.auth))
        refused(body, PushKeys(right.privateKey, right.publicKey, ByteArray(16)))
        refused(body.copyOf(body.size - 1), right)
        refused(body.copyOf(40), right)
        refused(ByteArray(0), right)
        refused(body + ByteArray(WebPush.MAX_BODY), right)
        val tampered = body.copyOf().also { it[it.size - 3] = (it[it.size - 3].toInt() xor 1).toByte() }
        refused(tampered, right)
    }

    @Test
    fun freshKeysHaveTheShapeOfASubscription() {
        val keys = WebPush.generate()
        assertEquals(65, keys.publicKey.size)
        assertEquals(87, WebPush.p256dh(keys).length)
        assertEquals(22, WebPush.auth(keys).length)
        assertEquals("PushKeys(…)", keys.toString())
    }

    @Test
    fun theDataFieldAndThePayload() {
        val input = cases()[1]["input"]!!.jsonObject
        val body = input.bytes("body")
        assertArrayEquals(body, PushMessage.body(Base64.getEncoder().encodeToString(body)))
        assertArrayEquals(body, PushMessage.body(Base64.getUrlEncoder().withoutPadding().encodeToString(body)))
        assertNull(PushMessage.body("not base64 !"))
        assertNull(PushMessage.body(Base64.getEncoder().encodeToString(ByteArray(WebPush.MAX_BODY + 1))))
        val message = PushMessage.parse("""{"title":"Mara: Neue Folge","body":"S01E02","url":"/items/01J"}""".toByteArray())!!
        assertEquals("Mara: Neue Folge", message.title)
        assertEquals("/items/01J", message.url)
        assertNull(PushMessage.parse("[1]".toByteArray()))
        assertNull(PushMessage.parse("""{"body":"no title"}""".toByteArray()))
    }

    @Test
    fun onlyTheOwnServersPathsAreOpened() {
        val server = "http://192.0.2.10:8096/".toHttpUrl()
        assertEquals("/items/42", PushMessage.path("/items/42", server))
        assertEquals("/wishes?focus=3", PushMessage.path("http://192.0.2.10:8096/wishes?focus=3", server))
        assertEquals("/items/42", PushMessage.path("https://media.example.test/sub/items/42", "https://media.example.test/sub/".toHttpUrl()))
        assertNull(PushMessage.path("https://evil.example.test/items/42", server))
        assertNull(PushMessage.path("//evil.example.test/x", server))
        assertNull(PushMessage.path("javascript:alert(1)", server))
        assertNull(PushMessage.path(null, server))
    }

    @Test
    fun theRelayGivesItsOwnAddressOnly() {
        MockWebServer().use { relay ->
            relay.start()
            val base = relay.url("/")
            val client = PushRelay(base, OkHttpClient())
            relay.enqueue(MockResponse(code = 200, body = """{"endpoint":"${base}v1/push/1.abc"}"""))
            assertEquals(RelayResult.Registered("${base}v1/push/1.abc"), client.register("token", "BKEY"))
            val request = relay.takeRequest()
            assertEquals("/v1/register", request.url.encodedPath)
            assertTrue(request.body!!.utf8().contains("\"fcmToken\":\"token\""))
            relay.enqueue(MockResponse(code = 200, body = """{"endpoint":"https://elsewhere.example.test/v1/push/1.abc"}"""))
            assertTrue(client.register("token", "BKEY") is RelayResult.Failed)
            relay.enqueue(MockResponse(code = 503))
            assertEquals(RelayResult.Failed(503), client.register("token", "BKEY"))
        }
    }
}
