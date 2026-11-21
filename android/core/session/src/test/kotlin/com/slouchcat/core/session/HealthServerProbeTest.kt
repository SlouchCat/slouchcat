package com.slouchcat.core.session

import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatHttp
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class HealthServerProbeTest {
    private val server = MockWebServer()
    private val probe = HealthServerProbe(SlouchcatHttp.client(ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0")))

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    private suspend fun answer(response: MockResponse): ProbeResult {
        server.enqueue(response)
        return probe.probe(server.url("/"))
    }

    @Test
    fun aCurrentServerIsFound() = runTest {
        val result = answer(MockResponse(body = """{"status":"ok","version":"1.42.0","contractVersion":"1.42.0","uptimeSeconds":5,"serverKey":"x"}"""))
        val info = (result as ProbeResult.Found).info
        assertEquals("1.42.0", info.version)
        assertFalse(info.encrypted)
        val request = server.takeRequest()
        assertEquals("/api/v1/healthz", request.url.encodedPath)
        assertEquals("mobile/0.1.0", request.headers[SlouchcatHttp.HEADER_CLIENT])
    }

    @Test
    fun withoutContractVersionTheServerIsTooOld() = runTest {
        val result = answer(MockResponse(body = """{"status":"ok","version":"1.41.3","uptimeSeconds":5}"""))
        assertEquals(ProbeResult.TooOld(serverVersion = "1.41.3", required = Compatibility.MIN_CONTRACT), result)
        val older = answer(MockResponse(body = """{"status":"ok","version":"1.41.9","contractVersion":"1.41.9","uptimeSeconds":5}"""))
        assertEquals(ProbeResult.TooOld("1.41.9", Compatibility.MIN_CONTRACT), older)
    }

    @Test
    fun otherAnswersAreNoSlouchcatServer() = runTest {
        assertEquals(ProbeResult.NotSlouchcat, answer(MockResponse(code = 404, body = "not found")))
        assertEquals(ProbeResult.NotSlouchcat, answer(MockResponse(body = "<html>router</html>")))
        assertEquals(ProbeResult.NotSlouchcat, answer(MockResponse(body = """{"status":"up"}""")))
        assertEquals(ProbeResult.NotSlouchcat, answer(MockResponse(code = 301, headers = okhttp3.Headers.headersOf("Location", "https://example.test/"))))
    }

    @Test
    fun aRedirectToHttpsOfTheSameHostIsOffered() = runTest {
        val https = "https://${server.hostName}:9443/api/v1/healthz"
        val result = answer(MockResponse(code = 308, headers = okhttp3.Headers.headersOf("Location", https)))
        assertEquals(ProbeResult.RequiresHttps(server.url("/").newBuilder().scheme("https").port(9443).build()), result)
    }

    @Test
    fun noAnswerIsUnreachable() = runTest {
        val gone = server.url("/")
        server.close()
        assertEquals(ProbeResult.Unreachable, probe.probe(gone))
    }

    @Test
    fun contractVersionsCompareAsNumbers() {
        assertTrue(Compatibility.supports("1.42.0"))
        assertTrue(Compatibility.supports("1.42.1+abc1234"))
        assertTrue(Compatibility.supports("1.100.0"))
        assertTrue(Compatibility.supports("2.0.0"))
        assertFalse(Compatibility.supports("1.41.99"))
        assertFalse(Compatibility.supports("0.9.0"))
        assertFalse(Compatibility.supports("dev"))
    }
}
