package com.slouchcat.app.tv.setup

import androidx.lifecycle.SavedStateHandle
import com.slouchcat.app.tv.testing.EMAIL
import com.slouchcat.app.tv.testing.FakeAuth
import com.slouchcat.app.tv.testing.FakeProbe
import com.slouchcat.app.tv.testing.FakeServers
import com.slouchcat.app.tv.testing.TV_DEVICE
import com.slouchcat.app.tv.testing.session
import com.slouchcat.app.tv.testing.settle
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.SessionState
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.net.InetAddress
import java.net.UnknownHostException

@OptIn(ExperimentalCoroutinesApi::class)
class HandSetupViewModelTest {
    private val dispatcher = StandardTestDispatcher()
    private val home = "http://172.16.5.20:8096/".toHttpUrl()
    private val remote = "http://media.example.test/".toHttpUrl()
    private val https = "https://media.example.test/".toHttpUrl()

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    private val resolver = HostResolver { host ->
        when (host) {
            "172.16.5.20" -> listOf(InetAddress.getByAddress(host, byteArrayOf(172.toByte(), 16, 5, 20)))
            "media.example.test" -> listOf(InetAddress.getByAddress(host, byteArrayOf(203.toByte(), 0, 113, 5)))
            else -> throw UnknownHostException(host)
        }
    }

    private fun found(url: okhttp3.HttpUrl) = ProbeResult.Found(ServerInfo(url, "1.43.0", "1.43.0", serverKey = "key"))

    private class Setup(val vm: HandSetupViewModel, val servers: FakeServers, val auth: FakeAuth, val session: com.slouchcat.core.session.SessionManager, val signedIn: MutableList<String>)

    private fun TestScope.setup(probe: FakeProbe, saved: SavedStateHandle = SavedStateHandle()): Setup {
        val servers = FakeServers(null)
        val auth = FakeAuth()
        val session = session(backgroundScope, server = null, token = null, servers = servers)
        val hosts = mutableListOf<String>()
        val vm = HandSetupViewModel(saved, probe, resolver, servers, session, auth, TV_DEVICE, onSignedIn = { hosts += it })
        return Setup(vm, servers, auth, session, hosts)
    }

    @Test
    fun aServerInTheHomeNetworkNeedsNoWarningAndTheSignInIsOfKindTv() = runTest(dispatcher) {
        val s = setup(FakeProbe(found(home)))
        s.vm.onAddress("http://172.16.5.20:8096/")
        settle()
        val check = s.vm.state.value.check as AddressCheck.Found
        assertFalse(check.needsConsent)
        assertTrue(s.vm.state.value.serverReady)
        assertEquals("http://172.16.5.20:8096", s.vm.state.value.address)
        s.vm.onNext()
        assertEquals(HandStep.Email, s.vm.state.value.step)
        s.vm.onEmail("not-an-address")
        s.vm.onNext()
        assertEquals(HandError.EmailInvalid, s.vm.state.value.error)
        s.vm.onEmail(EMAIL)
        s.vm.onNext()
        assertEquals(HandStep.Password, s.vm.state.value.step)
        s.vm.onPassword("secret-password")
        assertFalse(s.vm.state.value.toString().contains("secret-password"))
        s.vm.onSignIn()
        settle()
        assertEquals(listOf("login:$EMAIL:tv"), s.auth.calls)
        assertEquals("secret-password", s.auth.lastPassword)
        assertEquals("", s.vm.state.value.password)
        assertEquals(home, s.servers.flow.value?.address)
        assertEquals("key", s.servers.flow.value?.serverKey)
        assertTrue(s.session.state.value is SessionState.ChooseProfile)
        assertEquals(listOf("172.16.5.20"), s.signedIn)
    }

    @Test
    fun plainHttpOutsideTheHomeNetworkAsksFirst() = runTest(dispatcher) {
        val s = setup(FakeProbe(found(remote)))
        s.vm.onAddress("http://media.example.test")
        settle()
        val check = s.vm.state.value.check as AddressCheck.Found
        assertTrue(check.needsConsent)
        assertFalse(s.vm.state.value.serverReady)
        s.vm.onNext()
        assertEquals(HandStep.Server, s.vm.state.value.step)
        s.vm.onConsent(true)
        assertTrue(s.vm.state.value.serverReady)
        // HTTPS needs nothing.
        val t = setup(FakeProbe(found(https)))
        t.vm.onAddress("media.example.test")
        settle()
        assertEquals("https://media.example.test", t.vm.state.value.address)
        assertFalse((t.vm.state.value.check as AddressCheck.Found).needsConsent)
    }

