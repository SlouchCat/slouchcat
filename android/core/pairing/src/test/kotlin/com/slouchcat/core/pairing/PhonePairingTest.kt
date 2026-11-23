package com.slouchcat.core.pairing

import com.google.crypto.tink.subtle.Ed25519Sign
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.api.crypto.Base64Url
import com.slouchcat.core.api.generated.models.MyDevice
import com.slouchcat.core.api.generated.models.SessionKind
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import mockwebserver3.Dispatcher
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import mockwebserver3.RecordedRequest
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.File
import java.security.MessageDigest
import java.time.Instant
import java.time.OffsetDateTime

/**
 * The phone's side of the pairing (concept 27 section 10): what it reads from a scanned code, the
 * voucher, the packet at the relay with the three refusals, the wait for the TV; and the whole
 * pairing of phone and TV, both sides of :core:pairing, through a relay in memory that answers like
 * relay/cmd/pair.
 */
class PhonePairingTest {
    private val relayHost = MockWebServer()
    private lateinit var relayUrl: HttpUrl
    private val server = "http://192.0.2.10:8096".toHttpUrl()
    private val serverKeys = Ed25519Sign.KeyPair.newKeyPair()
    private val serverKey = Base64Url.encode(serverKeys.publicKey)
    private val start = Instant.parse("2026-11-22T10:00:00Z")
    private val id = Base64Url.encode(ByteArray(16) { (it * 3).toByte() })
    private val tv = TvKeyPair.generate()

    @Before
    fun up() {
        relayHost.start()
        relayUrl = relayHost.url("/")
    }

    @After
    fun down() = relayHost.close()

    private fun code(relay: HttpUrl = relayUrl) = PairingCode(relay, id, tv.publicText)

    /** A TV whose device session started [at] (after the vouchers of the tests by default). */
    private fun tvDevice(id: String, name: String = "Living room", at: String = "2026-11-22T10:00:30Z") = MyDevice(
        id = id,
        lastSeenAt = OffsetDateTime.parse(at),
        downloads = 0,
        current = false,
        name = name,
        type = MyDevice.Type.TV,
        sessionKind = SessionKind.DEVICE,
        signedInAt = OffsetDateTime.parse(at),
    )

    private fun phoneDevice(id: String) = tvDevice(id, "Pixel").copy(type = MyDevice.Type.PHONE, current = true, sessionKind = SessionKind.APP)

    /** The own server: a voucher for every key, «Meine Geräte» as the test sets it. */
    private class FakeServer : PairingServer {
        var voucher: ApiResult<Pair<String, Instant>> = ApiResult.Ok("V".repeat(43) to Instant.parse("2026-11-22T10:02:00Z"))
        var list: () -> ApiResult<List<MyDevice>> = { ApiResult.Ok(emptyList()) }
        val keys = mutableListOf<String>()
        var listed = 0

        override suspend fun voucher(tvKey: String): ApiResult<Pair<String, Instant>> = voucher.also { keys += tvKey }

        override suspend fun devices(): ApiResult<List<MyDevice>> = list().also { listed++ }
    }

    private val fake = FakeServer()

    private fun TestScope.phone(relay: HttpUrl = relayUrl) =
        PhonePairing(RelayClient(OkHttpClient(), relay), fake, now = { start.plusMillis(testScheduler.currentTime) })

    // --- what a scanned text is ---

    @Test
    fun aCodeOfTheTvAppForTheRelayInUseIsRead() {
        val relay = "https://pair.slouchcat.com".toHttpUrl()
        val text = code(relay).text
        assertEquals(ScannedCode.Read.Ok(code(relay)), ScannedCode.read(text, relay))
        // Blanks around it (a scanner may add a line break) do not matter; the relay with a slash neither.
        assertEquals(ScannedCode.Read.Ok(code(relay)), ScannedCode.read("$text\n", "https://pair.slouchcat.com/".toHttpUrl()))
        assertEquals("Ok", ScannedCode.read(text, relay).toString())
    }

