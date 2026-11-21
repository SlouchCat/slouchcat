package com.slouchcat.app.mobile.setup

import com.slouchcat.core.session.HostResolver
import com.slouchcat.core.session.ProbeResult
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerInfo
import com.slouchcat.core.session.ServerProbe
import com.slouchcat.core.session.ServerStore
import androidx.lifecycle.SavedStateHandle
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.net.InetAddress
import java.net.UnknownHostException

@OptIn(ExperimentalCoroutinesApi::class)
class SetupViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    private class FakeStore(initial: SavedServer? = null) : ServerStore {
        val flow = MutableStateFlow(initial)
        override val server = flow
        var saves = 0
        override suspend fun save(server: SavedServer) {
            saves++
            flow.value = server
        }
        override suspend fun clear() {
            flow.value = null
        }
    }

    private class FakeProbe(var result: (HttpUrl) -> ProbeResult) : ServerProbe {
        val asked = mutableListOf<HttpUrl>()
        override suspend fun probe(address: HttpUrl): ProbeResult {
            asked += address
            return result(address)
        }
    }

    private val found = { url: HttpUrl -> ProbeResult.Found(ServerInfo(url, "1.42.0", "1.42.0")) }
    private val addresses = mapOf(
        "nas.example.test" to listOf(InetAddress.getByName("192.168.77.20")),
        "media.example.test" to listOf(InetAddress.getByName("198.51.100.7")),
        "split.example.test" to listOf(InetAddress.getByName("192.168.77.20"), InetAddress.getByName("198.51.100.7")),
    )
    private val resolver = HostResolver { host ->
        // Literal addresses resolve without DNS; every other unknown name does not exist.
        addresses[host] ?: if (host.first().isDigit()) listOf(InetAddress.getByName(host)) else throw UnknownHostException(host)
    }

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    private fun vm(
        probe: ServerProbe = FakeProbe(found),
        store: ServerStore = FakeStore(),
        saved: SavedStateHandle = SavedStateHandle(),
    ) = SetupViewModel(saved, probe, resolver, store)

    @Test
    fun emptyAndBrokenAddressesAreToldWithoutAsking() = runTest(dispatcher) {
        val probe = FakeProbe(found)
        val model = vm(probe)
        model.onConnect()
        assertEquals(SetupError.Empty, model.state.value.error)
        model.onAddressChange("https://exa mple.test")
        assertNull("typing clears the error", model.state.value.error)
        model.onConnect()
        assertEquals(SetupError.Invalid, model.state.value.error)
        advanceUntilIdle()
        assertTrue(probe.asked.isEmpty())
    }

    @Test
    fun httpsIsCheckedAtOnceAndStoredOnNext() = runTest(dispatcher) {
        val store = FakeStore()
        val model = vm(store = store)
        model.onAddressChange("media.example.test/")
        model.onConnect()
        assertTrue(model.state.value.busy)
        advanceUntilIdle()
        val state = model.state.value
        assertEquals("https://media.example.test", state.address)
        assertFalse(state.busy)
        assertTrue(state.found!!.encrypted)
        assertNull(state.insecureAddress)

        model.onAddressChange("ignored while found")
        assertEquals("https://media.example.test", model.state.value.address)
        val done = mutableListOf<Unit>()
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { model.done.collect { done += it } }
        model.onNext()
        model.onNext()
        advanceUntilIdle()
        assertEquals("one move on, however often «Weiter» is tapped", 1, done.size)
        assertTrue(model.state.value.saving)
        assertEquals(1, store.saves)
        assertEquals("1.42.0", store.flow.value!!.version)
        assertEquals("https://media.example.test/", store.flow.value!!.address.toString())
    }

    @Test
    fun plainHttpInTheHomeNetworkNeedsNoConfirmation() = runTest(dispatcher) {
        val probe = FakeProbe(found)
        val model = vm(probe)
        model.onAddressChange("http://nas.example.test:8096")
        model.onConnect()
        advanceUntilIdle()
        assertNull(model.state.value.insecureAddress)
        assertFalse(model.state.value.found!!.encrypted)
        assertEquals(1, probe.asked.size)
    }

    @Test
    fun plainHttpOutsideTheHomeNetworkWaitsForConfirmation() = runTest(dispatcher) {
        val probe = FakeProbe(found)
        val model = vm(probe)
        model.onAddressChange("http://media.example.test")
        model.onConnect()
        advanceUntilIdle()
        assertEquals("http://media.example.test", model.state.value.insecureAddress)
        assertTrue("nothing sent before the answer", probe.asked.isEmpty())

        model.onDismissInsecure()
        assertNull(model.state.value.insecureAddress)
        assertNull(model.state.value.found)

        model.onConnect()
        advanceUntilIdle()
        model.onConfirmInsecure()
        advanceUntilIdle()
        assertEquals(1, probe.asked.size)
        assertFalse(model.state.value.found!!.encrypted)
    }

    @Test
    fun oneAddressOutsideTheHomeNetworkIsEnoughForTheWarning() = runTest(dispatcher) {
        val model = vm()
        model.onAddressChange("http://split.example.test")
        model.onConnect()
        advanceUntilIdle()
        assertEquals("http://split.example.test", model.state.value.insecureAddress)
    }

    @Test
    fun theAnswersOfTheProbeBecomeErrors() = runTest(dispatcher) {
        val probe = FakeProbe { ProbeResult.Unreachable }
        val model = vm(probe)
        model.onAddressChange("media.example.test")
        model.onConnect()
        advanceUntilIdle()
        assertEquals(SetupError.Unreachable, model.state.value.error)

        probe.result = { ProbeResult.NotSlouchcat }
        model.onConnect()
        advanceUntilIdle()
        assertEquals(SetupError.NotSlouchcat, model.state.value.error)

        probe.result = { ProbeResult.TooOld("1.41.3", "1.42.0") }
        model.onConnect()
        advanceUntilIdle()
        assertEquals(SetupError.TooOld("1.41.3", "1.42.0"), model.state.value.error)
        assertNull(model.state.value.found)
    }

    @Test
    fun theAddressAndAnOpenWarningSurviveTheProcess() = runTest(dispatcher) {
        val saved = SavedStateHandle()
        val first = vm(saved = saved)
        first.onAddressChange("http://media.example.test")
        first.onConnect()
        advanceUntilIdle()
        assertEquals("http://media.example.test", first.state.value.insecureAddress)

        // The process ends; the system hands the saved state to a new ViewModel.
        val probe = FakeProbe(found)
        val second = vm(probe = probe, saved = saved)
        assertEquals("http://media.example.test", second.state.value.address)
        assertEquals("http://media.example.test", second.state.value.insecureAddress)
        second.onConfirmInsecure()
        advanceUntilIdle()
        assertEquals(1, probe.asked.size)
    }

    @Test
    fun anUnknownHostIsUnreachable() = runTest(dispatcher) {
        val probe = FakeProbe(found)
        val model = vm(probe)
        model.onAddressChange("http://nowhere.example.test")
        model.onConnect()
        advanceUntilIdle()
        assertEquals(SetupError.Unreachable, model.state.value.error)
        assertTrue(probe.asked.isEmpty())
    }

    @Test
    fun anotherAddressStartsFromTheFieldAndTheStoredServerFillsIt() = runTest(dispatcher) {
        val model = vm(store = FakeStore(SavedServer("http://192.0.2.10:8096/".toHttpUrl(), "1.42.0")))
        advanceUntilIdle()
        assertEquals("http://192.0.2.10:8096", model.state.value.address)
        model.onConnect()
        advanceUntilIdle()
        model.onConfirmInsecure()
        advanceUntilIdle()
        assertTrue(model.state.value.found != null)
        model.onChangeAddress()
        assertNull(model.state.value.found)
        model.onAddressChange("https://media.example.test")
        assertEquals("https://media.example.test", model.state.value.address)
    }
}
