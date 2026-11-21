package com.slouchcat.core.api

import com.slouchcat.core.api.generated.apis.SystemApi
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.withTimeout
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ApiCallerTest {
    private val server = MockWebServer()
    private val events = SessionEvents()
    private val caller = ApiCaller(events)
    private val http = SlouchcatHttp.client(ClientIdentity(ClientIdentity.Kind.MOBILE, "0.1.0"))

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.close()

    private suspend fun answer(response: MockResponse): ApiResult<*> {
        server.enqueue(response)
        return caller.call { SlouchcatApi.create(::SystemApi, server.url("/"), http).getHealth() }
    }

    private fun problem(status: Int, code: String) =
        MockResponse(code = status, body = """{"type":"about:blank","title":"x","status":$status,"code":"$code"}""")

    @Test
    fun anAnswerIsOk() = runTest {
        val result = answer(MockResponse(body = """{"status":"ok","version":"1.42.0","contractVersion":"1.42.0","serverKey":"k","uptimeSeconds":1}"""))
        assertEquals("1.42.0", ((result as ApiResult.Ok<*>).value as com.slouchcat.core.api.generated.models.Health).version)
    }

    @Test
    fun statusCodesBecomeTheirCases() = runTest {
        assertEquals(ApiResult.NotFound, answer(problem(404, "not_found")))
        assertEquals(ApiResult.NotAllowed("client_not_allowed"), answer(problem(403, "client_not_allowed")))
        assertEquals(ApiResult.Failed(500, "internal"), answer(problem(500, "internal")))
        assertEquals(ApiResult.Failed(409, "conflict"), answer(problem(409, "conflict")))
        assertEquals(ApiResult.Failed(null, "unreadable"), answer(MockResponse(body = "{\"status\":")))
    }

    @Test
    fun signedOutAndAppTooOldReachTheWholeApp() = runTest {
        val first = async { events.events.first() }
        testScheduler.advanceUntilIdle()
        assertEquals(ApiResult.SignedOut, answer(problem(401, "unauthenticated")))
        assertEquals(SessionEvent.SignedOut, withTimeout(5_000) { first.await() })

        val second = async { events.events.first() }
        testScheduler.advanceUntilIdle()
        assertEquals(ApiResult.AppTooOld, answer(problem(426, "app_too_old")))
        assertEquals(SessionEvent.AppTooOld, withTimeout(5_000) { second.await() })
    }

    @Test
    fun aWrongPasswordIsNoLostSession() = runTest {
        val reported = mutableListOf<SessionEvent>()
        val collector = backgroundScope.async { events.events.collect { reported += it } }
        testScheduler.advanceUntilIdle()
        assertEquals(ApiResult.Failed(401, "invalid_credentials"), answer(problem(401, "invalid_credentials")))
        testScheduler.advanceUntilIdle()
        assertEquals(emptyList<SessionEvent>(), reported)
        collector.cancel()
    }

    @Test
    fun aPauseIsReadFromTheProblemOrTheHeader() = runTest {
        val locked = MockResponse(
            code = 429,
            body = """{"type":"about:blank","title":"x","status":429,"code":"pin_locked","retryAfterSeconds":42}""",
        )
        assertEquals(ApiResult.Failed(429, "pin_locked", 42), answer(locked))
        val header = MockResponse(
            code = 429,
            headers = okhttp3.Headers.headersOf("Retry-After", "17"),
            body = """{"type":"about:blank","title":"x","status":429,"code":"too_many_attempts"}""",
        )
        assertEquals(ApiResult.Failed(429, "too_many_attempts", 17), answer(header))
    }

    @Test
    fun aServerThatDidNotProveItselfIsOffline() = runTest {
        val result = caller.call<Unit> { throw ServerKeyChangedException("key") }
        assertEquals(ApiResult.Offline, result)
    }

    @Test
    fun noAnswerIsOffline() = runTest {
        val gone = server.url("/")
        server.close()
        val result = caller.call { SlouchcatApi.create(::SystemApi, gone, http).getHealth() }
        assertTrue(result.toString(), result == ApiResult.Offline)
    }
}