    @Test
    fun anotherVersionIsNamedAsSuch() {
        val relay = "https://pair.slouchcat.com".toHttpUrl()
        val text = code(relay).text.replace("#v=1&", "#v=2&")
        assertEquals(ScannedCode.Read.OtherVersion, ScannedCode.read(text, relay))
        // In another order of the fields, or with other fields of a newer version, as well.
        val reordered = "https://pair.slouchcat.com/tv#k=${tv.publicText}&v=2&i=$id"
        assertEquals(ScannedCode.Read.OtherVersion, ScannedCode.read(reordered, relay))
        assertEquals(ScannedCode.Read.OtherVersion, ScannedCode.read("https://pair.slouchcat.com/tv#v=2", relay))
        assertEquals(ScannedCode.Read.NotACode, ScannedCode.read("https://pair.slouchcat.com/tv#v=x", relay))
    }

    @Test
    fun aCodeForAnotherRelayIsRefusedWithItsHostOnly() {
        val relay = "https://pair.slouchcat.com".toHttpUrl()
        val other = code("https://pair.example.test".toHttpUrl()).text
        assertEquals(ScannedCode.Read.OtherRelay("pair.example.test"), ScannedCode.read(other, relay))
        // The same host over plain HTTP, another port or another path is another relay as well.
        assertEquals(ScannedCode.Read.OtherRelay("pair.slouchcat.com"), ScannedCode.read(code("http://pair.slouchcat.com".toHttpUrl()).text, relay))
        assertEquals(ScannedCode.Read.OtherRelay("pair.slouchcat.com"), ScannedCode.read(code("https://pair.slouchcat.com:8443".toHttpUrl()).text, relay))
        assertEquals(ScannedCode.Read.OtherRelay("pair.slouchcat.com"), ScannedCode.read(code("https://pair.slouchcat.com/x/".toHttpUrl()).text, relay))
    }

    @Test
    fun everythingElseIsNoCode() {
        val relay = "https://pair.slouchcat.com".toHttpUrl()
        val good = code(relay).text
        val broken = listOf(
            "",
            "   ",
            "https://pair.slouchcat.com/",
            "https://example.test/login?next=evil",
            "slouchcat://pair?i=$id",
            "WIFI:S:home;T:WPA;P:secret;;",
            good.substringBefore('#'),
            good.replace("&k=", "&x="),
            good + "&extra=1",
            good.replace("/tv#", "/tv?x=1#"),
            good.replace("/tv#", "/other#"),
            good.dropLast(2),
            good.replace("#v=1&i=$id", "#v=1&i=${id.dropLast(1)}!"),
            good.replace("\u0000", "") + "\u0000",
            good + "x".repeat(ScannedCode.MAX_LENGTH),
            "javascript:alert(1)#v=1&i=$id&k=${tv.publicText}",
        )
        for (text in broken) assertEquals(text.take(60), ScannedCode.Read.NotACode, ScannedCode.read(text, relay))
    }

    // --- voucher ---

    @Test
    fun theVoucherIsForTheTvsKeyAndRemembersTheDevicesBefore() = runTest {
        fake.list = { ApiResult.Ok(listOf(phoneDevice("p"), tvDevice("old"))) }
        val issued = phone().issue(code()) as PhonePairing.Issue.Ok
        assertEquals(listOf(tv.publicText), fake.keys)
        assertEquals(setOf("p", "old"), issued.voucher.knownDevices)
        assertEquals("IssuedVoucher(expiresAt=2026-11-22T10:02:00Z, voucher=[redacted])", issued.voucher.toString())
        assertFalse(issued.toString().contains("VVVV"))
    }

