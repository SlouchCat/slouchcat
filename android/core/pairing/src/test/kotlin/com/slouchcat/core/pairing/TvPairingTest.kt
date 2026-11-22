package com.slouchcat.core.pairing

import com.google.crypto.tink.subtle.Ed25519Sign
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.crypto.Base64Url
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.time.Instant

/** The TV's whole pairing against a relay that answers like relay/cmd/pair and a fake server. */
class TvPairingTest {
    private val relayServer = MockWebServer()
    private val serverKeys = Ed25519Sign.KeyPair.newKeyPair()
    private val serverKey = Base64Url.encode(serverKeys.publicKey)
    private val server = "https://media.example.test".toHttpUrl()
    private val start = Instant.parse("2026-11-21T10:00:00Z")

    /** The server: seals [answer] for the TV's key and signs it, or answers [result]. */
    private inner class FakeServer : Redeemer {
        var result: ApiResult<Redeemed>? = null
        var signWith: Ed25519Sign? = Ed25519Sign(serverKeys.privateKey)
        val answer = RedeemAnswer("device-token-1", "dev_1", "owner@example.test")
        val requests = mutableListOf<List<String>>()
        val discarded = mutableListOf<String>()

        override suspend fun redeem(server: HttpUrl, serverKey: String, voucher: String, tvKey: String, deviceName: String, systemVersion: String): ApiResult<Redeemed> {
            requests += listOf(server.toString(), serverKey, voucher, tvKey, deviceName, systemVersion)
            result?.let { return it }
            val sealed = Hpke.seal(Base64Url.decode(tvKey)!!, PairingProtocol.REDEEM_INFO.encodeToByteArray(), Json.encodeToString(RedeemAnswer.serializer(), answer).encodeToByteArray())
            val signature = signWith?.sign(RedeemSignature.message(sealed)) ?: ByteArray(64)
            return ApiResult.Ok(Redeemed(Base64Url.encode(sealed), Base64Url.encode(signature)))
        }

        override suspend fun discard(server: HttpUrl, serverKey: String, token: String): ApiResult<Unit> {
            discarded += token
            return ApiResult.Ok(Unit)
        }
    }

    private val fake = FakeServer()

    @Before
    fun up() = relayServer.start()

    @After
    fun down() = relayServer.close()

    private fun TestScope.pairing() = TvPairing(
        RelayClient(OkHttpClient(), relayServer.url("/")),
        fake,
        now = { start.plusMillis(testScheduler.currentTime) },
    )

    private fun opened(seconds: Long = 300) =
        MockResponse(code = 201, body = """{"id":"${Base64Url.encode(ByteArray(16) { 9 })}","expiresAt":"${start.plusSeconds(seconds)}"}""")

    private fun packetFor(meeting: TvPairing.Meeting, packet: PairingPacket = PairingPacket(server = server.toString(), voucher = "V".repeat(43), serverKey = serverKey)) =
        MockResponse(code = 200, body = """{"packet":"${Base64Url.encode(packet.seal(Base64Url.decode(meeting.code.tvKey)!!, meeting.code.id))}"}""")

    @Test
    fun fromTheQrCodeToTheToken() = runTest {
        val pairing = pairing()
        relayServer.enqueue(opened())
        val meeting = (pairing.start() as TvPairing.Started.Ok).meeting
        assertEquals(relayServer.url("/"), meeting.code.relay)
        assertEquals(PairingCode.parse(meeting.code.text), meeting.code)
        relayServer.enqueue(MockResponse(code = 204))
        relayServer.enqueue(MockResponse(code = 204))
        relayServer.enqueue(packetFor(meeting))
        val waited = pairing.awaitPacket(meeting) as TvPairing.Waited.Packet
        // Two empty answers, two pauses of two seconds (virtual time).
        assertEquals(4_000L, testScheduler.currentTime)
        val redeemed = pairing.redeem(meeting, waited.packet, server, "Fernseher · Android TV 14", "Android TV 14")
        assertEquals(TvPairing.Redeem.Ok(fake.answer), redeemed)
        assertEquals(listOf(server.toString(), serverKey, "V".repeat(43), meeting.code.tvKey, "Fernseher · Android TV 14", "Android TV 14"), fake.requests.single())
        relayServer.enqueue(MockResponse(code = 204))
        pairing.close(meeting)
        val requests = List(5) { relayServer.takeRequest() }
        assertEquals(listOf("POST", "GET", "GET", "GET", "DELETE"), requests.map { it.method })
    }