    @Test
    fun theAddressErrorsAreNamed() = runTest(dispatcher) {
        val probe = FakeProbe(ProbeResult.Unreachable)
        val s = setup(probe)
        s.vm.onAddress("   ")
        assertEquals(AddressCheck.Empty, s.vm.state.value.check)
        s.vm.onAddress("http:// not")
        assertEquals(AddressCheck.Invalid, s.vm.state.value.check)
        s.vm.onAddress("http://unknown.example.test")
        settle()
        assertEquals(AddressCheck.Unreachable, s.vm.state.value.check)
        s.vm.onAddress("https://media.example.test")
        settle()
        assertEquals(AddressCheck.Unreachable, s.vm.state.value.check)
        probe.result = ProbeResult.NotSlouchcat
        s.vm.onAddress("https://media.example.test")
        settle()
        assertEquals(AddressCheck.NotSlouchcat, s.vm.state.value.check)
        probe.result = ProbeResult.TooOld("1.41.0", "1.42.0")
        s.vm.onAddress("https://media.example.test")
        settle()
        assertEquals(AddressCheck.TooOld("1.41.0", "1.42.0"), s.vm.state.value.check)
        probe.result = ProbeResult.RequiresHttps(https)
        s.vm.onAddress("http://172.16.5.20:8096")
        settle()
        assertEquals(AddressCheck.HttpsRequired("https://media.example.test"), s.vm.state.value.check)
        assertEquals("https://media.example.test", s.vm.state.value.address)
    }

    @Test
    fun theSignInErrorsKeepTheStepAndForgetThePassword() = runTest(dispatcher) {
        val s = setup(FakeProbe(found(https)))
        s.vm.onAddress("https://media.example.test")
        settle()
        s.vm.onNext()
        s.vm.onEmail(EMAIL)
        s.vm.onNext()
        val cases = mapOf(
            ApiResult.Failed(401, "invalid_credentials") to HandError.InvalidCredentials,
            ApiResult.Failed(429, "too_many_attempts", 42) to HandError.TooManyAttempts(42),
            ApiResult.Offline to HandError.Unreachable,
            ApiResult.AppTooOld to HandError.AppTooOld,
            ApiResult.Failed(null, "server_unconfirmed") to HandError.ServerUnconfirmed,
            ApiResult.Failed(500, null) to HandError.Failed,
        )
        for ((result, error) in cases) {
            s.auth.login = result
            s.vm.onPassword("pw")
            s.vm.onSignIn()
            settle()
            assertEquals(result.toString(), error, s.vm.state.value.error)
            assertEquals("", s.vm.state.value.password)
            assertEquals(HandStep.Password, s.vm.state.value.step)
        }
        assertTrue(s.signedIn.isEmpty())
    }

    @Test
    fun backGoesOneStepBackAndTheStepsSurviveTheProcess() = runTest(dispatcher) {
        val saved = SavedStateHandle()
        val s = setup(FakeProbe(found(https)), saved)
        s.vm.onAddress("https://media.example.test")
        settle()
        s.vm.onNext()
        s.vm.onEmail(EMAIL)
        s.vm.onNext()
        s.vm.onPassword("pw")
        assertTrue(s.vm.onBack())
        assertEquals(HandStep.Email, s.vm.state.value.step)
        assertEquals("", s.vm.state.value.password)
        // A new process: address, e-mail and step come back, the server is asked again.
        val again = setup(FakeProbe(found(https)), saved)
        settle()
        assertEquals(HandStep.Email, again.vm.state.value.step)
        assertEquals(EMAIL, again.vm.state.value.email)
        assertTrue(again.vm.state.value.check is AddressCheck.Found)
        assertTrue(again.vm.onBack())
        assertFalse(again.vm.onBack())
    }
}