    @Test
    fun theRefusalsOfTheServerHaveTheirOwnOutcome() = runTest {
        val cases = listOf(
            ApiResult.NotAllowed("primary_profile_required") to PhonePairing.Issue.PrimaryOnly,
            ApiResult.NotAllowed("app_session_required") to PhonePairing.Issue.NotAllowed("app_session_required"),
            ApiResult.Failed(429, "too_many_attempts", 30) to PhonePairing.Issue.TooManyAttempts(30),
            ApiResult.Failed(null, ApiCaller.SERVER_UNCONFIRMED) to PhonePairing.Issue.ServerUnconfirmed,
            ApiResult.Failed(422, "validation_failed") to PhonePairing.Issue.Failed,
            ApiResult.Offline to PhonePairing.Issue.Offline,
            ApiResult.SignedOut to PhonePairing.Issue.SessionEnded,
            ApiResult.AppTooOld to PhonePairing.Issue.SessionEnded,
        )
        for ((answer, outcome) in cases) {
            fake.voucher = answer
            assertEquals(answer.toString(), outcome, phone().issue(code()))
        }
        // Without the list of devices no voucher is asked for: the phone could not tell the new TV.
        fake.keys.clear()
        fake.list = { ApiResult.Offline }
        assertEquals(PhonePairing.Issue.Offline, phone().issue(code()))
        assertTrue(fake.keys.isEmpty())
    }

    // --- the packet at the relay ---

    private fun issued() = IssuedVoucher("h1jmtV4VqCWk7rjzeBnX1tak3NxbsMFdmU0esl9hTHs", start.plusSeconds(120), emptySet())

    @Test
    fun thePacketIsSealedForTheTvAndOnlyTheTvOpensIt() = runTest {
        relayHost.enqueue(MockResponse(code = 204))
        assertEquals(PhonePairing.Deliver.Delivered, phone().deliver(code(), issued(), server, serverKey))
        val put = relayHost.takeRequest()
        assertEquals("PUT", put.method)
        assertEquals("/v1/pair/$id", put.url.encodedPath)
        val body = put.body!!.utf8()
        // The relay sees ciphertext only: neither voucher nor server address nor key.
        assertFalse(body.contains(issued().voucher))
        assertFalse(body.contains("192.0.2.10"))
        assertFalse(body.contains(serverKey))
        val sealed = Base64Url.decode(Json.parseToJsonElement(body).jsonObject["packet"]!!.jsonPrimitive.content)!!
        val packet = PairingPacket.open(tv, id, sealed)!!
        assertEquals(PairingPacket(server = "http://192.0.2.10:8096", voucher = issued().voucher, serverKey = serverKey), packet)
        // Bound to the meeting point and the key: another TV or another meeting point opens nothing.
        assertEquals(null, PairingPacket.open(TvKeyPair.generate(), id, sealed))
        assertEquals(null, PairingPacket.open(tv, Base64Url.encode(ByteArray(16)), sealed))
    }

    @Test
    fun theThreeRefusalsOfTheRelay() = runTest {
        for ((status, outcome) in listOf(409 to PhonePairing.Deliver.Occupied, 410 to PhonePairing.Deliver.Occupied, 404 to PhonePairing.Deliver.Expired, 500 to PhonePairing.Deliver.Failed)) {
            relayHost.enqueue(MockResponse(code = status, body = """{"error":"x"}"""))
            assertEquals("$status", outcome, phone().deliver(code(), issued(), server, serverKey))
        }
    }

    @Test
    fun aRelayThatDoesNotAnswerIsUnreachable() = runTest {
        val gone = relayHost.url("/")
        relayHost.close()
        assertEquals(PhonePairing.Deliver.RelayUnreachable, phone(gone).deliver(code(gone), issued(), server, serverKey))
    }

    @Test
    fun aCodeWhoseKeyIsNoKeySealsNothing() = runTest {
        val bad = PairingCode(relayUrl, id, Base64Url.encode(ByteArray(31)))
        assertEquals(PhonePairing.Deliver.Failed, phone().deliver(bad, issued(), server, serverKey))
        assertEquals(0, relayHost.requestCount)
    }