    @Test
    fun anAnswerNotSignedByThePacketsKeyIsThrownAway() = runTest {
        val pairing = pairing()
        relayServer.enqueue(opened())
        val meeting = (pairing.start() as TvPairing.Started.Ok).meeting
        relayServer.enqueue(packetFor(meeting))
        val packet = (pairing.awaitPacket(meeting) as TvPairing.Waited.Packet).packet
        fake.signWith = Ed25519Sign(Ed25519Sign.KeyPair.newKeyPair().privateKey)
        assertEquals(TvPairing.Redeem.Untrusted, pairing.redeem(meeting, packet, server, "TV", "Android TV 14"))
        fake.signWith = null
        assertEquals(TvPairing.Redeem.Untrusted, pairing.redeem(meeting, packet, server, "TV", "Android TV 14"))
        // A packet without the server's key cannot be checked: nothing goes to the server.
        val before = fake.requests.size
        assertEquals(TvPairing.Redeem.Untrusted, pairing.redeem(meeting, packet.copy(serverKey = null), server, "TV", "Android TV 14"))
        assertEquals(before, fake.requests.size)
    }

    @Test
    fun theServersRefusalsBecomeWhatTheTvShows() = runTest {
        val pairing = pairing()
        relayServer.enqueue(opened())
        val meeting = (pairing.start() as TvPairing.Started.Ok).meeting
        val packet = PairingPacket(server = server.toString(), voucher = "V".repeat(43), serverKey = serverKey)
        val cases = mapOf(
            ApiResult.Failed(410, "voucher_invalid") to TvPairing.Redeem.VoucherInvalid,
            ApiResult.Failed(429, "too_many_attempts", 30) to TvPairing.Redeem.TooManyAttempts(30),
            ApiResult.Failed(null, "server_unconfirmed") to TvPairing.Redeem.ServerUnreachable,
            ApiResult.Offline to TvPairing.Redeem.ServerUnreachable,
            ApiResult.AppTooOld to TvPairing.Redeem.AppTooOld,
            ApiResult.Failed(422, "validation_failed") to TvPairing.Redeem.Failed,
            ApiResult.NotAllowed("csrf_header_missing") to TvPairing.Redeem.Failed,
        )
        for ((result, expected) in cases) {
            fake.result = result
            assertEquals(result.toString(), expected, pairing.redeem(meeting, packet, server, "TV", "Android TV 14"))
        }
    }

    @Test
    fun aCodeExpiresAndASilentRelayIsReported() = runTest {
        val pairing = pairing()
        relayServer.enqueue(opened(seconds = 6))
        val meeting = (pairing.start() as TvPairing.Started.Ok).meeting
        repeat(3) { relayServer.enqueue(MockResponse(code = 204)) }
        assertEquals(TvPairing.Waited.Expired, pairing.awaitPacket(meeting))
        assertEquals(6_000L, testScheduler.currentTime)

        relayServer.enqueue(opened(seconds = 400))
        val next = (pairing.start() as TvPairing.Started.Ok).meeting
        // Renewed before the expiry: the caller asks only until then.
        relayServer.enqueue(MockResponse(code = 204))
        assertEquals(TvPairing.Waited.Expired, pairing.awaitPacket(next, until = start.plusMillis(testScheduler.currentTime + 1)))

        repeat(3) { relayServer.enqueue(MockResponse(code = 502)) }
        assertEquals(TvPairing.Waited.RelayUnreachable, pairing.awaitPacket(next))
        relayServer.enqueue(MockResponse(code = 410, body = """{"error":"consumed"}"""))
        assertEquals(TvPairing.Waited.Expired, pairing.awaitPacket(next))
        relayServer.enqueue(MockResponse(code = 403, body = """{"error":"forbidden"}"""))
        assertEquals(TvPairing.Waited.Invalid, pairing.awaitPacket(next))
    }

    @Test
    fun aPacketForAnotherKeyIsNoPacket() = runTest {
        val pairing = pairing()
        relayServer.enqueue(opened())
        val meeting = (pairing.start() as TvPairing.Started.Ok).meeting
        val other = TvKeyPair.generate()
        val foreign = PairingPacket(server = server.toString(), voucher = "V".repeat(43), serverKey = serverKey).seal(other.publicKey, meeting.code.id)
        relayServer.enqueue(MockResponse(code = 200, body = """{"packet":"${Base64Url.encode(foreign)}"}"""))
        assertEquals(TvPairing.Waited.Invalid, pairing.awaitPacket(meeting))
    }

    @Test
    fun aRelayThatCannotOpenAMeetingPointSaysWhy() = runTest {
        val pairing = pairing()
        relayServer.enqueue(MockResponse(code = 503, body = """{"error":"full"}"""))
        relayServer.enqueue(MockResponse(code = 500))
        assertEquals(TvPairing.Started.RelayBusy, pairing.start())
        assertEquals(TvPairing.Started.Failed, pairing.start())
        relayServer.close()
        assertEquals(TvPairing.Started.RelayUnreachable, pairing.start())
        assertTrue(fake.requests.isEmpty())
    }
}
