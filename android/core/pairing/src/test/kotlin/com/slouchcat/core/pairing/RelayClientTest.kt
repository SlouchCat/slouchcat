package com.slouchcat.core.pairing

import com.slouchcat.core.api.crypto.Base64Url
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.net.ServerSocket
import java.time.Instant

/** The relay's answers as the client reads them (relay/cmd/pair, docs/relay/BETRIEB.md). */
class RelayClientTest {
    private val server = MockWebServer()
    private val http = OkHttpClient()
    private lateinit var relay: RelayClient
    private val id = Base64Url.encode(ByteArray(16) { 1 })

    @Before
    fun start() {
        server.start()
        relay = RelayClient(http, server.url("/"))
    }

    @After
    fun stop() = server.close()

    @Test
    fun opensAMeetingPointWithTheHashOfTheSecretOnly() = runTest {
        server.enqueue(MockResponse(code = 201, body = """{"id":"$id","expiresAt":"2026-11-21T10:05:00Z"}"""))
        val secret = PickupSecret.generate()
        val opened = relay.open(secret)
        assertEquals(RelayClient.Open.Ok(id, Instant.parse("2026-11-21T10:05:00Z")), opened)
        val sent = server.takeRequest()
        assertEquals("POST", sent.method)
        assertEquals("/v1/pair", sent.url.encodedPath)
        val body = sent.body!!.utf8()
        assertTrue(body.contains(secret.hash))
        assertFalse(body.contains(secret.authorization.removePrefix("Pickup ")))
    }

    @Test
    fun aFullOrBusyRelayAndBrokenAnswersAreNotAMeetingPoint() = runTest {
        server.enqueue(MockResponse(code = 429, body = """{"error":"too_many_open"}"""))
        server.enqueue(MockResponse(code = 503, body = """{"error":"full"}"""))
        server.enqueue(MockResponse(code = 201, body = """{"id":"short","expiresAt":"2026-11-21T10:05:00Z"}"""))
        server.enqueue(MockResponse(code = 201, body = "not json"))
        server.enqueue(MockResponse(code = 400, body = """{"error":"bad_pickup_hash"}"""))
        val secret = PickupSecret.generate()
        assertEquals(RelayClient.Open.Busy, relay.open(secret))
        assertEquals(RelayClient.Open.Busy, relay.open(secret))
        assertEquals(RelayClient.Open.Failed(201), relay.open(secret))
        assertEquals(RelayClient.Open.Failed(201), relay.open(secret))
        assertEquals(RelayClient.Open.Failed(400), relay.open(secret))
    }

    @Test
    fun noRelayIsUnreachable() = runTest {
        val port = ServerSocket(0).use { it.localPort }
        val gone = RelayClient(http, "http://127.0.0.1:$port/".toHttpUrl())
        assertEquals(RelayClient.Open.Unreachable, gone.open(PickupSecret.generate()))
        assertEquals(RelayClient.Poll.Unreachable, gone.poll(id, PickupSecret.generate()))
        assertEquals(RelayClient.Put.Unreachable, gone.put(id, ByteArray(48)))
        assertFalse(gone.cancel(id, PickupSecret.generate()))
    }

    @Test
    fun pollsWithTheSecretAndReadsEveryAnswer() = runTest {
        val packet = ByteArray(80) { it.toByte() }
        server.enqueue(MockResponse(code = 204))
        server.enqueue(MockResponse(code = 200, body = """{"packet":"${Base64Url.encode(packet)}"}"""))
        server.enqueue(MockResponse(code = 410, body = """{"error":"consumed"}"""))
        server.enqueue(MockResponse(code = 404, body = """{"error":"not_found"}"""))
        server.enqueue(MockResponse(code = 403, body = """{"error":"forbidden"}"""))
        server.enqueue(MockResponse(code = 401, body = """{"error":"pickup_secret_required"}"""))
        server.enqueue(MockResponse(code = 200, body = """{"packet":"${Base64Url.encode(ByteArray(PairingProtocol.MAX_PACKET_BYTES + 1))}"}"""))
        server.enqueue(MockResponse(code = 500))
        val secret = PickupSecret.generate()
        assertEquals(RelayClient.Poll.Waiting, relay.poll(id, secret))
        val got = relay.poll(id, secret)
        assertArrayEquals(packet, (got as RelayClient.Poll.Packet).sealed)
        assertEquals(RelayClient.Poll.Gone, relay.poll(id, secret))
        assertEquals(RelayClient.Poll.Gone, relay.poll(id, secret))
        assertEquals(RelayClient.Poll.Refused, relay.poll(id, secret))
        assertEquals(RelayClient.Poll.Refused, relay.poll(id, secret))
        assertEquals(RelayClient.Poll.Failed(200), relay.poll(id, secret))
        assertEquals(RelayClient.Poll.Failed(500), relay.poll(id, secret))
        val first = server.takeRequest()
        assertEquals("GET", first.method)
        assertEquals("/v1/pair/$id", first.url.encodedPath)
        assertEquals(secret.authorization, first.headers["Authorization"])
    }

    @Test
    fun thePhoneLaysThePacketDownAndTheTvCancels() = runTest {
        server.enqueue(MockResponse(code = 204))
        server.enqueue(MockResponse(code = 409, body = """{"error":"occupied"}"""))
        server.enqueue(MockResponse(code = 410, body = """{"error":"consumed"}"""))
        server.enqueue(MockResponse(code = 404, body = """{"error":"not_found"}"""))
        server.enqueue(MockResponse(code = 413, body = """{"error":"too_large"}"""))
        server.enqueue(MockResponse(code = 204))
        server.enqueue(MockResponse(code = 403))
        val sealed = ByteArray(64) { 5 }
        assertEquals(RelayClient.Put.Delivered, relay.put(id, sealed))
        assertEquals(RelayClient.Put.Occupied, relay.put(id, sealed))
        assertEquals(RelayClient.Put.Consumed, relay.put(id, sealed))
        assertEquals(RelayClient.Put.Unknown, relay.put(id, sealed))
        assertEquals(RelayClient.Put.Failed, relay.put(id, sealed))
        val secret = PickupSecret.generate()
        assertTrue(relay.cancel(id, secret))
        assertFalse(relay.cancel(id, secret))
        val put = server.takeRequest()
        assertEquals("PUT", put.method)
        assertEquals("""{"packet":"${Base64Url.encode(sealed)}"}""", put.body!!.utf8())
        assertEquals(null, put.headers["Authorization"])
        repeat(4) { server.takeRequest() }
        val delete = server.takeRequest()
        assertEquals("DELETE", delete.method)
        assertEquals(secret.authorization, delete.headers["Authorization"])
    }
}