    @Test
    fun theSealOfThePhoneOpensWithTheKeyOfTheSharedVector() {
        // The recipient of the Go vector «pair packet»: what Kotlin seals for it opens as the same packet.
        val dir = System.getProperty("slouchcat.vectors").orEmpty()
        val root = Json.parseToJsonElement(File(dir, "hpke.json").readText()).jsonObject
        val case = root["cases"]!!.jsonArray.map { it.jsonObject }.first { it["name"]!!.jsonPrimitive.content.startsWith("pair packet") }
        val input = case["input"]!!.jsonObject
        assertEquals(2, input["aead"]!!.jsonPrimitive.int)
        val keys = TvKeyPair.of(input.bytes("recipientPrivateKey"))
        val info = input.bytes("info").decodeToString()
        val meeting = info.removePrefix(PairingProtocol.PACKET_INFO_PREFIX)
        val expected = Json.decodeFromString(PairingPacket.serializer(), Base64Url.decode(case["expected"]!!.jsonObject["plaintext"]!!.jsonPrimitive.content)!!.decodeToString())
        val sealed = expected.seal(input.bytes("recipientPublicKey"), meeting)
        assertEquals(expected, PairingPacket.open(keys, meeting, sealed))
        // And the Go ciphertext opens as the packet the phone would have sealed.
        assertEquals(expected, PairingPacket.open(keys, meeting, input.bytes("sealed")))
    }

    private fun JsonObject.bytes(key: String): ByteArray = Base64Url.decode(get(key)!!.jsonPrimitive.content)!!

    // --- the wait for the TV ---

    @Test
    fun theNewTvIsTheOneThatWasNotThereBefore() = runTest {
        var round = 0
        fake.list = {
            round++
            ApiResult.Ok(
                when {
                    round < 3 -> listOf(phoneDevice("p"), tvDevice("old", "Kitchen"))
                    // A phone that signs in meanwhile is no TV.
                    round < 4 -> listOf(phoneDevice("p"), tvDevice("old", "Kitchen"), phoneDevice("p2"))
                    else -> listOf(phoneDevice("p"), tvDevice("old", "Kitchen"), phoneDevice("p2"), tvDevice("new"))
                },
            )
        }
        val linked = phone().awaitTv(IssuedVoucher("x", start, setOf("p", "old")))
        assertEquals("new", (linked as PhonePairing.Linked.Ok).device.id)
        assertEquals(4, fake.listed)
    }

    @Test
    fun aTvThatSignedInBeforeTheVoucherOrAnotherKindIsNotThisOne() = runTest {
        // Issued 10:00:00 (expires 10:02): a TV of 09:50 that the list missed before, a TV signed in by
        // hand (an app session), then this TV.
        val voucher = IssuedVoucher("x", start.plusSeconds(120), setOf("p"))
        var round = 0
        fake.list = {
            round++
            val earlier = listOf(tvDevice("early", at = "2026-11-22T09:50:00Z"), tvDevice("hand").copy(sessionKind = SessionKind.APP))
            ApiResult.Ok(if (round < 3) earlier else earlier + tvDevice("mine"))
        }
        assertEquals("mine", (phone().awaitTv(voucher) as PhonePairing.Linked.Ok).device.id)
    }

    @Test
    fun twoNewTvsAtOnceAreNoGuess() = runTest {
        fake.list = { ApiResult.Ok(listOf(tvDevice("a", "Kitchen"), tvDevice("b", "Living room"))) }
        assertEquals(PhonePairing.Linked.Several, phone().awaitTv(issued()))
    }

    @Test
    fun aSessionThatMayNoLongerAskEndsTheWait() = runTest {
        for (answer in listOf(ApiResult.SignedOut, ApiResult.AppTooOld, ApiResult.NotAllowed("client_not_allowed"))) {
            fake.list = { answer }
            fake.listed = 0
            assertEquals(answer.toString(), PhonePairing.Linked.Ended, phone().awaitTv(issued()))
            assertEquals(1, fake.listed)
        }
    }

    @Test
    fun theShortLookAfterARefusalEndsInTime() = runTest {
        fake.list = { ApiResult.Ok(emptyList()) }
        assertEquals(PhonePairing.Linked.TimedOut, phone().awaitTv(issued(), PhonePairing.LOOK_FOR_TV))
        assertEquals(PhonePairing.LOOK_FOR_TV.toMillis(), testScheduler.currentTime)
    }

    @Test
    fun afterTwoMinutesWithoutTheTvItDidNotWork() = runTest {
        fake.list = { if (fake.listed % 2 == 0) ApiResult.Offline else ApiResult.Ok(emptyList()) }
        assertEquals(PhonePairing.Linked.TimedOut, phone().awaitTv(issued()))
        assertEquals(PhonePairing.WAIT_FOR_TV.toMillis(), testScheduler.currentTime)
    }

    // --- phone and TV, both sides of :core:pairing, through a relay in memory ---

    /** The protocol of relay/cmd/pair in memory: one meeting point, its packet, its pickup hash. */
    private class MemoryRelay(private val expires: Instant) : Dispatcher() {
        var id: String? = null
        var pickupHash: String? = null
        var packet: String? = null
        var consumed = false
        val puts = mutableListOf<Int>()

        override fun dispatch(request: RecordedRequest): MockResponse {
            val path = request.url.encodedPath
            val auth = request.headers["Authorization"]?.removePrefix("Pickup ")
            val ownsIt = auth != null && Base64Url.encode(MessageDigest.getInstance("SHA-256").digest(Base64Url.decode(auth)!!)) == pickupHash
            return when {
                request.method == "POST" && path == "/v1/pair" -> {
                    pickupHash = Json.parseToJsonElement(request.body!!.utf8()).jsonObject["pickupHash"]!!.jsonPrimitive.content
                    id = Base64Url.encode(ByteArray(16) { 5 })
                    MockResponse(code = 201, body = """{"id":"$id","expiresAt":"$expires"}""")
                }
                request.method == "PUT" && path == "/v1/pair/$id" -> {
                    val status = when {
                        consumed -> 410
                        packet != null -> 409
                        else -> 204.also { packet = Json.parseToJsonElement(request.body!!.utf8()).jsonObject["packet"]!!.jsonPrimitive.content }
                    }
                    puts += status
                    MockResponse(code = status)
                }
                request.method == "PUT" -> MockResponse(code = 404)
                request.method == "GET" && path == "/v1/pair/$id" && ownsIt -> {
                    val p = packet
                    if (p == null) {
                        MockResponse(code = 204)
                    } else {
                        packet = null
                        consumed = true
                        MockResponse(code = 200, body = """{"packet":"$p"}""")
                    }
                }
                request.method == "DELETE" && ownsIt -> MockResponse(code = 204)
                else -> MockResponse(code = 403)
            }
        }
    }

    /** The user's server for the TV: seals the token for the TV's key and signs it, when the voucher is the phone's. */
    private inner class Redeems(private val voucher: String) : Redeemer {
        val servers = mutableListOf<String>()

        override suspend fun redeem(server: HttpUrl, serverKey: String, voucher: String, tvKey: String, deviceName: String, systemVersion: String): ApiResult<Redeemed> {
            servers += server.toString()
            if (voucher != this.voucher) return ApiResult.Failed(410, "voucher_invalid")
            val answer = RedeemAnswer("device-token", "dev_tv", "owner@example.test")
            val sealed = Hpke.seal(Base64Url.decode(tvKey)!!, PairingProtocol.REDEEM_INFO.encodeToByteArray(), Json.encodeToString(RedeemAnswer.serializer(), answer).encodeToByteArray())
            val signature = Ed25519Sign(serverKeys.privateKey).sign(RedeemSignature.message(sealed))
            return ApiResult.Ok(Redeemed(Base64Url.encode(sealed), Base64Url.encode(signature)))
        }

        override suspend fun discard(server: HttpUrl, serverKey: String, token: String) = ApiResult.Ok(Unit)
    }

    @Test
    fun aPhoneAndATvPairThroughTheRelay() = runTest {
        val relay = MemoryRelay(start.plusSeconds(300))
        relayHost.dispatcher = relay
        val voucher = "h1jmtV4VqCWk7rjzeBnX1tak3NxbsMFdmU0esl9hTHs"
        fake.voucher = ApiResult.Ok(voucher to start.plusSeconds(120))
        val redeems = Redeems(voucher)
        val tvSide = TvPairing(RelayClient(OkHttpClient(), relayUrl), redeems, now = { start.plusMillis(testScheduler.currentTime) })

        // The TV shows its code; the phone reads it from the scanner.
        val meeting = (tvSide.start() as TvPairing.Started.Ok).meeting
        val read = ScannedCode.read(meeting.code.text, relayUrl) as ScannedCode.Read.Ok
        assertEquals(PairingProtocol.VERSION, 1)
        assertTrue(tvSide.awaitPacket(meeting, until = start.plusSeconds(1)) is TvPairing.Waited.Expired)

        var redeemed = false
        fake.list = { ApiResult.Ok(listOf(phoneDevice("p")) + if (redeemed) listOf(tvDevice("dev_tv")) else emptyList()) }
        val phoneSide = phone()
        val issued = phoneSide.issue(read.code) as PhonePairing.Issue.Ok
        assertEquals(PhonePairing.Deliver.Delivered, phoneSide.deliver(read.code, issued.voucher, server, serverKey))

        // The TV takes the packet, redeems at the address the phone sent and opens its token.
        val packet = (tvSide.awaitPacket(meeting) as TvPairing.Waited.Packet).packet
        assertEquals("http://192.0.2.10:8096", packet.server)
        val result = tvSide.redeem(meeting, packet, packet.server.toHttpUrl(), "Living room", "Android TV 14")
        assertEquals("device-token", ((result as TvPairing.Redeem.Ok).answer.token))
        assertEquals(listOf("http://192.0.2.10:8096/"), redeems.servers)
        redeemed = true
        assertEquals("dev_tv", (phoneSide.awaitTv(issued.voucher) as PhonePairing.Linked.Ok).device.id)

        // A second phone (or the same one twice) finds the meeting point used up.
        assertEquals(PhonePairing.Deliver.Occupied, phoneSide.deliver(read.code, issued.voucher, server, serverKey))
        assertEquals(listOf(204, 410), relay.puts)
    }

    @Test
    fun aPacketLaidDownFirstByAnotherMakesThePhoneWarn() = runTest {
        val relay = MemoryRelay(start.plusSeconds(300))
        relayHost.dispatcher = relay
        val tvSide = TvPairing(RelayClient(OkHttpClient(), relayUrl), Redeems("x"), now = { start.plusMillis(testScheduler.currentTime) })
        val meeting = (tvSide.start() as TvPairing.Started.Ok).meeting
        val read = ScannedCode.read(meeting.code.text, relayUrl) as ScannedCode.Read.Ok
        // Whoever photographed the code lays a packet down first.
        val intruder = PairingPacket(server = "https://evil.example.test", voucher = "E".repeat(43), serverKey = serverKey)
        relay.packet = Base64Url.encode(intruder.seal(Base64Url.decode(read.code.tvKey)!!, read.code.id))
        assertEquals(PhonePairing.Deliver.Occupied, phone().deliver(read.code, issued(), server, serverKey))
        assertEquals(listOf(409), relay.puts)
    }
}
